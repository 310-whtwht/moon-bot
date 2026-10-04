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
}

type memOrder struct {
	Order
	Status string
	Reason string
	Fill   Fill
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

func (s *memStore) InsertPosition(_ context.Context, p Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positions = append(s.positions, &memPosition{Position: p})
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
