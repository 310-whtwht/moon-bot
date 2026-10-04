// Package paper is a simulated broker: it takes real quotes from another
// broker's market data and fills market orders against them, so the bot can
// run end to end without real money. Orders, positions and balance live in
// memory; the bot persists them and restores the broker on start.
package paper

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
)

// BrokerName is stored on orders and positions made by this broker.
const BrokerName = "paper"

// ErrMarketClosed is returned when the quote source reports the market as not open.
var ErrMarketClosed = errors.New("paper: market is closed")

// Options configures the simulation.
type Options struct {
	AccountID      string
	InitialBalance decimal.Decimal // account currency (JPY)
	FeeRate        decimal.Decimal // per fill on notional (GMO FX API: 0.00002)
	Leverage       decimal.Decimal // default 25
	Now            func() time.Time
}

// Broker implements broker.Broker on top of a real market data source.
type Broker struct {
	broker.MarketData // quotes, bars and instruments come from the source

	opts Options

	mu         sync.Mutex
	balance    decimal.Decimal
	positions  map[string]broker.Position
	executions map[string][]broker.Execution // by order ID
	seq        int64
	subs       []chan broker.Execution
}

var _ broker.Broker = (*Broker)(nil)

// New creates a paper broker that fills against source's quotes.
func New(source broker.MarketData, opts Options) *Broker {
	if opts.AccountID == "" {
		opts.AccountID = "default"
	}
	if opts.Leverage.IsZero() {
		opts.Leverage = decimal.NewFromInt(25)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Broker{
		MarketData: source,
		opts:       opts,
		balance:    opts.InitialBalance,
		positions:  map[string]broker.Position{},
		executions: map[string][]broker.Execution{},
	}
}

// Name identifies this broker on orders and positions. Market data keeps the
// source's name (bars are stored under the source broker).
func (b *Broker) Name() string { return BrokerName }

// Source returns the name of the broker providing quotes.
func (b *Broker) Source() string { return b.MarketData.Name() }

func (b *Broker) AccountID() string { return b.opts.AccountID }

// Restore loads state persisted by the bot: the realised balance and the
// positions that were open when it last stopped.
func (b *Broker) Restore(balance decimal.Decimal, positions []broker.Position) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.balance = balance
	b.positions = make(map[string]broker.Position, len(positions))
	for _, p := range positions {
		b.positions[p.PositionID] = p
	}
}

func (b *Broker) nextID(prefix string) string {
	b.seq++
	return fmt.Sprintf("%s-%d-%d", prefix, b.opts.Now().UnixNano(), b.seq)
}

// quote returns the current tick for symbol, refusing closed markets.
func (b *Broker) quote(ctx context.Context, symbol string) (market.Tick, error) {
	ticks, err := b.MarketData.Ticks(ctx)
	if err != nil {
		return market.Tick{}, fmt.Errorf("paper: quotes: %w", err)
	}
	for _, t := range ticks {
		if t.Key.Symbol != symbol {
			continue
		}
		if t.Status != market.StatusOpen {
			return market.Tick{}, fmt.Errorf("%w (%s is %s)", ErrMarketClosed, symbol, t.Status)
		}
		return t, nil
	}
	return market.Tick{}, fmt.Errorf("paper: no quote for %s", symbol)
}

func (b *Broker) usedMargin() decimal.Decimal {
	used := decimal.Zero
	for _, p := range b.positions {
		used = used.Add(p.OpenPrice.Mul(p.Size).Div(b.opts.Leverage))
	}
	return used
}

// Assets reports the realised balance; open positions only reduce the available margin.
func (b *Broker) Assets(ctx context.Context) (broker.Assets, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return broker.Assets{
		Currency:        "JPY",
		Equity:          b.balance,
		AvailableMargin: b.balance.Sub(b.usedMargin()),
	}, nil
}

func (b *Broker) OpenPositions(ctx context.Context, symbol string) ([]broker.Position, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []broker.Position
	for _, p := range b.positions {
		if symbol == "" || p.Symbol == symbol {
			out = append(out, p)
		}
	}
	return out, nil
}

// PlaceOpen fills a market order immediately: buys at ASK, sells at BID.
func (b *Broker) PlaceOpen(ctx context.Context, order broker.OpenOrder) (broker.OrderAck, error) {
	if order.Type != broker.OrderMarket {
		return broker.OrderAck{}, fmt.Errorf("paper: only market orders are supported (got %s)", order.Type)
	}
	if !order.Size.IsPositive() {
		return broker.OrderAck{}, errors.New("paper: size must be positive")
	}
	tick, err := b.quote(ctx, order.Symbol)
	if err != nil {
		return broker.OrderAck{}, err
	}
	price := tick.Ask
	if order.Side == broker.SideSell {
		price = tick.Bid
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	required := price.Mul(order.Size).Div(b.opts.Leverage)
	if available := b.balance.Sub(b.usedMargin()); required.GreaterThan(available) {
		return broker.OrderAck{}, fmt.Errorf("paper: insufficient margin (need %s, have %s)", required.StringFixed(0), available.StringFixed(0))
	}

	now := b.opts.Now().UTC()
	orderID, positionID := b.nextID("po"), b.nextID("pp")
	fee := price.Mul(order.Size).Mul(b.opts.FeeRate)
	b.balance = b.balance.Sub(fee)
	b.positions[positionID] = broker.Position{
		PositionID: positionID, Symbol: order.Symbol, Side: order.Side,
		Size: order.Size, OpenPrice: price, OpenedAt: now,
	}
	b.record(broker.Execution{
		ExecutionID: b.nextID("pe"), OrderID: orderID, PositionID: positionID,
		Symbol: order.Symbol, Side: order.Side, SettleType: "OPEN",
		Size: order.Size, Price: price, Fee: fee, ExecutedAt: now,
	})
	return broker.OrderAck{OrderID: orderID, ClientOrderID: order.ClientOrderID, Status: "EXECUTED", AcceptedAt: now}, nil
}

// PlaceClose closes a whole position at market: longs sell at BID, shorts buy at ASK.
func (b *Broker) PlaceClose(ctx context.Context, order broker.CloseOrder) (broker.OrderAck, error) {
	if order.Type != broker.OrderMarket {
		return broker.OrderAck{}, fmt.Errorf("paper: only market orders are supported (got %s)", order.Type)
	}
	tick, err := b.quote(ctx, order.Symbol)
	if err != nil {
		return broker.OrderAck{}, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	pos, ok := b.positions[order.PositionID]
	if !ok {
		return broker.OrderAck{}, fmt.Errorf("paper: position %s not found", order.PositionID)
	}
	if !order.Size.Equal(pos.Size) {
		return broker.OrderAck{}, errors.New("paper: partial close is not supported")
	}

	price, closeSide := tick.Bid, broker.SideSell
	pnl := price.Sub(pos.OpenPrice).Mul(pos.Size)
	if pos.Side == broker.SideSell {
		price, closeSide = tick.Ask, broker.SideBuy
		pnl = pos.OpenPrice.Sub(price).Mul(pos.Size)
	}

	now := b.opts.Now().UTC()
	orderID := b.nextID("po")
	fee := price.Mul(pos.Size).Mul(b.opts.FeeRate)
	b.balance = b.balance.Add(pnl).Sub(fee)
	delete(b.positions, order.PositionID)
	b.record(broker.Execution{
		ExecutionID: b.nextID("pe"), OrderID: orderID, PositionID: order.PositionID,
		Symbol: order.Symbol, Side: closeSide, SettleType: "CLOSE",
		Size: pos.Size, Price: price, Fee: fee, RealizedPnL: pnl, ExecutedAt: now,
	})
	return broker.OrderAck{OrderID: orderID, ClientOrderID: order.ClientOrderID, Status: "EXECUTED", AcceptedAt: now}, nil
}

// record stores an execution and notifies subscribers. Caller holds the lock.
func (b *Broker) record(e broker.Execution) {
	b.executions[e.OrderID] = append(b.executions[e.OrderID], e)
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default: // never block trading on a slow subscriber
		}
	}
}

// Cancel is a no-op: market orders fill immediately, so nothing is ever pending.
func (b *Broker) Cancel(ctx context.Context, orderID string) error {
	return fmt.Errorf("paper: order %s is not cancellable", orderID)
}

func (b *Broker) Executions(ctx context.Context, orderID string) ([]broker.Execution, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]broker.Execution(nil), b.executions[orderID]...), nil
}

func (b *Broker) SubscribeExecutions(ctx context.Context) (<-chan broker.Execution, error) {
	ch := make(chan broker.Execution, 64)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch, nil
}
