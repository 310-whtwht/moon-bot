package trader

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// --- scripted strategy: emits fixed signals on given bar indices -------------

var (
	scriptsMu sync.Mutex
	scripts   = map[int]map[int]strategy.Signal{} // variant -> hours since t0 -> signal
)

func setScript(variant int, signals map[int]strategy.Signal) {
	scriptsMu.Lock()
	defer scriptsMu.Unlock()
	scripts[variant] = signals
}

type scripted struct{ variant int }

func (s scripted) OnBar(bar strategy.Bar, _ *strategy.Position) strategy.Signal {
	scriptsMu.Lock()
	defer scriptsMu.Unlock()
	hour := int(bar.Time.Sub(t0) / time.Hour)
	if sig, ok := scripts[s.variant][hour]; ok {
		return sig
	}
	return strategy.Signal{Action: strategy.Hold}
}

func init() {
	strategy.Register(strategy.Definition{
		Type:   "test_scripted",
		Params: []strategy.ParamSpec{{Name: "variant", Integer: true, Min: 0, Max: 9}},
		Factory: func(p strategy.Params) strategy.Strategy {
			return scripted{variant: int(p["variant"])}
		},
	})
}

// --- fake market data --------------------------------------------------------

// Monday 2026-10-05 00:00 UTC.
var t0 = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

// feed serves flat hourly bars at 150.000 and a settable quote.
type feed struct {
	mu       sync.Mutex
	now      time.Time
	bid, ask decimal.Decimal
	status   market.MarketStatus
}

func newFeed() *feed {
	return &feed{now: t0, bid: d("150.000"), ask: d("150.010"), status: market.StatusOpen}
}

func (f *feed) clock() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }

// at moves the clock to `hours` after t0 plus 5 seconds (just after a bar close).
func (f *feed) at(hours int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t0.Add(time.Duration(hours)*time.Hour + 5*time.Second)
}

func (f *feed) quote(bid, ask string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bid, f.ask = d(bid), d(ask)
}

func (f *feed) setStatus(s market.MarketStatus) { f.mu.Lock(); defer f.mu.Unlock(); f.status = s }

func (f *feed) tick() market.Tick {
	f.mu.Lock()
	defer f.mu.Unlock()
	return market.Tick{Key: market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"}, Bid: f.bid, Ask: f.ask, Status: f.status, Time: f.now}
}

func (f *feed) Name() string                                        { return "gmo" }
func (f *feed) Status(context.Context) (market.MarketStatus, error) { return f.status, nil }
func (f *feed) Instruments(context.Context) ([]market.Instrument, error) {
	return nil, nil
}

// Ticks quotes every symbol the tests deploy at the same price.
func (f *feed) Ticks(context.Context) ([]market.Tick, error) {
	usd := f.tick()
	gbp := usd
	gbp.Key.Symbol = "GBP_JPY"
	return []market.Tick{usd, gbp}, nil
}
func (f *feed) SubscribeTicks(context.Context, []string) (<-chan market.Tick, error) {
	return nil, nil
}

// Bars returns one bar per hour since t0, including the bar still forming.
func (f *feed) Bars(_ context.Context, req broker.BarsRequest) ([]market.Bar, error) {
	var out []market.Bar
	for at := t0; at.Before(req.To); at = at.Add(time.Hour) {
		if at.Before(req.From) {
			continue
		}
		p := d("150.000")
		out = append(out, market.Bar{Key: market.InstrumentKey{Broker: "gmo", Symbol: req.Symbol},
			Timeframe: req.Timeframe, PriceType: req.PriceType, OpenTime: at, Open: p, High: p, Low: p, Close: p})
	}
	if len(out) == 0 {
		return nil, broker.ErrNoData
	}
	return out, nil
}

// --- harness -----------------------------------------------------------------

type harness struct {
	t      *testing.T
	feed   *feed
	store  *memStore
	paper  *paper.Broker
	mgr    *Manager
	events []Event
	logs   []string
}

func (h *harness) Notify(e Event) { h.events = append(h.events, e) }

func (h *harness) kinds() []string {
	var out []string
	for _, e := range h.events {
		out = append(out, e.Kind)
	}
	return out
}

func (h *harness) logged(substr string) bool {
	for _, l := range h.logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

func dep() Deployment {
	return Deployment{ID: "dep-00000001", Name: "test", StrategyID: "strat-1", Broker: paper.BrokerName,
		AccountID: "default", Symbol: "USD_JPY", Timeframe: market.TF1Hour, Units: d("100"), Enabled: true}
}

func newHarness(t *testing.T, cfg Config) *harness {
	t.Helper()
	h := &harness{t: t, feed: newFeed(), store: newMemStore()}
	h.store.deployments = []Deployment{dep()}
	h.store.versions["v1"] = Version{ID: "v1", Type: "test_scripted", Params: strategy.Params{"variant": 0}}
	h.store.versions["v2"] = Version{ID: "v2", Type: "test_scripted", Params: strategy.Params{"variant": 1}}
	h.store.active["strat-1"] = "v1"
	setScript(0, nil)
	setScript(1, nil)
	h.newManager(cfg)
	return h
}

// newManager (re)creates the paper broker and manager on the same store, like a bot restart.
func (h *harness) newManager(cfg Config) {
	h.paper = paper.New(h.feed, paper.Options{InitialBalance: d("30000"), FeeRate: d("0.00002"), Now: h.feed.clock})
	require.NoError(h.t, RestorePaper(context.Background(), h.store, h.paper, d("30000")))
	h.mgr = &Manager{
		Store: h.store, Brokers: map[string]broker.Broker{paper.BrokerName: h.paper},
		Notifier: h, Config: cfg, Now: h.feed.clock,
		Logf: func(format string, args ...any) { h.logs = append(h.logs, fmt.Sprintf(format, args...)) },
	}
}

func (h *harness) poll(hours int) {
	h.t.Helper()
	h.feed.at(hours)
	require.NoError(h.t, h.mgr.PollOnce(context.Background()))
}

func (h *harness) openPositions() []Position {
	ps, _ := h.store.OpenPositions(context.Background(), paper.BrokerName, "default")
	return ps
}

func long(stop float64) strategy.Signal {
	return strategy.Signal{Action: strategy.EnterLong, StopLoss: stop, Reason: "test long"}
}

func short(stop float64) strategy.Signal {
	return strategy.Signal{Action: strategy.EnterShort, StopLoss: stop, Reason: "test short"}
}

// --- tests -------------------------------------------------------------------

func TestWarmupDoesNotTrade_ThenFreshSignalOpens(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{5: long(149), 10: long(149)})

	h.poll(10) // bars 0..9 are history: bar 5's signal must not trade
	assert.Empty(t, h.store.statuses())
	assert.True(t, h.logged("ready"))

	h.poll(11) // bar 10 just closed
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, broker.SideBuy, pos[0].Side)
	assert.Equal(t, "150.01", pos[0].OpenPrice.String(), "bought at ASK")
	assert.Equal(t, "149", pos[0].StopPrice.String(), "BID 150.000 minus the strategy's 1.000 distance")
	assert.Equal(t, "v1", pos[0].StrategyVersionID)
	assert.Equal(t, []string{"opened"}, h.kinds())

	h.poll(11) // polling again changes nothing
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())
}

func TestEveryJudgedBarIsLogged(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{11: long(149)})

	h.poll(10)
	assert.False(t, h.logged("-> HOLD"), "warmup bars are not logged one by one")

	h.poll(11) // bar 10 closed: no signal
	assert.True(t, h.logged("test: bar 2026-10-05T10:00:00Z close 150 -> HOLD (flat)"))

	h.poll(12) // bar 11 closed: entry
	assert.True(t, h.logged("test: bar 2026-10-05T11:00:00Z close 150 -> ENTER_LONG (flat)"))

	h.poll(13)
	assert.True(t, h.logged("test: bar 2026-10-05T12:00:00Z close 150 -> HOLD (holding BUY)"))

	// The same decisions are kept for the UI.
	require.Len(t, h.store.bars, 3)
	assert.Equal(t, t0.Add(10*time.Hour), h.store.bars[0].BarTime)
	assert.Equal(t, "HOLD", h.store.bars[0].Action)
	assert.Equal(t, broker.Side(""), h.store.bars[0].Holding)
	assert.Equal(t, "ENTER_LONG", h.store.bars[1].Action)
	assert.Equal(t, broker.SideBuy, h.store.bars[2].Holding)
	assert.Equal(t, "150", h.store.bars[2].Close.String())
}

func TestTwoDeploymentsTradeSideBySide(t *testing.T) {
	second := dep()
	second.ID, second.Name, second.Symbol = "dep-00000002", "test gbp", "GBP_JPY"

	// With room for two positions, both deployments open on the same bar.
	h := newHarness(t, Config{AccountLimits: risk.Limits{MaxOpenPositions: 2}})
	h.store.deployments = append(h.store.deployments, second)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	pos := h.openPositions()
	require.Len(t, pos, 2)
	assert.ElementsMatch(t, []string{"USD_JPY", "GBP_JPY"}, []string{pos[0].Symbol, pos[1].Symbol})
	assert.ElementsMatch(t, []string{"dep-00000001", "dep-00000002"}, []string{pos[0].DeploymentID, pos[1].DeploymentID})

	// With room for one, the second entry is refused by the account limit.
	h = newHarness(t, Config{AccountLimits: risk.Limits{MaxOpenPositions: 1}})
	h.store.deployments = append(h.store.deployments, second)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)
	assert.Contains(t, h.kinds(), "rejected")
	assert.Contains(t, h.events[len(h.events)-1].Message, "max_open_positions")
}

func TestDeploymentKeyDiffersForIDsSharingAPrefix(t *testing.T) {
	// The two seeded deployments differ only in their last character.
	a := deploymentKey("dddddddd-dddd-dddd-dddd-dddddddddddd")
	b := deploymentKey("dddddddd-dddd-dddd-dddd-ddddddddddd2")
	assert.NotEqual(t, a, b)
	assert.Len(t, a, 8)
	assert.Equal(t, a, deploymentKey("dddddddd-dddd-dddd-dddd-dddddddddddd"), "stable")
}

func TestScriptStrategyTradesAndABrokenOneOnlyHolds(t *testing.T) {
	// Buys on the bar that opens at 11:00 JST... i.e. bar index 11 from t0 (09:00 JST).
	h := newHarness(t, Config{})
	h.store.versions["v1"] = Version{ID: "v1", Type: strategy.ScriptType, Script: `
def on_bar(bar, pos):
    explain("hour " + str(bar.hour))
    if bar.hour == 20 and pos == None:
        return buy(stop=bar.close - 1, reason="eight pm")
    return hold()
`}
	h.poll(11) // warm-up on bars 0..10 (09:00..19:00 JST)
	assert.Empty(t, h.store.statuses())
	h.poll(12) // bar 11 (20:00 JST) closed
	require.Len(t, h.openPositions(), 1)
	assert.Equal(t, "149", h.openPositions()[0].StopPrice.String())
	assert.True(t, h.logged("-> ENTER_LONG (flat) [hour 20]"))

	// A script that fails at run time stops deciding and says so once.
	h = newHarness(t, Config{})
	h.store.versions["v1"] = Version{ID: "v1", Type: strategy.ScriptType, Script: `
def on_bar(bar, pos):
    if bar.hour == 20:
        return buy(stop=-1)
    return hold()
`}
	h.poll(11)
	h.poll(12)
	h.poll(13)
	h.poll(14)
	assert.Empty(t, h.openPositions())
	require.Equal(t, []string{"error"}, h.kinds(), "reported once, not on every bar")
	assert.Contains(t, h.events[0].Message, "stop must be a positive price")

	// One that cannot even compile is a poll error, as with an unknown type.
	h = newHarness(t, Config{})
	h.store.versions["v1"] = Version{ID: "v1", Type: strategy.ScriptType, Script: "def on_bar(bar):\n    pass"}
	h.feed.at(11)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.True(t, h.logged("on_bar must take 2 arguments"))
}

func TestSameDecisionIsNeverSentTwice(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)

	// Simulate the bot forgetting it acted (crash between send and record):
	// replaying the same bar's decision must not reach the broker again.
	r := h.mgr.runners["dep-00000001"]
	r.pos = nil
	bar := market.Bar{OpenTime: t0.Add(10 * time.Hour), Close: d("150.000")}
	require.NoError(t, r.act(context.Background(), long(149), bar))

	brokerPositions, _ := h.paper.OpenPositions(context.Background(), "")
	assert.Len(t, brokerPositions, 1, "no second position at the broker")
	assert.True(t, h.logged("already exists"))
}

func TestNoEntryWhenMarketClosed(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)

	h.feed.setStatus(market.StatusClosed)
	h.poll(11)
	assert.Empty(t, h.store.statuses(), "weekend / maintenance: nothing is sent")
	assert.Empty(t, h.openPositions())
}

func TestStaleSignalIsSkipped(t *testing.T) {
	h := newHarness(t, Config{MaxSignalAge: 5 * time.Minute})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)

	// The bot notices bar 10 only 20 minutes after it closed.
	h.feed.mu.Lock()
	h.feed.now = t0.Add(11*time.Hour + 20*time.Minute)
	h.feed.mu.Unlock()
	require.NoError(t, h.mgr.PollOnce(context.Background()))

	assert.Empty(t, h.store.statuses())
	assert.True(t, h.logged("skipped stale"))
}

func TestStopLossClosesOnTick(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)

	ctx := context.Background()
	h.feed.quote("149.500", "149.510")
	h.mgr.OnTick(ctx, h.feed.tick())
	assert.Len(t, h.openPositions(), 1, "above the stop")

	h.feed.quote("148.900", "148.910")
	h.mgr.OnTick(ctx, h.feed.tick())
	h.mgr.OnTick(ctx, h.feed.tick()) // a second tick must not close twice
	assert.Empty(t, h.openPositions())
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())

	closed := h.store.positions[0]
	assert.Equal(t, "148.9", closed.ClosePrice.String(), "long closes at BID")
	// (148.900 - 150.010) * 100 = -111, minus fees 0.30002 + 0.2978
	assert.Equal(t, "-111.59782", closed.RealizedPnL.String())
	assert.Equal(t, []string{"opened", "closed"}, h.kinds())
}

func TestStopLossIsAlsoCheckedOnPoll(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)

	// No tick is delivered (quote stream down); the next poll still stops out.
	h.feed.quote("148.900", "148.910")
	h.feed.mu.Lock()
	h.feed.now = h.feed.now.Add(30 * time.Second)
	h.feed.mu.Unlock()
	require.NoError(t, h.mgr.PollOnce(context.Background()))

	assert.Empty(t, h.openPositions())
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
}

func TestReverseClosesThenOpens(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149), 11: short(151)})
	h.poll(10)
	h.poll(11)
	h.poll(12)

	assert.Equal(t, []string{"open:filled", "close:filled", "open:filled"}, h.store.statuses())
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, broker.SideSell, pos[0].Side)
	assert.Equal(t, "150", pos[0].OpenPrice.String(), "sold at BID")
	assert.Equal(t, "151.01", pos[0].StopPrice.String(), "ASK 150.010 plus 1.000")
}

func TestDailyLossLimitRejectsNextEntry(t *testing.T) {
	h := newHarness(t, Config{AccountLimits: risk.Limits{MaxDailyLossJPY: 100}})
	setScript(0, map[int]strategy.Signal{10: long(149), 12: long(148)})
	h.poll(10)
	h.poll(11)

	h.feed.quote("148.900", "148.910") // stop out: about -112 JPY
	h.mgr.OnTick(context.Background(), h.feed.tick())
	require.Empty(t, h.openPositions())

	h.poll(13)
	assert.Equal(t, []string{"open:filled", "close:filled", "open:rejected"}, h.store.statuses())
	assert.Empty(t, h.openPositions())
	last := h.store.orders[h.store.orderSeq[2]]
	assert.Contains(t, last.Reason, "daily_loss")
	assert.Contains(t, h.kinds(), "rejected")
}

func TestEntryWithoutStopIsRefused(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: {Action: strategy.EnterLong, StopLoss: 151}}) // stop above price
	h.poll(10)
	h.poll(11)
	assert.Empty(t, h.store.statuses())
	assert.Equal(t, []string{"skipped"}, h.kinds())
}

type denyAll struct{}

func (denyAll) EntriesAllowed(context.Context, string) (bool, string) { return false, "kill switch" }

func TestGuardBlocksEntriesButNotExits(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}, 12: long(149)})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)

	h.mgr.runners["dep-00000001"].guard = denyAll{}
	h.poll(12) // exit still executes
	assert.Empty(t, h.openPositions())
	h.poll(13) // new entry is blocked
	assert.Empty(t, h.openPositions())
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
	assert.Equal(t, []string{"opened", "closed", "skipped"}, h.kinds())
}

func TestVersionSwitchWaitsForPositionToClose(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149), 12: {Action: strategy.Exit}})
	setScript(1, map[int]strategy.Signal{11: short(151), 13: short(151), 14: short(151)})
	h.poll(10)
	h.poll(11) // v1 opens a long

	h.store.active["strat-1"] = "v2" // user activates a new version

	h.poll(12) // bar 11: v2 would go short, but v1 still owns the position → nothing
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, broker.SideBuy, pos[0].Side)

	h.poll(13) // bar 12: v1 exits by its own rule
	assert.Empty(t, h.openPositions())

	h.poll(14) // flat → switch to v2; bar 13 only warms the new strategy up
	assert.Empty(t, h.openPositions())
	assert.True(t, h.logged("switched strategy version v1 -> v2"))

	h.poll(15) // bar 14: v2 trades from the next bar
	pos = h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, broker.SideSell, pos[0].Side)
	assert.Equal(t, "v2", pos[0].StrategyVersionID)
}

func TestDeferredExitIsRetried(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}})
	h.poll(10)
	h.poll(11)

	h.feed.setStatus(market.StatusClosed)
	h.poll(12) // exit signal while the market is closed
	require.Len(t, h.openPositions(), 1, "exit deferred")

	h.feed.setStatus(market.StatusOpen)
	h.feed.mu.Lock()
	h.feed.now = h.feed.now.Add(2 * time.Minute) // no new bar, just the next poll
	h.feed.mu.Unlock()
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Empty(t, h.openPositions(), "retried once the market reopened")
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
}

func TestDisabledDeployment(t *testing.T) {
	t.Run("without a position gets no runner", func(t *testing.T) {
		h := newHarness(t, Config{})
		h.store.deployments[0].Enabled = false
		setScript(0, map[int]strategy.Signal{10: long(149)})
		h.poll(10)
		h.poll(11)
		assert.Empty(t, h.mgr.runners)
		assert.Empty(t, h.store.statuses())
	})

	t.Run("with a position keeps exits and stops, blocks entries", func(t *testing.T) {
		h := newHarness(t, Config{})
		setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}, 12: long(149)})
		h.poll(10)
		h.poll(11)
		require.Len(t, h.openPositions(), 1)

		h.store.deployments[0].Enabled = false
		h.poll(12) // exit honoured
		assert.Empty(t, h.openPositions())
		h.poll(13) // runner is dropped once flat; no new entry
		h.poll(14)
		assert.Empty(t, h.mgr.runners)
		assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
	})
}

func TestRestartRestoresPositionAndBalance(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	before, _ := h.paper.Assets(context.Background())

	h.newManager(Config{}) // bot restart: new broker and manager, same database
	after, _ := h.paper.Assets(context.Background())
	assert.Equal(t, before.Equity.String(), after.Equity.String(), "balance restored (entry fee included)")
	assert.Equal(t, before.AvailableMargin.String(), after.AvailableMargin.String())

	h.poll(12) // warm-up only; the restored position is managed again
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())

	h.feed.quote("148.900", "148.910")
	h.mgr.OnTick(context.Background(), h.feed.tick())
	assert.Empty(t, h.openPositions(), "stop still enforced after restart")
	final, _ := h.paper.Assets(context.Background())
	// 30000 - 0.30002 (entry fee) - 111 - 0.2978 (exit fee)
	assert.Equal(t, "29888.40218", final.Equity.String())
}

func TestUnknownBrokerIsIgnored(t *testing.T) {
	h := newHarness(t, Config{})
	h.store.deployments[0].Broker = "gmo"
	h.poll(10)
	assert.Empty(t, h.mgr.runners)
	assert.True(t, h.logged("not available"))
}
