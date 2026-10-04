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

	mu      sync.Mutex
	runners map[string]*Runner
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
			cfg: m.Config, now: m.Now, logf: m.Logf,
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
	return nil
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
