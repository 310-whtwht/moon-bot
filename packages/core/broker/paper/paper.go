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
	working    map[string]broker.OpenOrder   // limit orders waiting for their price, by order ID
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
		working:    map[string]broker.OpenOrder{},
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

// PlaceOpen opens a position.
//
// A market order fills immediately: buys at ASK, sells at BID.
//
// A limit order fills when the market reaches its price: a buy when the ASK
// is at or below the limit, a sell when the BID is at or above it. That is
// the conservative reading of a quote-driven market: a buy resting at the
// BID is only filled once the ASK comes down to it. Until then the order is
// working and can be cancelled; it is checked whenever its executions are read.
func (b *Broker) PlaceOpen(ctx context.Context, order broker.OpenOrder) (broker.OrderAck, error) {
	if order.Type != broker.OrderMarket && order.Type != broker.OrderLimit {
		return broker.OrderAck{}, fmt.Errorf("paper: only market and limit orders are supported (got %s)", order.Type)
	}
	if !order.Size.IsPositive() {
		return broker.OrderAck{}, errors.New("paper: size must be positive")
	}
	if order.Type == broker.OrderLimit && (order.Price == nil || !order.Price.IsPositive()) {
		return broker.OrderAck{}, errors.New("paper: limit order needs a price")
	}
	tick, err := b.quote(ctx, order.Symbol)
	if err != nil {
		return broker.OrderAck{}, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.opts.Now().UTC()
	orderID := b.nextID("po")
	price, fillable := fillPrice(order, tick)
	if !fillable {
		b.working[orderID] = order
		return broker.OrderAck{OrderID: orderID, ClientOrderID: order.ClientOrderID, Status: "ORDERED", AcceptedAt: now}, nil
	}
	if err := b.fillOpen(orderID, order, price, now); err != nil {
		return broker.OrderAck{}, err
	}
	return broker.OrderAck{OrderID: orderID, ClientOrderID: order.ClientOrderID, Status: "EXECUTED", AcceptedAt: now}, nil
}

// fillPrice returns the price an opening order fills at on this quote, and
// whether it fills at all.
func fillPrice(order broker.OpenOrder, tick market.Tick) (decimal.Decimal, bool) {
	touch := tick.Ask
	if order.Side == broker.SideSell {
		touch = tick.Bid
	}
	if order.Type == broker.OrderMarket {
		return touch, true
	}
	limit := *order.Price
	if order.Side == broker.SideBuy && touch.LessThanOrEqual(limit) {
		return limit, true
	}
	if order.Side == broker.SideSell && touch.GreaterThanOrEqual(limit) {
		return limit, true
	}
	return decimal.Zero, false
}

// fillOpen books the position and its execution. Caller holds the lock.
func (b *Broker) fillOpen(orderID string, order broker.OpenOrder, price decimal.Decimal, now time.Time) error {
	required := price.Mul(order.Size).Div(b.opts.Leverage)
	if available := b.balance.Sub(b.usedMargin()); required.GreaterThan(available) {
		return fmt.Errorf("paper: insufficient margin (need %s, have %s)", required.StringFixed(0), available.StringFixed(0))
	}
	positionID := b.nextID("pp")
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
	return nil
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

// Cancel withdraws a working limit order. Filled orders cannot be cancelled.
func (b *Broker) Cancel(ctx context.Context, orderID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.working[orderID]; !ok {
		return fmt.Errorf("paper: order %s is not working", orderID)
	}
	delete(b.working, orderID)
	return nil
}

// Executions returns an order's fills. Reading a working limit order checks
// it against the current quote first, filling it if the market got there.
func (b *Broker) Executions(ctx context.Context, orderID string) ([]broker.Execution, error) {
	b.mu.Lock()
	order, working := b.working[orderID]
	b.mu.Unlock()

	if working {
		// No quote (market closed, feed down): the order just keeps waiting.
		if tick, err := b.quote(ctx, order.Symbol); err == nil {
			b.mu.Lock()
			if _, still := b.working[orderID]; still {
				if price, ok := fillPrice(order, tick); ok {
					delete(b.working, orderID)
					// Without margin at fill time the order is dropped, as a broker would reject it.
					_ = b.fillOpen(orderID, order, price, b.opts.Now().UTC())
				}
			}
			b.mu.Unlock()
		}
	}

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
