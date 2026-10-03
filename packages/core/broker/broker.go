// Package broker defines the broker-agnostic interfaces every adapter
// (paper, gmofx, ...) implements. Nothing here may depend on a specific broker.
package broker

import (
	"context"
	"errors"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
)

// ErrNoData is returned when the broker has no data for the requested range
// (weekends, holidays, before the broker's history starts).
var ErrNoData = errors.New("no data for requested range")

// BarsRequest asks for bars whose OpenTime is in [From, To).
type BarsRequest struct {
	Symbol    string
	Timeframe market.Timeframe
	PriceType market.PriceType
	From      time.Time
	To        time.Time
}

// MarketData is the read-only, unauthenticated side of a broker.
type MarketData interface {
	// Name is the broker identifier used in InstrumentKey and DB rows (e.g. "gmo").
	Name() string
	Status(ctx context.Context) (market.MarketStatus, error)
	Instruments(ctx context.Context) ([]market.Instrument, error)
	Ticks(ctx context.Context) ([]market.Tick, error)
	Bars(ctx context.Context, req BarsRequest) ([]market.Bar, error)
	// SubscribeTicks streams quotes until ctx is cancelled. The channel is
	// closed when the subscription ends.
	SubscribeTicks(ctx context.Context, symbols []string) (<-chan market.Tick, error)
}

// Side is the direction of an order or position.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// OrderType is the execution type of an order.
type OrderType string

const (
	OrderMarket OrderType = "MARKET"
	OrderLimit  OrderType = "LIMIT"
	OrderStop   OrderType = "STOP"
)

// OpenOrder opens a new position.
type OpenOrder struct {
	ClientOrderID     string
	Symbol            string
	Side              Side
	Type              OrderType
	Size              decimal.Decimal
	Price             *decimal.Decimal
	StrategyVersionID string
}

// CloseOrder closes (part of) an existing position.
type CloseOrder struct {
	ClientOrderID string
	Symbol        string
	PositionID    string
	Side          Side
	Type          OrderType
	Size          decimal.Decimal
	Price         *decimal.Decimal
}

// OrderAck is the broker's acknowledgement of an accepted order.
type OrderAck struct {
	OrderID       string
	ClientOrderID string
	Status        string
	AcceptedAt    time.Time
}

// Position is an open position held at the broker.
type Position struct {
	PositionID string
	Symbol     string
	Side       Side
	Size       decimal.Decimal
	OpenPrice  decimal.Decimal
	OpenedAt   time.Time
}

// Execution is a fill reported by the broker.
type Execution struct {
	ExecutionID string
	OrderID     string
	PositionID  string
	Symbol      string
	Side        Side
	SettleType  string // OPEN or CLOSE
	Size        decimal.Decimal
	Price       decimal.Decimal
	Fee         decimal.Decimal
	RealizedPnL decimal.Decimal
	ExecutedAt  time.Time
}

// Assets is the account balance summary.
type Assets struct {
	Currency        string
	Equity          decimal.Decimal
	AvailableMargin decimal.Decimal
}

// Trading is the authenticated, order-placing side of a broker.
// Implemented by the paper broker (Phase 3) and gmofx (Phase 4).
type Trading interface {
	AccountID() string
	Assets(ctx context.Context) (Assets, error)
	OpenPositions(ctx context.Context, symbol string) ([]Position, error)
	PlaceOpen(ctx context.Context, order OpenOrder) (OrderAck, error)
	PlaceClose(ctx context.Context, order CloseOrder) (OrderAck, error)
	Cancel(ctx context.Context, orderID string) error
	Executions(ctx context.Context, orderID string) ([]Execution, error)
	SubscribeExecutions(ctx context.Context) (<-chan Execution, error)
}

// Broker is a full adapter: market data plus trading.
type Broker interface {
	MarketData
	Trading
}
