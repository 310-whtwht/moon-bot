package trader

import (
	"context"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *feed) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// tickNow moves the clock and delivers the current quote, as the stream would.
func (h *harness) tickAfter(d time.Duration) {
	h.t.Helper()
	h.feed.advance(d)
	h.mgr.OnTick(context.Background(), h.feed.tick())
}

// limitHarness is a harness whose deployment enters with limit orders. A long
// signal fires on bar 10, so it is acted on by poll(11).
func limitHarness(t *testing.T, cfg Config, tune func(*Deployment)) *harness {
	h := newHarness(t, cfg)
	d := &h.store.deployments[0]
	d.EntryOrder, d.LimitWait = EntryLimit, 30*time.Second
	if tune != nil {
		tune(d)
	}
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	return h
}

func TestLimitEntryWaitsAndFillsAtItsPrice(t *testing.T) {
	h := limitHarness(t, Config{}, nil)
	h.poll(11) // BID 150.000 / ASK 150.010: a buy rests at the BID

	assert.Empty(t, h.openPositions(), "not filled while the ASK is above the limit")
	assert.Equal(t, []string{"open:submitted"}, h.store.statuses())
	assert.True(t, h.logged("limit entry BUY 100 USD_JPY @ 150 is working"))
	assert.Empty(t, h.kinds(), "waiting is not worth a notification")
	require.Len(t, h.store.bars, 1)
	assert.Contains(t, h.store.bars[0].Result, "pending: limit BUY 100 USD_JPY @ 150, waiting up to 30s")

	h.tickAfter(5 * time.Second)
	assert.Empty(t, h.openPositions(), "still waiting")

	// The ASK comes down to the limit.
	h.feed.quote("149.985", "149.995")
	h.tickAfter(5 * time.Second)
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, "150", pos[0].OpenPrice.String(), "filled at the limit, not at the ASK it would have paid")
	assert.Equal(t, "148.985", pos[0].StopPrice.String(), "the stop keeps its distance from the BID at fill time")
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())
	assert.Equal(t, []string{"opened"}, h.kinds())
	assert.Regexp(t, `^pending: .* / opened: BUY 100 USD_JPY @ 150, `, h.store.bars[0].Result,
		"the fill is added to the record of the bar the entry came from")

	// Nothing is left waiting: later ticks only watch the stop.
	h.tickAfter(time.Minute)
	assert.Len(t, h.openPositions(), 1)
	assert.Len(t, h.store.orders, 1)
}

func TestLimitEntryIsCancelledWhenItsTimeIsUp(t *testing.T) {
	h := limitHarness(t, Config{}, nil)
	h.poll(11)

	h.tickAfter(29 * time.Second)
	assert.Equal(t, []string{"open:submitted"}, h.store.statuses())

	h.tickAfter(2 * time.Second)
	assert.Empty(t, h.openPositions())
	assert.Equal(t, []string{"open:cancelled"}, h.store.statuses())
	assert.Equal(t, []string{"cancelled"}, h.kinds())
	assert.Contains(t, h.store.bars[0].Result, "cancelled: limit BUY USD_JPY @ 150: not filled within 30s")

	// The price arriving afterwards changes nothing: the order is gone.
	h.feed.quote("149.985", "149.995")
	h.tickAfter(5 * time.Second)
	assert.Empty(t, h.openPositions())
	assert.Len(t, h.store.orders, 1)
}

func TestLimitEntryFallsBackToMarketWhenAskedTo(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitFallback = FallbackMarket })
	h.poll(11)
	h.feed.quote("150.020", "150.030") // the market moved away
	h.tickAfter(31 * time.Second)

	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, "150.03", pos[0].OpenPrice.String(), "bought at the ASK after the limit was withdrawn")
	assert.Equal(t, []string{"open:cancelled", "open:filled"}, h.store.statuses())
	assert.Equal(t, []string{"cancelled", "opened"}, h.kinds())
	assert.Regexp(t, `^pending: .* / cancelled: .* / opened: BUY 100 USD_JPY @ 150.03`, h.store.bars[0].Result)
}

func TestNewSignalReplacesAWaitingLimitEntry(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitWait = 2 * time.Hour })
	setScript(0, map[int]strategy.Signal{10: long(149), 11: short(151)})
	h.poll(11) // buy limit at 150.000, waiting
	require.Equal(t, []string{"open:submitted"}, h.store.statuses())

	h.poll(12) // bar 11's short signal: the buy is withdrawn, a sell rests at the ASK
	assert.Equal(t, []string{"open:cancelled", "open:submitted"}, h.store.statuses())
	assert.Empty(t, h.openPositions())
	assert.Contains(t, h.store.bars[0].Result, "withdrawn before it filled")
	assert.Contains(t, h.store.bars[1].Result, "pending: limit SELL 100 USD_JPY @ 150.01")

	h.feed.quote("150.010", "150.020") // the BID rises to the sell limit
	h.tickAfter(5 * time.Second)
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, broker.SideSell, pos[0].Side)
	assert.Equal(t, "150.01", pos[0].OpenPrice.String())
}

func TestWaitingLimitEntryTakesAPositionSlot(t *testing.T) {
	second := dep()
	second.ID, second.Name, second.Symbol = "dep-00000002", "test gbp", "GBP_JPY"
	second.EntryOrder, second.LimitWait = EntryLimit, time.Hour

	h := newHarness(t, Config{AccountLimits: risk.Limits{MaxOpenPositions: 1}})
	h.store.deployments[0].EntryOrder, h.store.deployments[0].LimitWait = EntryLimit, time.Hour
	h.store.deployments = append(h.store.deployments, second)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)

	// Both would fill later; only one may be sent.
	assert.ElementsMatch(t, []string{"open:submitted", "open:rejected"}, h.store.statuses())
	assert.Contains(t, h.kinds(), "rejected")
}

func TestLimitEntryLeftWorkingIsWithdrawnAtStartUp(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitWait = time.Hour })
	h.poll(11)
	require.Equal(t, []string{"open:submitted"}, h.store.statuses())

	h.newManager(Config{}) // the bot restarts; the paper broker forgets the order
	h.poll(11)
	assert.Equal(t, []string{"open:cancelled"}, h.store.statuses())
	assert.True(t, h.logged("withdrew limit entry"))
	assert.Empty(t, h.openPositions())

	h.feed.quote("149.985", "149.995")
	h.tickAfter(5 * time.Second)
	assert.Empty(t, h.openPositions(), "nothing fills after the restart")
}

func TestKillSwitchWithCloseWithdrawsAWaitingEntry(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitWait = time.Hour })
	ops := withOps(h)
	h.poll(11)
	require.Equal(t, []string{"open:submitted"}, h.store.statuses())

	ops.set(KillSwitch{Scope: GlobalScope, Active: true, ClosePositions: true})
	h.feed.advance(time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Equal(t, []string{"open:cancelled"}, h.store.statuses())
	assert.Empty(t, h.openPositions())
}

func TestEntryIsSkippedWhileTheSpreadIsTooWide(t *testing.T) {
	h := newHarness(t, Config{})
	h.store.deployments[0].MaxSpread = d("0.008")
	setScript(0, map[int]strategy.Signal{10: long(149), 12: long(149)})
	h.poll(10)

	h.poll(11) // spread 0.010 > 0.008
	assert.Empty(t, h.openPositions())
	assert.Empty(t, h.store.orders, "no order is even recorded")
	assert.Equal(t, []string{"skipped"}, h.kinds())
	assert.Contains(t, h.store.bars[0].Result, "skipped: spread 0.01 is wider than the limit 0.008")

	h.feed.quote("150.000", "150.005") // back to normal
	h.poll(12)
	h.poll(13)
	assert.Len(t, h.openPositions(), 1)
}

func TestDeletedDeploymentWithdrawsItsWaitingEntry(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitWait = time.Hour })
	h.poll(11)
	require.Equal(t, []string{"open:submitted"}, h.store.statuses())

	h.store.deployments = nil // deleted
	h.feed.advance(time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Equal(t, []string{"open:cancelled"}, h.store.statuses())

	h.feed.quote("149.985", "149.995")
	h.tickAfter(5 * time.Second)
	assert.Empty(t, h.openPositions())
}

func TestVersionSwitchWaitsForAWaitingEntry(t *testing.T) {
	h := limitHarness(t, Config{}, func(d *Deployment) { d.LimitWait = time.Hour })
	h.poll(11)
	h.store.active["strat-1"] = "v2"
	h.feed.advance(time.Minute)
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.False(t, h.logged("switched strategy version"))

	h.feed.quote("149.985", "149.995")
	h.tickAfter(5 * time.Second)
	pos := h.openPositions()
	require.Len(t, pos, 1)
	assert.Equal(t, "v1", pos[0].StrategyVersionID, "the position belongs to the version that asked for it")
}
