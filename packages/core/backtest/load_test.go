package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sideStore returns fixed bars per price type.
type sideStore struct {
	bars map[market.PriceType][]market.Bar
}

func (s sideStore) UpsertBars(context.Context, []market.Bar) error { return nil }
func (s sideStore) LatestOpenTime(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (s sideStore) OpenTimes(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (s sideStore) Bars(_ context.Context, _ market.InstrumentKey, _ market.Timeframe, pt market.PriceType, _, _ time.Time) ([]market.Bar, error) {
	return s.bars[pt], nil
}

func bar(at time.Time, price string) market.Bar {
	p := decimal.RequireFromString(price)
	return market.Bar{OpenTime: at, Open: p, High: p, Low: p, Close: p}
}

func TestLoadCandles_PairsBidAndAsk(t *testing.T) {
	key := market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"}
	store := sideStore{bars: map[market.PriceType][]market.Bar{
		market.PriceBid: {bar(t0, "150.000"), bar(t0.Add(time.Hour), "150.100"), bar(t0.Add(2*time.Hour), "150.200")},
		market.PriceAsk: {bar(t0, "150.010"), bar(t0.Add(2*time.Hour), "150.210")}, // 01:00 missing
	}}

	candles, err := LoadCandles(context.Background(), store, key, market.TF1Hour, t0, t0.Add(3*time.Hour))
	require.NoError(t, err)
	require.Len(t, candles, 2)
	assert.Equal(t, t0.Add(2*time.Hour), candles[1].Time)
	assert.InDelta(t, 150.2, candles[1].Bid.Close, 1e-9)
	assert.InDelta(t, 150.21, candles[1].Ask.Close, 1e-9)
}

func TestLoadCandles_Errors(t *testing.T) {
	_, err := LoadCandles(context.Background(), sideStore{}, market.InstrumentKey{Broker: "gmo", Symbol: "EUR_USD"}, market.TF1Hour, t0, t0.Add(time.Hour))
	assert.Error(t, err, "non-JPY quote")

	_, err = LoadCandles(context.Background(), sideStore{}, market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"}, market.TF1Hour, t0, t0.Add(time.Hour))
	assert.Error(t, err, "no data")
}
