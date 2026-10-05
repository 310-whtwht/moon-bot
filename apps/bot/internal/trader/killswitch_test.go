package trader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwitchGuard(t *testing.T) {
	ctx := context.Background()
	ops := &memOps{}
	g := SwitchGuard{Store: ops}

	ok, _ := g.EntriesAllowed(ctx, "paper")
	assert.True(t, ok, "no switches")

	ops.set(KillSwitch{Scope: "gmo", Active: true})
	ok, _ = g.EntriesAllowed(ctx, "paper")
	assert.True(t, ok, "another broker's switch does not apply")

	ops.set(KillSwitch{Scope: "paper", Active: true, Reason: "testing"})
	ok, why := g.EntriesAllowed(ctx, "paper")
	assert.False(t, ok)
	assert.Contains(t, why, "paper")
	assert.Contains(t, why, "testing")

	ops.set(KillSwitch{Scope: "paper", Active: false})
	ops.set(KillSwitch{Scope: GlobalScope, Active: true})
	ok, why = g.EntriesAllowed(ctx, "paper")
	assert.False(t, ok, "global covers every broker")
	assert.Contains(t, why, "global")

	ops.set(KillSwitch{Scope: GlobalScope, Active: false})
	ops.set(KillSwitch{Scope: "gmo", Active: false})
	ok, _ = g.EntriesAllowed(ctx, "paper")
	assert.True(t, ok, "released")

	ops.killErr = errors.New("db down")
	ok, why = g.EntriesAllowed(ctx, "paper")
	assert.False(t, ok, "unknown state fails closed")
	assert.Contains(t, why, "unknown")
	ops.killErr = nil

	ok, why = SwitchGuard{Store: ops, EnvActive: true}.EntriesAllowed(ctx, "paper")
	assert.False(t, ok)
	assert.Contains(t, why, "environment")
}

// withOps attaches kill switches and ops to the harness manager.
func withOps(h *harness) *memOps {
	ops := &memOps{store: h.store}
	h.mgr.Kills, h.mgr.Ops = ops, ops
	h.mgr.Guard = SwitchGuard{Store: ops}
	h.mgr.Instance, h.mgr.PollInterval = "test", 30*time.Second
	return ops
}

func TestKillSwitchBlocksEntryAndNotifiesOnChange(t *testing.T) {
	h := newHarness(t, Config{})
	ops := withOps(h)
	setScript(0, map[int]strategy.Signal{10: long(149), 12: long(149)})
	h.poll(10)

	ops.set(KillSwitch{Scope: GlobalScope, Active: true, Reason: "様子見"})
	h.poll(11) // entry signal while the switch is on
	assert.Empty(t, h.store.statuses())
	assert.Equal(t, []string{"skipped", "kill_switch"}, h.kinds())
	assert.Contains(t, h.events[1].Message, "発動")
	assert.Contains(t, h.events[1].Message, "様子見")

	h.poll(12) // unchanged switch is not announced again
	assert.Len(t, h.events, 2)

	ops.set(KillSwitch{Scope: GlobalScope, Active: false})
	h.poll(13) // released: bar 12's entry goes through
	assert.Equal(t, []string{"open:filled"}, h.store.statuses())
	assert.Contains(t, h.kinds(), "kill_switch")
	assert.Contains(t, h.events[len(h.events)-1].Message, "解除")
}

func TestKillSwitchToggledBetweenPollsIsStillReported(t *testing.T) {
	h := newHarness(t, Config{})
	ops := withOps(h)
	h.poll(10)
	require.Empty(t, h.events, "the state at start-up is the baseline, not news")

	// On and off again before the bot looks: it never sees the switch active.
	ops.set(KillSwitch{Scope: GlobalScope, Active: true})
	ops.set(KillSwitch{Scope: GlobalScope, Active: false})
	h.poll(11)
	require.Equal(t, []string{"kill_switch"}, h.kinds())
	assert.Contains(t, h.events[0].Message, "現在は解除されています")

	h.poll(12) // not repeated
	assert.Len(t, h.events, 1)
}

func TestStartIsAnnouncedWithWhatWasPickedUp(t *testing.T) {
	h := newHarness(t, Config{})
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)

	h.newManager(Config{}) // restart
	h.poll(12)
	h.events = nil
	h.mgr.announceStart()
	require.Equal(t, []string{"started"}, h.kinds())
	assert.Contains(t, h.events[0].Message, "稼働中の割り当て 1 件、保有中の建玉 1 件")
	assert.Contains(t, FormatEvent(h.events[0]), "起動")
}

func TestKillSwitchWithClosePositions(t *testing.T) {
	h := newHarness(t, Config{})
	ops := withOps(h)
	setScript(0, map[int]strategy.Signal{10: long(149)})
	h.poll(10)
	h.poll(11)
	require.Len(t, h.openPositions(), 1)

	// Without close_positions the position is kept (and still protected by its stop).
	ops.set(KillSwitch{Scope: "paper", Active: true})
	h.poll(12)
	assert.Len(t, h.openPositions(), 1)

	ops.set(KillSwitch{Scope: "paper", Active: true, ClosePositions: true})
	h.feed.mu.Lock()
	h.feed.now = h.feed.now.Add(30 * time.Second)
	h.feed.mu.Unlock()
	require.NoError(t, h.mgr.PollOnce(context.Background()))
	assert.Empty(t, h.openPositions(), "closed by the kill switch")
	assert.Equal(t, []string{"open:filled", "close:filled"}, h.store.statuses())
	assert.Contains(t, h.events[len(h.events)-1].Message, "kill switch")
}

func TestHeartbeatAndDailySummary(t *testing.T) {
	h := newHarness(t, Config{})
	ops := withOps(h)
	setScript(0, map[int]strategy.Signal{10: long(149), 11: {Action: strategy.Exit}})
	h.poll(10)
	h.poll(11)
	h.feed.quote("150.500", "150.510")
	h.poll(12) // exit at 150.500: +49 gross, about +48 net
	assert.Equal(t, 3, ops.heartbeats)
	assert.Equal(t, 1, ops.lastRunner)
	assert.NotContains(t, h.kinds(), "daily_summary", "same trading day")

	h.poll(22) // 22:00 UTC is past the 21:00 UTC (06:00 JST) rollover
	require.Contains(t, h.kinds(), "daily_summary")
	summary := h.events[len(h.events)-1].Message
	assert.Contains(t, summary, "決済 1 件")
	assert.Contains(t, summary, "+48 円")

	h.poll(23) // sent once per day
	count := 0
	for _, k := range h.kinds() {
		if k == "daily_summary" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}
