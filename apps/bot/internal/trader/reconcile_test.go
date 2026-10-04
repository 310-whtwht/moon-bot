package trader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stopBroker is a paper broker that also holds stop orders, like a real one.
type stopBroker struct {
	*paper.Broker

	mu        sync.Mutex
	stops     map[string]*heldStop
	seq       int
	orderIDs  []string // every order placed, to build the execution history
	closes    int      // market closes sent by the bot
	failStop  error
	noHistory bool
}

type heldStop struct {
	order    broker.StopOrder
	active   bool
	executed bool
}

func newStopBroker(b *paper.Broker) *stopBroker {
	return &stopBroker{Broker: b, stops: map[string]*heldStop{}}
}

func (s *stopBroker) Name() string { return "gmo" }

func (s *stopBroker) Instruments(context.Context) ([]market.Instrument, error) {
	return []market.Instrument{{Key: market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"}, TickSize: d("0.001")}}, nil
}

func (s *stopBroker) track(ack broker.OrderAck, err error) (broker.OrderAck, error) {
	if err == nil {
		s.mu.Lock()
		s.orderIDs = append(s.orderIDs, ack.OrderID)
		s.mu.Unlock()
	}
	return ack, err
}

func (s *stopBroker) PlaceOpen(ctx context.Context, o broker.OpenOrder) (broker.OrderAck, error) {
	return s.track(s.Broker.PlaceOpen(ctx, o))
}

func (s *stopBroker) PlaceClose(ctx context.Context, o broker.CloseOrder) (broker.OrderAck, error) {
	s.mu.Lock()
	s.closes++
	for _, st := range s.stops {
		if st.active && st.order.PositionID == o.PositionID {
			s.mu.Unlock()
			return broker.OrderAck{}, errors.New("ERR-423: close size exceeds the position while a stop order is working")
		}
	}
	s.mu.Unlock()
	return s.track(s.Broker.PlaceClose(ctx, o))
}

func (s *stopBroker) PlaceProtectiveStop(_ context.Context, o broker.StopOrder) (broker.OrderAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failStop != nil {
		return broker.OrderAck{}, s.failStop
	}
	s.seq++
	id := fmt.Sprintf("stop-%d", s.seq)
	s.stops[id] = &heldStop{order: o, active: true}
	return broker.OrderAck{OrderID: id, Status: "WAITING"}, nil
}

func (s *stopBroker) OrderActive(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.stops[id]
	return ok && st.active, nil
}

func (s *stopBroker) Cancel(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.stops[id]
	if !ok || !st.active {
		return errors.New("ERR-5122: order is already executed, cancelled or expired")
	}
	st.active = false
	return nil
}

func (s *stopBroker) RecentExecutions(ctx context.Context, _ string) ([]broker.Execution, error) {
	s.mu.Lock()
	ids := append([]string(nil), s.orderIDs...)
	hide := s.noHistory
	s.mu.Unlock()
	if hide {
		return nil, nil
	}
	var out []broker.Execution
	for _, id := range ids {
		execs, _ := s.Broker.Executions(ctx, id)
		out = append(out, execs...)
	}
	return out, nil
}

// trigger executes a held stop at the current quote, as the broker would.
func (s *stopBroker) trigger(t *testing.T, id string) {
	t.Helper()
	s.mu.Lock()
	st := s.stops[id]
	require.NotNil(t, st)
	st.active, st.executed = false, true
	s.mu.Unlock()
	_, err := s.track(s.Broker.PlaceClose(context.Background(), broker.CloseOrder{
		Symbol: st.order.Symbol, PositionID: st.order.PositionID, Side: st.order.Side,
		Type: broker.OrderMarket, Size: st.order.Size,
	}))
	require.NoError(t, err)
}

func (s *stopBroker) activeStops() []*heldStop {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*heldStop
	for _, st := range s.stops {
		if st.active {
			out = append(out, st)
		}
	}
	return out
}

func stopHarness(t *testing.T) (*harness, *stopBroker, *memOps) {
	h := newHarness(t, Config{FillPollInterval: time.Millisecond, FillPollAttempts: 3})
	sb := newStopBroker(h.paper)
	h.mgr.Brokers = map[string]broker.Broker{"gmo": sb}
	h.store.deployments[0].Broker = "gmo"
	ops := withOps(h)
	return h, sb, ops
}

func (h *harness) gmoPositions() []Position {
	ps, _ := h.store.OpenPositions(context.Background(), "gmo", "default")
	return ps
}

// later advances the clock without reaching the next bar.
func (h *harness) later(d time.Duration) {
	h.feed.mu.Lock()
	h.feed.now = h.feed.now.Add(d)
	h.feed.mu.Unlock()
}

func halted(ops *memOps) (bool, string) {
	ok, why := SwitchGuard{Store: ops}.EntriesAllowed(context.Background(), "gmo")
	return !ok, why
}

func TestProtectiveStopIsPlacedAtBrokerOnTheTickGrid(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149.0004)}) // distance 0.9996 → 149.0004 before rounding
	h.poll(10)
	h.poll(11)

	pos := h.gmoPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, "149", pos[0].StopPrice.String(), "rounded to the 0.001 tick")
	assert.NotEmpty(t, pos[0].StopOrderID)

	stops := sb.activeStops()
	require.Len(t, stops, 1)
	assert.Equal(t, broker.SideSell, stops[0].order.Side, "a long is closed by selling")
	assert.Equal(t, "149", stops[0].order.StopPrice.String())
	assert.Equal(t, "100", stops[0].order.Size.String())
}

func TestStopExecutedAtBrokerIsSettledByReconcile(t *testing.T) {
	h, sb, ops := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	stopID := h.gmoPositions()[0].StopOrderID

	// The bot is not looking; the broker executes the stop on its own.
	h.feed.quote("148.900", "148.910")
	sb.trigger(t, stopID)

	h.later(10 * time.Second)                         // past the 5 s reconcile throttle
	h.mgr.OnTick(context.Background(), h.feed.tick()) // quote beyond the stop → reconcile
	assert.Empty(t, h.gmoPositions())
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
	assert.Equal(t, 0, sb.closes, "the bot sent no close of its own")

	closed := h.store.positions[0]
	assert.Equal(t, "148.9", closed.ClosePrice.String())
	assert.Equal(t, "-111.59782", closed.RealizedPnL.String())
	assert.Contains(t, h.events[len(h.events)-1].Message, "ブローカー側で決済")
	stopped, _ := halted(ops)
	assert.False(t, stopped, "an explained close is not a mismatch")
}

func TestQuoteBeyondStopDoesNotSendACloseWhileBrokerHoldsTheStop(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)

	h.feed.quote("148.900", "148.910") // the broker has not executed its stop yet
	h.mgr.OnTick(context.Background(), h.feed.tick())
	h.mgr.OnTick(context.Background(), h.feed.tick())

	assert.Equal(t, 0, sb.closes)
	assert.Len(t, h.gmoPositions(), 1, "left to the broker's stop order")
	assert.Len(t, sb.activeStops(), 1)
}

func TestSignalExitCancelsTheStopFirst(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}})
	h.poll(10)
	h.poll(11)
	require.Len(t, sb.activeStops(), 1)

	h.poll(12)
	assert.Empty(t, h.gmoPositions())
	assert.Empty(t, sb.activeStops(), "stop order cancelled")
	assert.Equal(t, 1, sb.closes)
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
}

func TestExitRacingWithAnExecutedStopSettlesFromBroker(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}})
	h.poll(10)
	h.poll(11)
	stopID := h.gmoPositions()[0].StopOrderID

	// The stop executes just before the bot's exit signal is processed. Keep
	// the periodic reconcile out of the way so the exit path meets the race.
	h.feed.quote("148.900", "148.910")
	sb.trigger(t, stopID)
	h.mgr.Config.ReconcileInterval = 24 * time.Hour
	h.mgr.runners["dep-00000001"].cfg.ReconcileInterval = 24 * time.Hour

	h.poll(12)
	assert.Empty(t, h.gmoPositions())
	assert.Equal(t, 0, sb.closes, "no close order for a position that is already gone")
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
	assert.Equal(t, "148.9", h.store.positions[0].ClosePrice.String())
}

func TestStopPlacementFailureFallsBackToBotSideStop(t *testing.T) {
	h, sb, _ := stopHarness(t)
	sb.failStop = errors.New("ERR-5114: price is off the tick grid")
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)

	pos := h.gmoPositions()
	require.Len(t, pos, 1)
	assert.Empty(t, pos[0].StopOrderID)
	assert.Contains(t, h.kinds(), "error")
	assert.Contains(t, h.events[0].Message, "逆指値")

	sb.failStop = errors.New("still failing") // so reconcile cannot place it either
	h.feed.quote("148.900", "148.910")
	h.mgr.OnTick(context.Background(), h.feed.tick())
	assert.Empty(t, h.gmoPositions(), "the bot closes at market itself")
	assert.Equal(t, 1, sb.closes)
}

func TestExpiredStopIsPlacedAgain(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	first := h.gmoPositions()[0].StopOrderID

	sb.mu.Lock()
	sb.stops[first].active = false // expired or cancelled by hand
	sb.mu.Unlock()

	h.later(6 * time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))

	second := h.gmoPositions()[0].StopOrderID
	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
	require.Len(t, sb.activeStops(), 1)
	assert.Equal(t, "149", sb.activeStops()[0].order.StopPrice.String())
}

func TestUnknownPositionAtBrokerHaltsOnce(t *testing.T) {
	h, sb, ops := stopHarness(t)
	h.poll(10)

	// Someone trades by hand on the same account.
	_, err := sb.Broker.PlaceOpen(context.Background(), marketOrder())
	require.NoError(t, err)

	h.later(6 * time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	stopped, why := halted(ops)
	assert.True(t, stopped)
	assert.Contains(t, why, "建玉の不一致")
	require.Equal(t, []string{"error", "kill_switch"}, h.kinds())
	assert.Contains(t, h.events[0].Message, "bot の知らない")

	h.later(6 * time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Len(t, h.events, 2, "the same mismatch is not reported again")
}

func marketOrder() broker.OpenOrder {
	return broker.OpenOrder{ClientOrderID: "manual", Symbol: "USD_JPY", Side: broker.SideBuy, Type: broker.OrderMarket, Size: d("100")}
}

func TestPositionGoneWithoutAFillHalts(t *testing.T) {
	h, sb, ops := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	stopID := h.gmoPositions()[0].StopOrderID

	sb.trigger(t, stopID)
	sb.noHistory = true // the broker's history does not show the closing fill

	h.later(6 * time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	stopped, _ := halted(ops)
	assert.True(t, stopped)
	assert.Len(t, h.gmoPositions(), 1, "left open in the database for a human to resolve")
	assert.Contains(t, h.events[len(h.events)-2].Message, "決済の約定も見つかりません")
}

func TestExecutionEventTriggersImmediateReconcile(t *testing.T) {
	h, sb, _ := stopHarness(t)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	stopID := h.gmoPositions()[0].StopOrderID

	// The periodic check is far away; only the execution event can settle this.
	h.mgr.runners["dep-00000001"].cfg.ReconcileInterval = 24 * time.Hour
	h.feed.quote("148.900", "148.910")
	sb.trigger(t, stopID)

	events := make(chan broker.Execution, 2)
	events <- broker.Execution{Symbol: "EUR_JPY", SettleType: "CLOSE"} // another symbol: ignored
	events <- broker.Execution{Symbol: "USD_JPY", SettleType: "CLOSE"}
	close(events)
	h.mgr.WatchExecutions(context.Background(), "gmo", events)

	assert.Empty(t, h.gmoPositions(), "settled as soon as the broker reported the fill")
	assert.Equal(t, 0, sb.closes)
	assert.Equal(t, "148.9", h.store.positions[0].ClosePrice.String())
	assert.True(t, h.logged("execution stream ended"))
}
