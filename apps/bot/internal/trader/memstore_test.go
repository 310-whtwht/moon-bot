package trader

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/moomoo-trading/core/risk"
	"github.com/shopspring/decimal"
)

// memStore is an in-memory Store for tests.
type memStore struct {
	mu          sync.Mutex
	deployments []Deployment
	versions    map[string]Version
	active      map[string]string // strategy ID -> version ID
	orders      map[string]*memOrder
	orderSeq    []string
	positions   []*memPosition
	bars        []BarDecision
}

type memOrder struct {
	Order
	Status        string
	Reason        string
	Fill          Fill
	Working       bool // an accepted limit order waiting for its fill
	BrokerOrderID string
	Limit, Stop   decimal.Decimal
}

type memPosition struct {
	Position
	Closed      bool
	ClosePrice  decimal.Decimal
	RealizedPnL decimal.Decimal
	ClosedAt    time.Time
}

func newMemStore() *memStore {
	return &memStore{versions: map[string]Version{}, active: map[string]string{}, orders: map[string]*memOrder{}}
}

func (s *memStore) Deployments(context.Context) ([]Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Deployment(nil), s.deployments...), nil
}

func (s *memStore) ActiveVersion(_ context.Context, strategyID string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.versions[s.active[strategyID]]
	if !ok {
		return Version{}, sql.ErrNoRows
	}
	return v, nil
}

func (s *memStore) Version(_ context.Context, id string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.versions[id]
	if !ok {
		return Version{}, sql.ErrNoRows
	}
	return v, nil
}

func (s *memStore) OpenPosition(_ context.Context, deploymentID string) (*Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.positions {
		if p.DeploymentID == deploymentID && !p.Closed {
			cp := p.Position
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *memStore) OpenPositions(_ context.Context, brokerName, accountID string) ([]Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Position
	for _, p := range s.positions {
		if p.Broker == brokerName && p.AccountID == accountID && !p.Closed {
			out = append(out, p.Position)
		}
	}
	return out, nil
}

func (s *memStore) CreateOrder(_ context.Context, o Order) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.orders[o.ClientOrderID]; dup {
		return false, nil
	}
	s.orders[o.ClientOrderID] = &memOrder{Order: o, Status: "pending"}
	s.orderSeq = append(s.orderSeq, o.ClientOrderID)
	return true, nil
}

func (s *memStore) MarkOrderFilled(_ context.Context, o Order, f Fill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ClientOrderID].Status = "filled"
	s.orders[o.ClientOrderID].Working = false
	s.orders[o.ClientOrderID].Fill = f
	return nil
}

func (s *memStore) MarkOrderRejected(_ context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[id].Status = "rejected"
	s.orders[id].Reason = reason
	return nil
}

func (s *memStore) MarkOrderUnknown(_ context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[id].Status = "unknown"
	s.orders[id].Reason = reason
	return nil
}

func (s *memStore) InsertPosition(_ context.Context, p Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positions = append(s.positions, &memPosition{Position: p})
	return nil
}

func (s *memStore) SetPositionStopOrder(_ context.Context, id, stopOrderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.positions {
		if p.ID == id {
			p.StopOrderID = stopOrderID
		}
	}
	return nil
}

func (s *memStore) ClosePosition(_ context.Context, id string, price, pnl, _ decimal.Decimal, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.positions {
		if p.ID == id {
			p.Closed, p.ClosePrice, p.RealizedPnL, p.ClosedAt = true, price, pnl, at
		}
	}
	return nil
}

func (s *memStore) Exposure(_ context.Context, brokerName, accountID string, dayStart, weekStart time.Time) (risk.Exposure, risk.Exposure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var account, global risk.Exposure
	for _, p := range s.positions {
		mine := p.Broker == brokerName && p.AccountID == accountID
		if !p.Closed {
			global.OpenPositions++
			if mine {
				account.OpenPositions++
			}
			continue
		}
		pnl := p.RealizedPnL.InexactFloat64()
		if !p.ClosedAt.Before(weekStart) {
			global.WeeklyPnLJPY += pnl
			if mine {
				account.WeeklyPnLJPY += pnl
			}
		}
		if !p.ClosedAt.Before(dayStart) {
			global.DailyPnLJPY += pnl
			if mine {
				account.DailyPnLJPY += pnl
			}
		}
	}
	for _, o := range s.orders {
		if o.Working {
			global.OpenPositions++
			if o.Broker == brokerName && o.AccountID == accountID {
				account.OpenPositions++
			}
		}
	}
	return account, global, nil
}

func (s *memStore) RealizedPnL(_ context.Context, brokerName, accountID string) (decimal.Decimal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := decimal.Zero
	for _, p := range s.positions {
		if p.Broker == brokerName && p.AccountID == accountID && p.Closed {
			total = total.Add(p.RealizedPnL)
		}
	}
	return total, nil
}

// statuses returns order statuses in creation order.
func (s *memStore) statuses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.orderSeq))
	for _, id := range s.orderSeq {
		o := s.orders[id]
		out = append(out, o.SettleType+":"+o.Status)
	}
	return out
}

// --- kill switches and ops (heartbeat, summary) -------------------------------

type memOps struct {
	mu         sync.Mutex
	switches   []KillSwitch
	killErr    error
	heartbeats int
	writes     int
	lastRunner int
	store      *memStore
}

func (o *memOps) KillSwitches(context.Context) ([]KillSwitch, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]KillSwitch(nil), o.switches...), o.killErr
}

func (o *memOps) set(k KillSwitch) {
	o.mu.Lock()
	defer o.mu.Unlock()
	// Like the database, every write moves updated_at forward.
	o.writes++
	k.UpdatedAt = t0.Add(time.Duration(o.writes) * time.Millisecond)
	for i := range o.switches {
		if o.switches[i].Scope == k.Scope {
			o.switches[i] = k
			return
		}
	}
	o.switches = append(o.switches, k)
}

func (o *memOps) Heartbeat(_ context.Context, _ string, _, _ time.Time, _ time.Duration, runners int) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.heartbeats++
	o.lastRunner = runners
	return nil
}

func (o *memOps) ClosedSummary(_ context.Context, from, to time.Time) (int, float64, error) {
	o.store.mu.Lock()
	defer o.store.mu.Unlock()
	var n int
	var pnl float64
	for _, p := range o.store.positions {
		if p.Closed && !p.ClosedAt.Before(from) && p.ClosedAt.Before(to) {
			n++
			pnl += p.RealizedPnL.InexactFloat64()
		}
	}
	return n, pnl, nil
}

// SetKillSwitch lets the manager halt a broker in tests.
func (o *memOps) SetKillSwitch(_ context.Context, k KillSwitch) error {
	o.set(k)
	return nil
}

func (m *memStore) RecordBar(_ context.Context, d BarDecision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// One row per deployment and bar, like the table's unique key.
	for i := range m.bars {
		if m.bars[i].DeploymentID == d.DeploymentID && m.bars[i].BarTime.Equal(d.BarTime) {
			m.bars[i] = d
			return nil
		}
	}
	m.bars = append(m.bars, d)
	return nil
}

func (s *memStore) MarkOrderWorking(_ context.Context, id, brokerOrderID string, limit, stop decimal.Decimal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[id]
	o.Status, o.Working, o.BrokerOrderID, o.Limit, o.Stop = "submitted", true, brokerOrderID, limit, stop
	return nil
}

func (s *memStore) MarkOrderCancelled(_ context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[id]
	o.Status, o.Working, o.Reason = "cancelled", false, reason
	return nil
}

func (s *memStore) WorkingOrders(_ context.Context, deploymentID string) ([]WorkingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []WorkingOrder
	for _, id := range s.orderSeq {
		if o := s.orders[id]; o.Working && o.DeploymentID == deploymentID {
			out = append(out, WorkingOrder{ClientOrderID: id, BrokerOrderID: o.BrokerOrderID})
		}
	}
	return out, nil
}
