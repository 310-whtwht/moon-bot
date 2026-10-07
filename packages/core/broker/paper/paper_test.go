package paper

import (
	"context"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// quotes is a market data source with a settable USD_JPY tick.
type quotes struct {
	bid, ask string
	status   market.MarketStatus
}

func (q *quotes) Name() string { return "gmo" }
func (q *quotes) Status(context.Context) (market.MarketStatus, error) {
	return q.status, nil
}
func (q *quotes) Instruments(context.Context) ([]market.Instrument, error) { return nil, nil }
func (q *quotes) Ticks(context.Context) ([]market.Tick, error) {
	return []market.Tick{{
		Key: market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"},
		Bid: d(q.bid), Ask: d(q.ask), Status: q.status,
	}}, nil
}
func (q *quotes) Bars(context.Context, broker.BarsRequest) ([]market.Bar, error) { return nil, nil }
func (q *quotes) SubscribeTicks(context.Context, []string) (<-chan market.Tick, error) {
	return nil, nil
}

func newBroker(q *quotes, balance string) *Broker {
	return New(q, Options{
		InitialBalance: d(balance), FeeRate: d("0.00002"),
		Now: func() time.Time { return time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC) },
	})
}

func marketOpen(symbol string, side broker.Side, size string) broker.OpenOrder {
	return broker.OpenOrder{ClientOrderID: "c1", Symbol: symbol, Side: side, Type: broker.OrderMarket, Size: d(size)}
}

func TestLongRoundTrip(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusOpen}
	b := newBroker(q, "30000")
	assert.Equal(t, "paper", b.Name())
	assert.Equal(t, "gmo", b.Source())

	ack, err := b.PlaceOpen(ctx, marketOpen("USD_JPY", broker.SideBuy, "100"))
	require.NoError(t, err)
	assert.Equal(t, "c1", ack.ClientOrderID)

	execs, err := b.Executions(ctx, ack.OrderID)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	assert.Equal(t, "150.01", execs[0].Price.String(), "buy fills at ASK")
	assert.Equal(t, "0.30002", execs[0].Fee.String(), "150.01 * 100 * 0.00002")

	positions, _ := b.OpenPositions(ctx, "USD_JPY")
	require.Len(t, positions, 1)
	pos := positions[0]

	assets, _ := b.Assets(ctx)
	assert.Equal(t, "29999.69998", assets.Equity.String(), "entry fee deducted")
	// margin = 150.01 * 100 / 25 = 600.04
	assert.Equal(t, "29399.65998", assets.AvailableMargin.String())

	q.bid, q.ask = "151.000", "151.010"
	closeAck, err := b.PlaceClose(ctx, broker.CloseOrder{Symbol: "USD_JPY", PositionID: pos.PositionID,
		Side: broker.SideSell, Type: broker.OrderMarket, Size: pos.Size})
	require.NoError(t, err)

	execs, _ = b.Executions(ctx, closeAck.OrderID)
	require.Len(t, execs, 1)
	assert.Equal(t, "151", execs[0].Price.String(), "long closes at BID")
	assert.Equal(t, "99", execs[0].RealizedPnL.String(), "(151.000 - 150.010) * 100")
	assert.Equal(t, "CLOSE", execs[0].SettleType)

	assets, _ = b.Assets(ctx)
	// 30000 - 0.30002 + 99 - 0.302
	assert.Equal(t, "30098.39798", assets.Equity.String())
	assert.True(t, assets.Equity.Equal(assets.AvailableMargin))
	positions, _ = b.OpenPositions(ctx, "")
	assert.Empty(t, positions)
}

func TestShortFillsAtBidAndClosesAtAsk(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusOpen}
	b := newBroker(q, "30000")

	ack, err := b.PlaceOpen(ctx, marketOpen("USD_JPY", broker.SideSell, "100"))
	require.NoError(t, err)
	execs, _ := b.Executions(ctx, ack.OrderID)
	assert.Equal(t, "150", execs[0].Price.String(), "sell fills at BID")

	q.bid, q.ask = "149.000", "149.010"
	positions, _ := b.OpenPositions(ctx, "USD_JPY")
	closeAck, err := b.PlaceClose(ctx, broker.CloseOrder{Symbol: "USD_JPY", PositionID: positions[0].PositionID,
		Side: broker.SideBuy, Type: broker.OrderMarket, Size: d("100")})
	require.NoError(t, err)
	execs, _ = b.Executions(ctx, closeAck.OrderID)
	assert.Equal(t, "149.01", execs[0].Price.String(), "short closes at ASK")
	assert.Equal(t, "99", execs[0].RealizedPnL.String(), "(150.000 - 149.010) * 100")
}

func TestRejections(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusClosed}
	b := newBroker(q, "30000")

	_, err := b.PlaceOpen(ctx, marketOpen("USD_JPY", broker.SideBuy, "100"))
	assert.ErrorIs(t, err, ErrMarketClosed)

	q.status = market.StatusOpen
	_, err = b.PlaceOpen(ctx, marketOpen("EUR_JPY", broker.SideBuy, "100"))
	assert.Error(t, err, "no quote")

	_, err = b.PlaceOpen(ctx, marketOpen("USD_JPY", broker.SideBuy, "0"))
	assert.Error(t, err, "zero size")

	limit := marketOpen("USD_JPY", broker.SideBuy, "100")
	limit.Type = broker.OrderLimit
	_, err = b.PlaceOpen(ctx, limit)
	assert.Error(t, err, "limit orders unsupported")

	// 10,000 units need 150.01 * 10000 / 25 = 60,004 JPY of margin.
	_, err = b.PlaceOpen(ctx, marketOpen("USD_JPY", broker.SideBuy, "10000"))
	assert.ErrorContains(t, err, "insufficient margin")

	_, err = b.PlaceClose(ctx, broker.CloseOrder{Symbol: "USD_JPY", PositionID: "nope", Type: broker.OrderMarket, Size: d("100")})
	assert.Error(t, err, "unknown position")

	assert.Error(t, b.Cancel(ctx, "x"))
}

func TestRestoreAndSubscribe(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusOpen}
	b := newBroker(q, "30000")

	b.Restore(d("31000"), []broker.Position{{PositionID: "saved", Symbol: "USD_JPY", Side: broker.SideBuy,
		Size: d("100"), OpenPrice: d("149.000")}})
	assets, _ := b.Assets(ctx)
	assert.Equal(t, "31000", assets.Equity.String())

	events, err := b.SubscribeExecutions(ctx)
	require.NoError(t, err)

	_, err = b.PlaceClose(ctx, broker.CloseOrder{Symbol: "USD_JPY", PositionID: "saved",
		Side: broker.SideSell, Type: broker.OrderMarket, Size: d("100")})
	require.NoError(t, err)

	select {
	case e := <-events:
		assert.Equal(t, "saved", e.PositionID)
		assert.Equal(t, "100", e.RealizedPnL.String(), "(150.000 - 149.000) * 100")
	case <-time.After(time.Second):
		t.Fatal("no execution event")
	}
}

func limitOpen(side broker.Side, price string) broker.OpenOrder {
	p := d(price)
	return broker.OpenOrder{ClientOrderID: "l1", Symbol: "USD_JPY", Side: side, Type: broker.OrderLimit, Size: d("100"), Price: &p}
}

func TestLimitOrderWaitsForItsPrice(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusOpen}
	b := newBroker(q, "30000")

	// A buy resting at the BID is not filled while the ASK is above it.
	ack, err := b.PlaceOpen(ctx, limitOpen(broker.SideBuy, "150.000"))
	require.NoError(t, err)
	assert.Equal(t, "ORDERED", ack.Status)
	execs, err := b.Executions(ctx, ack.OrderID)
	require.NoError(t, err)
	assert.Empty(t, execs)
	positions, _ := b.OpenPositions(ctx, "")
	assert.Empty(t, positions)

	// The ASK comes down to the limit: filled at the limit price, not better.
	q.bid, q.ask = "149.985", "149.995"
	execs, err = b.Executions(ctx, ack.OrderID)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	assert.Equal(t, "150", execs[0].Price.String())
	assert.Equal(t, "0.3", execs[0].Fee.String())
	positions, _ = b.OpenPositions(ctx, "")
	require.Len(t, positions, 1)
	assert.Error(t, b.Cancel(ctx, ack.OrderID), "a filled order cannot be cancelled")

	// Reading again does not fill twice.
	execs, _ = b.Executions(ctx, ack.OrderID)
	assert.Len(t, execs, 1)
}

func TestLimitOrderCancelledAndSellSide(t *testing.T) {
	ctx := context.Background()
	q := &quotes{bid: "150.000", ask: "150.010", status: market.StatusOpen}
	b := newBroker(q, "30000")

	// A sell resting at the ASK waits for the BID to rise to it.
	ack, err := b.PlaceOpen(ctx, limitOpen(broker.SideSell, "150.010"))
	require.NoError(t, err)
	require.NoError(t, b.Cancel(ctx, ack.OrderID))
	q.bid, q.ask = "150.020", "150.030" // would have filled
	execs, _ := b.Executions(ctx, ack.OrderID)
	assert.Empty(t, execs, "cancelled orders never fill")
	assert.Error(t, b.Cancel(ctx, ack.OrderID))

	ack, err = b.PlaceOpen(ctx, limitOpen(broker.SideSell, "150.025"))
	require.NoError(t, err)
	assert.Equal(t, "ORDERED", ack.Status)
	q.bid, q.ask = "150.030", "150.040"
	execs, _ = b.Executions(ctx, ack.OrderID)
	require.Len(t, execs, 1)
	assert.Equal(t, "150.025", execs[0].Price.String())

	// A limit that is already marketable fills at once, at its own price.
	ack, err = b.PlaceOpen(ctx, limitOpen(broker.SideBuy, "150.100"))
	require.NoError(t, err)
	assert.Equal(t, "EXECUTED", ack.Status)

	// The market closing does not lose a working order.
	ack, err = b.PlaceOpen(ctx, limitOpen(broker.SideBuy, "149.000"))
	require.NoError(t, err)
	q.status = market.StatusClosed
	execs, err = b.Executions(ctx, ack.OrderID)
	require.NoError(t, err)
	assert.Empty(t, execs)
	require.NoError(t, b.Cancel(ctx, ack.OrderID))

	_, err = b.PlaceOpen(ctx, broker.OpenOrder{Symbol: "USD_JPY", Side: broker.SideBuy, Type: broker.OrderLimit, Size: d("100")})
	assert.ErrorContains(t, err, "needs a price")
}
