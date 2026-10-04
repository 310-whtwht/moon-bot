// Package trader runs strategies against a broker: on every closed bar it asks
// the strategy for a signal, checks risk, places orders and records what
// happened. The broker can be the paper broker or a real one; the logic is the
// same.
package trader

import (
	"context"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/shopspring/decimal"
)

// Deployment binds a strategy to an account and instrument. It always runs
// the strategy's active version (see Runner for the switching rule).
type Deployment struct {
	ID         string
	Name       string
	StrategyID string
	Broker     string
	AccountID  string
	Symbol     string
	Timeframe  market.Timeframe
	Units      decimal.Decimal
	Enabled    bool
}

// Version is a strategy version: a registered type plus resolved parameters.
type Version struct {
	ID     string
	Type   string
	Params strategy.Params
}

// Position is an open position owned by a deployment.
type Position struct {
	ID                string
	DeploymentID      string
	Broker            string
	AccountID         string
	BrokerPositionID  string
	Symbol            string
	Side              broker.Side
	Units             decimal.Decimal
	OpenPrice         decimal.Decimal
	StopPrice         decimal.Decimal
	Fees              decimal.Decimal // fees paid so far (entry)
	StrategyID        string
	StrategyVersionID string
	OpenedAt          time.Time
}

// Order is an order the bot is about to send. ClientOrderID is unique, which
// makes order placement idempotent.
type Order struct {
	ClientOrderID     string
	DeploymentID      string
	Broker            string
	AccountID         string
	StrategyID        string
	StrategyVersionID string
	Symbol            string
	Side              broker.Side
	SettleType        string // "open" or "close"
	Units             decimal.Decimal
	BrokerPositionID  string // for closes
}

// Fill is the result of an executed order.
type Fill struct {
	BrokerOrderID    string
	BrokerPositionID string
	Size             decimal.Decimal
	Price            decimal.Decimal
	Fee              decimal.Decimal
	At               time.Time
}

// Store is the persistence the trader needs. Implemented on MySQL and, for
// tests, in memory.
type Store interface {
	// Deployments returns every deployment, enabled or not.
	Deployments(ctx context.Context) ([]Deployment, error)
	ActiveVersion(ctx context.Context, strategyID string) (Version, error)
	Version(ctx context.Context, versionID string) (Version, error)

	OpenPosition(ctx context.Context, deploymentID string) (*Position, error)
	OpenPositions(ctx context.Context, brokerName, accountID string) ([]Position, error)

	// CreateOrder records a pending order. created is false when an order with
	// the same ClientOrderID already exists (the action was already taken).
	CreateOrder(ctx context.Context, o Order) (created bool, err error)
	MarkOrderFilled(ctx context.Context, o Order, f Fill) error
	MarkOrderRejected(ctx context.Context, clientOrderID, reason string) error
	// MarkOrderUnknown records that the order was sent but its outcome could
	// not be determined; it needs a human (or reconciliation) to resolve.
	MarkOrderUnknown(ctx context.Context, clientOrderID, reason string) error

	InsertPosition(ctx context.Context, p Position) error
	// ClosePosition closes a position. realizedPnL is net of all fees.
	ClosePosition(ctx context.Context, positionID string, closePrice, realizedPnL, closeFee decimal.Decimal, at time.Time) error

	// Exposure returns open positions and realised P&L for the account and for all accounts.
	Exposure(ctx context.Context, brokerName, accountID string, dayStart, weekStart time.Time) (account, global risk.Exposure, err error)
	// RealizedPnL is the total net realised P&L of an account (to restore the paper balance).
	RealizedPnL(ctx context.Context, brokerName, accountID string) (decimal.Decimal, error)
}

// Guard can block new entries (kill switch). Exits and stops are never blocked.
type Guard interface {
	EntriesAllowed(ctx context.Context, brokerName string) (allowed bool, reason string)
}

// AllowAll is the default guard.
type AllowAll struct{}

func (AllowAll) EntriesAllowed(context.Context, string) (bool, string) { return true, "" }

// Event is something worth telling a human about.
type Event struct {
	Kind       string // opened, closed, rejected, skipped, error
	Deployment Deployment
	Message    string
	At         time.Time
}

// Notifier receives events (Slack etc.). It must not block.
type Notifier interface {
	Notify(e Event)
}

// NoNotify discards events.
type NoNotify struct{}

func (NoNotify) Notify(Event) {}
