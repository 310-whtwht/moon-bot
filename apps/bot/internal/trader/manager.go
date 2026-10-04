package trader

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/risk"
)

// Manager runs one Runner per deployment and feeds them bars and quotes.
type Manager struct {
	Store    Store
	Brokers  map[string]broker.Broker // by deployment broker name (e.g. "paper")
	Guard    Guard
	Notifier Notifier
	Config   Config
	Now      func() time.Time
	Logf     func(format string, args ...any)

	// Kills, when set, makes the manager close positions for kill switches
	// activated with close_positions, and report switch changes.
	Kills KillStore
	// Ops, when set, receives heartbeats and provides the daily summary.
	Ops OpsStore
	// Instance identifies this bot in heartbeats.
	Instance     string
	PollInterval time.Duration

	mu          sync.Mutex
	runners     map[string]*Runner
	startedAt   time.Time
	activeKills map[string]bool
	summaryDay  time.Time
}

// OpsStore is the operational bookkeeping around trading.
type OpsStore interface {
	Heartbeat(ctx context.Context, instance string, startedAt, now time.Time, pollInterval time.Duration, runners int) error
	// ClosedSummary returns the number of positions closed in [from, to) and their net P&L.
	ClosedSummary(ctx context.Context, from, to time.Time) (closed int, pnlJPY float64, err error)
}

func (m *Manager) defaults() {
	if m.Guard == nil {
		m.Guard = AllowAll{}
	}
	if m.Notifier == nil {
		m.Notifier = NoNotify{}
	}
	if m.Now == nil {
		m.Now = time.Now
	}
	if m.Logf == nil {
		m.Logf = log.Printf
	}
	if m.runners == nil {
		m.runners = map[string]*Runner{}
	}
	if m.activeKills == nil {
		m.activeKills = map[string]bool{}
	}
	if m.startedAt.IsZero() {
		m.startedAt = m.Now()
	}
	m.Config = m.Config.withDefaults()
}

// sync reconciles runners with the deployments table. A disabled deployment
// keeps its runner while it still holds a position, so the position is still
// stopped out or exited; it just cannot open new ones.
func (m *Manager) sync(ctx context.Context) ([]*Runner, error) {
	deployments, err := m.Store.Deployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("load deployments: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults()

	seen := map[string]bool{}
	for _, d := range deployments {
		seen[d.ID] = true
		if r, ok := m.runners[d.ID]; ok {
			r.SetDeployment(d)
			continue
		}
		br, ok := m.Brokers[d.Broker]
		if !ok {
			m.Logf("%s: broker %q is not available, deployment ignored", d.Name, d.Broker)
			continue
		}
		if !d.Enabled {
			// Only worth a runner if a position is still open from before.
			pos, err := m.Store.OpenPosition(ctx, d.ID)
			if err != nil {
				return nil, err
			}
			if pos == nil {
				continue
			}
		}
		m.runners[d.ID] = &Runner{
			dep: d, broker: br, store: m.Store, guard: m.Guard, notifier: m.Notifier,
			cfg: m.Config, now: m.Now, logf: m.Logf, halt: m.haltBroker,
		}
		m.Logf("%s: runner started (%s %s %s, %s units, enabled=%v)",
			d.Name, d.Broker, d.Symbol, d.Timeframe, d.Units.String(), d.Enabled)
	}

	var active []*Runner
	for id, r := range m.runners {
		r.mu.Lock()
		drop := !seen[id] || (!r.dep.Enabled && r.loaded && r.pos == nil)
		r.mu.Unlock()
		if drop {
			delete(m.runners, id)
			m.Logf("runner %s stopped", id)
			continue
		}
		active = append(active, r)
	}
	sort.Slice(active, func(i, j int) bool { return active[i].dep.ID < active[j].dep.ID })
	return active, nil
}

// KillSetter can activate a kill switch (implemented by the MySQL store).
type KillSetter interface {
	SetKillSwitch(ctx context.Context, k KillSwitch) error
}

// haltBroker turns the broker's kill switch on. It is what a runner calls
// when it can no longer trust its picture of an order.
func (m *Manager) haltBroker(ctx context.Context, brokerName, reason string) {
	setter, ok := m.Kills.(KillSetter)
	if !ok {
		m.Logf("trader: cannot halt %s (no kill switch store): %s", brokerName, reason)
		return
	}
	err := setter.SetKillSwitch(ctx, KillSwitch{Scope: brokerName, Active: true, Reason: reason, UpdatedBy: "bot"})
	if err != nil {
		m.Logf("trader: halt %s failed: %v", brokerName, err)
	}
}

// PollOnce syncs deployments and lets every runner process newly closed bars.
// One failing runner does not stop the others.
func (m *Manager) PollOnce(ctx context.Context) error {
	runners, err := m.sync(ctx)
	if err != nil {
		return err
	}
	for _, r := range runners {
		if err := r.Poll(ctx); err != nil {
			m.Logf("%s: poll failed: %v", r.dep.Name, err)
		}
	}
	m.checkStopsFromREST(ctx, runners)
	m.applyKillSwitches(ctx, runners)
	m.housekeeping(ctx, len(runners))
	return nil
}

// applyKillSwitches reports switch changes and, for switches activated with
// close_positions, closes the positions they cover. (New entries are blocked
// separately, by the Guard, right before each order.)
func (m *Manager) applyKillSwitches(ctx context.Context, runners []*Runner) {
	if m.Kills == nil {
		return
	}
	switches, err := m.Kills.KillSwitches(ctx)
	if err != nil {
		m.Logf("trader: kill switches: %v", err)
		return
	}

	m.mu.Lock()
	for _, k := range switches {
		if k.Active != m.activeKills[k.Scope] {
			m.activeKills[k.Scope] = k.Active
			state := "解除"
			if k.Active {
				state = "発動"
				if k.ClosePositions {
					state += "（全決済）"
				}
			}
			msg := fmt.Sprintf("Kill Switch %s: %s", k.Scope, state)
			if k.Reason != "" {
				msg += " — " + k.Reason
			}
			m.Logf("%s", msg)
			m.Notifier.Notify(Event{Kind: "kill_switch", Message: msg, At: m.Now()})
		}
	}
	m.mu.Unlock()

	for _, r := range runners {
		for _, k := range switches {
			if k.covers(r.dep.Broker) && k.ClosePositions && r.HasPosition() {
				if err := r.ForceClose(ctx, "kill switch"); err != nil {
					m.Logf("%s: kill switch close failed: %v", r.dep.Name, err)
				}
				break
			}
		}
	}
}

// housekeeping writes the heartbeat and sends the daily summary once the
// trading day (06:00 JST) rolls over.
func (m *Manager) housekeeping(ctx context.Context, runners int) {
	if m.Ops == nil {
		return
	}
	now := m.Now()
	if err := m.Ops.Heartbeat(ctx, m.Instance, m.startedAt, now, m.PollInterval, runners); err != nil {
		m.Logf("trader: heartbeat: %v", err)
	}

	today := risk.TradingDayStart(now)
	m.mu.Lock()
	previous := m.summaryDay
	m.summaryDay = today
	m.mu.Unlock()
	if previous.IsZero() || !today.After(previous) {
		return
	}
	closed, pnl, err := m.Ops.ClosedSummary(ctx, previous, today)
	if err != nil {
		m.Logf("trader: daily summary: %v", err)
		return
	}
	msg := fmt.Sprintf("日次サマリ（%s 〜）: 決済 %d 件、損益 %+.0f 円",
		previous.In(time.FixedZone("JST", 9*60*60)).Format("1/2 15:04"), closed, pnl)
	m.Logf("%s", msg)
	m.Notifier.Notify(Event{Kind: "daily_summary", Message: msg, At: now})
}

// checkStopsFromREST is the safety net for the quote stream: every poll it
// fetches current quotes over REST and runs the stop-loss check, so a silent
// WebSocket outage cannot leave a position unprotected for long.
func (m *Manager) checkStopsFromREST(ctx context.Context, runners []*Runner) {
	checked := map[string]bool{}
	for _, r := range runners {
		if !r.HasPosition() || checked[r.dep.Broker] {
			continue
		}
		checked[r.dep.Broker] = true
		ticks, err := r.broker.Ticks(ctx)
		if err != nil {
			m.Logf("trader: quotes for stop check: %v", err)
			continue
		}
		for _, tick := range ticks {
			m.OnTick(ctx, tick)
		}
	}
}

// OnTick forwards a quote to every runner (stop-loss checks).
func (m *Manager) OnTick(ctx context.Context, tick market.Tick) {
	m.mu.Lock()
	runners := make([]*Runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	for _, r := range runners {
		if err := r.OnTick(ctx, tick); err != nil {
			m.Logf("%s: stop check failed: %v", r.dep.Name, err)
		}
	}
}

// OnExecution reacts to a fill reported by a broker's execution stream: the
// runners trading that symbol on that broker reconcile at once, so a stop
// order executed at the broker is settled within moments rather than at the
// next periodic check. The bot's own fills arrive here too; reconciling after
// them is harmless.
func (m *Manager) OnExecution(ctx context.Context, brokerName string, e broker.Execution) {
	m.mu.Lock()
	var affected []*Runner
	for _, r := range m.runners {
		if r.dep.Broker == brokerName && r.dep.Symbol == e.Symbol {
			affected = append(affected, r)
		}
	}
	m.mu.Unlock()

	for _, r := range affected {
		if err := r.Reconcile(ctx); err != nil {
			m.Logf("%s: reconcile after execution: %v", r.dep.Name, err)
		}
	}
}

// WatchExecutions forwards a broker's execution stream to OnExecution until
// the stream ends or ctx is cancelled.
func (m *Manager) WatchExecutions(ctx context.Context, brokerName string, events <-chan broker.Execution) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok {
				m.Logf("trader: %s execution stream ended; relying on periodic reconciliation", brokerName)
				return
			}
			m.OnExecution(ctx, brokerName, e)
		}
	}
}

// Run polls on an interval and watches quotes until ctx is cancelled.
// Stop-losses are enforced from quotes, so pass the broker's tick stream for
// every symbol that can hold a position.
func (m *Manager) Run(ctx context.Context, interval time.Duration, quotes <-chan market.Tick) error {
	m.mu.Lock()
	m.defaults()
	m.mu.Unlock()

	if err := m.PollOnce(ctx); err != nil {
		m.Logf("trader: %v", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.PollOnce(ctx); err != nil {
				m.Logf("trader: %v", err)
			}
		case tick, ok := <-quotes:
			if !ok {
				quotes = nil // stream ended; keep polling
				m.Logf("trader: quote stream ended; stop-loss checks are no longer running")
				continue
			}
			m.OnTick(ctx, tick)
		}
	}
}
