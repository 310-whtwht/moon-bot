package trader

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flaky wraps the paper broker and misbehaves like a real one can.
type flaky struct {
	*paper.Broker
	name string

	mu         sync.Mutex
	loseAnswer bool // the order executes, but the answer is lost
	drop       bool // the order never reaches the broker, and no answer comes back
	emptyFills int  // Executions returns nothing this many times (fills arrive late)
	byClient   map[string]string
	opens      int
}

func newFlaky(b *paper.Broker, name string) *flaky {
	return &flaky{Broker: b, name: name, byClient: map[string]string{}}
}

func (f *flaky) Name() string { return f.name }

func (f *flaky) place(clientID string, do func() (broker.OrderAck, error)) (broker.OrderAck, error) {
	f.mu.Lock()
	drop, lose := f.drop, f.loseAnswer
	f.mu.Unlock()
	if drop {
		return broker.OrderAck{}, fmt.Errorf("timeout: %w", broker.ErrUnknownResult)
	}
	ack, err := do()
	if err != nil {
		return ack, err
	}
	f.mu.Lock()
	f.byClient[clientID] = ack.OrderID
	f.mu.Unlock()
	if lose {
		return broker.OrderAck{}, fmt.Errorf("connection reset: %w", broker.ErrUnknownResult)
	}
	return ack, nil
}

func (f *flaky) PlaceOpen(ctx context.Context, o broker.OpenOrder) (broker.OrderAck, error) {
	f.mu.Lock()
	f.opens++
	f.mu.Unlock()
	return f.place(o.ClientOrderID, func() (broker.OrderAck, error) { return f.Broker.PlaceOpen(ctx, o) })
}

func (f *flaky) PlaceClose(ctx context.Context, o broker.CloseOrder) (broker.OrderAck, error) {
	return f.place(o.ClientOrderID, func() (broker.OrderAck, error) { return f.Broker.PlaceClose(ctx, o) })
}

func (f *flaky) Executions(ctx context.Context, orderID string) ([]broker.Execution, error) {
	f.mu.Lock()
	if f.emptyFills > 0 {
		f.emptyFills--
		f.mu.Unlock()
		return nil, nil
	}
	f.mu.Unlock()
	return f.Broker.Executions(ctx, orderID)
}

func (f *flaky) FindOrder(ctx context.Context, _ string, clientOrderID string) ([]broker.Execution, bool, error) {
	f.mu.Lock()
	orderID, ok := f.byClient[clientOrderID]
	f.mu.Unlock()
	if !ok {
		return nil, false, nil
	}
	execs, err := f.Broker.Executions(ctx, orderID)
	return execs, true, err
}

// flakyHarness swaps the harness's broker for a flaky one and attaches the kill switch store.
func flakyHarness(t *testing.T, brokerName string) (*harness, *flaky, *memOps) {
	h := newHarness(t, Config{FillPollInterval: time.Millisecond, FillPollAttempts: 3})
	f := newFlaky(h.paper, brokerName)
	h.mgr.Brokers = map[string]broker.Broker{brokerName: f}
	h.store.deployments[0].Broker = brokerName
	ops := withOps(h)
	return h, f, ops
}

func TestFillsThatArriveLateAreAwaited(t *testing.T) {
	h, f, _ := flakyHarness(t, paper.BrokerName)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)

	f.emptyFills = 2 // the first two polls see no fill yet
	h.poll(11)
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())
	require.Len(t, h.openPositions(), 1)
	assert.Equal(t, "100", h.openPositions()[0].Units.String())
}

func TestLostAnswerIsResolvedByLookup(t *testing.T) {
	h, f, ops := flakyHarness(t, paper.BrokerName)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)

	f.loseAnswer = true // the order executes, but the bot never hears back
	h.poll(11)

	assert.Equal(t, []string{"open:filled"}, h.store.statuses(), "found by client order ID")
	require.Len(t, h.openPositions(), 1)
	assert.Equal(t, 1, f.opens, "never sent a second time")
	brokerPositions, _ := h.paper.OpenPositions(context.Background(), "")
	assert.Len(t, brokerPositions, 1)
	ok, _ := SwitchGuard{Store: ops}.EntriesAllowed(context.Background(), paper.BrokerName)
	assert.True(t, ok, "resolved: no halt")
}

func TestUnresolvedOpenHaltsBrokerAndIsNotResent(t *testing.T) {
	h, f, ops := flakyHarness(t, paper.BrokerName)
	setScript(0, map[int]strategy.Signal{10: long(149), 12: long(149)})
	h.poll(10)

	f.drop = true // no answer, and the broker has no trace of the order
	h.poll(11)

	assert.Equal(t, []string{"open:unknown"}, h.store.statuses())
	assert.Empty(t, h.openPositions())
	assert.Equal(t, 1, f.opens, "not re-sent")
	assert.Contains(t, h.kinds(), "error")
	assert.Contains(t, h.events[0].Message, "新規発注を停止")

	ok, why := SwitchGuard{Store: ops}.EntriesAllowed(context.Background(), paper.BrokerName)
	assert.False(t, ok, "the broker's kill switch is on")
	assert.Contains(t, why, "発注結果が不明")

	f.drop = false
	h.poll(13) // the next signal is blocked until a human releases the switch
	assert.Equal(t, 1, f.opens)
	assert.Equal(t, []string{"open:unknown"}, h.store.statuses())
}

func TestUnresolvedCloseStopsRetrying(t *testing.T) {
	h, f, ops := flakyHarness(t, paper.BrokerName)
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)

	f.drop = true
	h.poll(12) // exit signal: the close order's outcome is unknown
	assert.Equal(t, []string{"open:filled", "close:unknown"}, h.store.statuses())
	assert.Len(t, h.openPositions(), 1, "still open in the database until someone checks")
	ok, _ := SwitchGuard{Store: ops}.EntriesAllowed(context.Background(), paper.BrokerName)
	assert.False(t, ok)

	// No retry loop: later polls and ticks do not send more close orders.
	f.drop = false
	h.feed.mu.Lock()
	h.feed.now = h.feed.now.Add(2 * time.Minute)
	h.feed.mu.Unlock()
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Equal(t, []string{"open:filled", "close:unknown"}, h.store.statuses())
}

func TestDefiniteRejectionIsNotTreatedAsUnknown(t *testing.T) {
	h, _, ops := flakyHarness(t, paper.BrokerName)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.store.deployments[0].Units = d("100000") // margin for 100,000 units is far beyond 30,000 JPY
	h.mgr.Config.MarginBuffer = 1
	h.poll(10)
	h.poll(11)

	assert.Equal(t, []string{"open:rejected"}, h.store.statuses())
	ok, _ := SwitchGuard{Store: ops}.EntriesAllowed(context.Background(), paper.BrokerName)
	assert.True(t, ok, "a clear rejection does not halt trading")
}

func TestHardCapForRealMoneyBrokers(t *testing.T) {
	h, f, _ := flakyHarness(t, "gmo")
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.store.deployments[0].Units = d("10001") // one over HardMaxLiveUnits
	h.poll(10)
	h.poll(11)

	assert.Equal(t, 0, f.opens)
	assert.Empty(t, h.store.statuses())
	assert.Equal(t, []string{"skipped"}, h.kinds())
	assert.Contains(t, h.events[0].Message, "hard cap")
}
