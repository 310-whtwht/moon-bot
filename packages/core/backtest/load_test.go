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

// sideStore returns fixed bars per price type, and per symbol for the
// conversion pairs listed in rates.
type sideStore struct {
	bars  map[market.PriceType][]market.Bar
	rates map[string][]market.Bar // BID bars by symbol
}

func (s sideStore) UpsertBars(context.Context, []market.Bar) error { return nil }
func (s sideStore) LatestOpenTime(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (s sideStore) OpenTimes(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (s sideStore) Bars(_ context.Context, key market.InstrumentKey, _ market.Timeframe, pt market.PriceType, _, _ time.Time) ([]market.Bar, error) {
	// With rates set, the store holds EUR_USD in bars and yen pairs in rates.
	if s.rates != nil && key.Symbol != "EUR_USD" {
		return s.rates[key.Symbol], nil
	}
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

func TestConversionSymbol(t *testing.T) {
	assert.Equal(t, "", ConversionSymbol("USD_JPY"))
	assert.Equal(t, "USD_JPY", ConversionSymbol("EUR_USD"))
	assert.Equal(t, "GBP_JPY", ConversionSymbol("EUR_GBP"))
	assert.Equal(t, "SEK_JPY", ConversionSymbol("NOK_SEK"))
	assert.Equal(t, "USD", QuoteCurrency("EUR_USD"))
}

func TestLoadCandles_AddsTheYenRateForOtherQuoteCurrencies(t *testing.T) {
	key := market.InstrumentKey{Broker: "gmo", Symbol: "EUR_USD"}
	hours := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Hour) }
	store := sideStore{
		bars: map[market.PriceType][]market.Bar{
			market.PriceBid: {bar(hours(0), "1.1000"), bar(hours(1), "1.1010"), bar(hours(2), "1.1020"), bar(hours(3), "1.1030")},
			market.PriceAsk: {bar(hours(0), "1.1001"), bar(hours(1), "1.1011"), bar(hours(2), "1.1021"), bar(hours(3), "1.1031")},
		},
		// The dollar's bars start an hour late and miss 02:00.
		rates: map[string][]market.Bar{"USD_JPY": {bar(hours(1), "150.5"), bar(hours(3), "151.5")}},
	}

	candles, err := LoadCandles(context.Background(), store, key, market.TF1Hour, t0, hours(4))
	require.NoError(t, err)
	require.Len(t, candles, 4)
	assert.Equal(t, 150.5, candles[0].QuoteRate, "before the first rate bar: the first known rate")
	assert.Equal(t, 150.5, candles[1].QuoteRate)
	assert.Equal(t, 150.5, candles[2].QuoteRate, "a missing bar keeps the last rate")
	assert.Equal(t, 151.5, candles[3].QuoteRate)

	// Without the conversion pair's bars the backtest cannot be in yen.
	store.rates = map[string][]market.Bar{}
	_, err = LoadCandles(context.Background(), store, key, market.TF1Hour, t0, hours(4))
	assert.ErrorContains(t, err, "USD_JPY 1h bars are needed")
}

func TestLoadCandles_Errors(t *testing.T) {
	_, err := LoadCandles(context.Background(), sideStore{}, market.InstrumentKey{Broker: "gmo", Symbol: "USD_JPY"}, market.TF1Hour, t0, t0.Add(time.Hour))
	assert.Error(t, err, "no data")
}
