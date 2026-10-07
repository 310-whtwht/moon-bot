package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeInstruments struct {
	calls int
	err   error
}

func (f *fakeInstruments) Instruments(context.Context) ([]market.Instrument, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	in := func(symbol, quote string, min int64) market.Instrument {
		return market.Instrument{
			Key: market.InstrumentKey{Broker: "gmo", Symbol: symbol}, QuoteCurrency: quote,
			MinOrderSize: decimal.NewFromInt(min), SizeStep: decimal.NewFromInt(min),
		}
	}
	return []market.Instrument{in("USD_JPY", "JPY", 100), in("EUR_USD", "USD", 100), in("TRY_JPY", "JPY", 10000)}, nil
}

func TestInstrumentCache_OnlyYenPairsAndCachedForAnHour(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	src := &fakeInstruments{}
	cache := &instrumentCache{src: src, now: func() time.Time { return now }}

	list, err := cache.list(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []tradable{{Symbol: "USD_JPY", MinUnits: 100, Step: 100}, {Symbol: "TRY_JPY", MinUnits: 10000, Step: 10000}}, list,
		"P&L and limits are counted in yen, so only JPY-quoted pairs")

	now = now.Add(30 * time.Minute)
	_, _ = cache.list(context.Background())
	assert.Equal(t, 1, src.calls)

	// After an hour it reloads; if the broker is down the old list is kept.
	now = now.Add(time.Hour)
	src.err = errors.New("down")
	list, err = cache.list(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, 2, src.calls)
}

func TestTradable_CheckUnits(t *testing.T) {
	usd := tradable{Symbol: "USD_JPY", MinUnits: 100, Step: 100}
	assert.NoError(t, usd.checkUnits(100))
	assert.NoError(t, usd.checkUnits(1500))
	assert.Error(t, usd.checkUnits(50), "below the minimum")
	assert.Error(t, usd.checkUnits(150), "not a multiple of the step")
	assert.Error(t, tradable{Symbol: "TRY_JPY", MinUnits: 10000, Step: 10000}.checkUnits(100))
}

func TestEntryRequest_Settings(t *testing.T) {
	// Nothing given: a market order, no spread limit.
	got, err := entryRequest{}.settings()
	require.NoError(t, err)
	assert.Equal(t, "market", got.Order)
	assert.Equal(t, "skip", got.Fallback)
	assert.Equal(t, 30, got.WaitSeconds)
	assert.Nil(t, got.MaxSpread)

	wait, spread, zero := 60, 0.02, 0.0
	got, err = entryRequest{Order: "limit", WaitSeconds: &wait, Fallback: "market", MaxSpread: &spread}.settings()
	require.NoError(t, err)
	assert.Equal(t, "limit", got.Order)
	assert.Equal(t, "market", got.Fallback)
	assert.Equal(t, 60, got.WaitSeconds)
	assert.Equal(t, 0.02, *got.MaxSpread)

	got, err = entryRequest{Order: "market", MaxSpread: &zero}.settings()
	require.NoError(t, err)
	assert.Nil(t, got.MaxSpread, "0 removes the limit")

	tooShort, tooLong, negative := 1, 3600, -0.01
	for _, bad := range []entryRequest{
		{Order: "stop"},
		{Order: "limit", Fallback: "retry"},
		{Order: "limit", WaitSeconds: &tooShort},
		{Order: "limit", WaitSeconds: &tooLong},
		{Order: "market", MaxSpread: &negative},
	} {
		_, err := bad.settings()
		assert.Error(t, err, "%+v", bad)
	}
}
