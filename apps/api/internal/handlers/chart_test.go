package handlers

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

// fakeBars serves hourly bars from `start` up to (and including) the hour of `now`.
type fakeBars struct {
	start, now time.Time
	closes     map[time.Time]int64 // overrides the close of one bar
	requests   []broker.BarsRequest
}

func (f *fakeBars) Bars(_ context.Context, req broker.BarsRequest) ([]market.Bar, error) {
	f.requests = append(f.requests, req)
	var bars []market.Bar
	for t := f.start; !t.After(f.now); t = t.Add(time.Hour) {
		if t.Before(req.From) || !t.Before(req.To) {
			continue
		}
		c := decimal.NewFromInt(150)
		if v, ok := f.closes[t]; ok {
			c = decimal.NewFromInt(v)
		}
		bars = append(bars, market.Bar{OpenTime: t, Open: c, High: c, Low: c, Close: c})
	}
	if len(bars) == 0 {
		return nil, broker.ErrNoData
	}
	return bars, nil
}

func TestBarCache_RefetchesOnlyFromTheLastBar(t *testing.T) {
	now := time.Date(2026, 10, 5, 3, 20, 0, 0, time.UTC)
	hour := now.Truncate(time.Hour)
	src := &fakeBars{start: now.Add(-2000 * time.Hour).Truncate(time.Hour), now: now, closes: map[time.Time]int64{}}
	cache := newBarCache(src)
	ctx := context.Background()

	bars, err := cache.recent(ctx, "USD_JPY", market.TF1Hour, 10, now)
	require.NoError(t, err)
	require.Len(t, bars, 10)
	assert.Equal(t, hour, bars[9].OpenTime, "the forming bar is included")
	assert.Equal(t, now.Add(-lookback(time.Hour, 10)), src.requests[0].From)

	// The forming bar moves, then a new bar opens.
	src.closes[hour] = 151
	src.now = now.Add(time.Hour)
	bars, err = cache.recent(ctx, "USD_JPY", market.TF1Hour, 10, src.now)
	require.NoError(t, err)
	require.Len(t, bars, 10)
	assert.Equal(t, hour, src.requests[1].From, "only the last cached bar onwards is fetched")
	assert.Equal(t, "151", bars[8].Close.String(), "the bar that was forming is replaced")
	assert.Equal(t, hour.Add(time.Hour), bars[9].OpenTime)
	for i := 1; i < len(bars); i++ {
		assert.Equal(t, time.Hour, bars[i].OpenTime.Sub(bars[i-1].OpenTime), "no duplicates or holes")
	}
}

func TestBarCache_NoNewDataKeepsTheCache(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	src := &fakeBars{start: now.Add(-100 * time.Hour), now: now.Add(-10 * time.Hour)}
	cache := newBarCache(src)

	first, err := cache.recent(context.Background(), "USD_JPY", market.TF1Hour, 5, now)
	require.NoError(t, err)
	src.start = now.Add(time.Hour) // the market is closed: nothing in range
	again, err := cache.recent(context.Background(), "USD_JPY", market.TF1Hour, 5, now.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, first, again)
}

func TestBarCache_ReloadsWhenMoreHistoryIsAsked(t *testing.T) {
	now := time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	src := &fakeBars{start: now.Add(-2000 * time.Hour), now: now}
	cache := newBarCache(src)

	_, err := cache.recent(context.Background(), "USD_JPY", market.TF1Hour, 10, now)
	require.NoError(t, err)
	bars, err := cache.recent(context.Background(), "USD_JPY", market.TF1Hour, 300, now)
	require.NoError(t, err)
	assert.Len(t, bars, 300)
}
