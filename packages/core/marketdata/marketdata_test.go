package marketdata

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory BarStore.
type memStore struct {
	bars map[string]market.Bar
}

func newMemStore() *memStore { return &memStore{bars: map[string]market.Bar{}} }

func barID(b market.Bar) string {
	return b.Key.String() + string(b.Timeframe) + string(b.PriceType) + b.OpenTime.Format(time.RFC3339)
}

func (s *memStore) UpsertBars(_ context.Context, bars []market.Bar) error {
	for _, b := range bars {
		s.bars[barID(b)] = b
	}
	return nil
}

func (s *memStore) times(key market.InstrumentKey, tf market.Timeframe, pt market.PriceType) []time.Time {
	var out []time.Time
	for _, b := range s.bars {
		if b.Key == key && b.Timeframe == tf && b.PriceType == pt {
			out = append(out, b.OpenTime)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (s *memStore) LatestOpenTime(_ context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType) (time.Time, bool, error) {
	ts := s.times(key, tf, pt)
	if len(ts) == 0 {
		return time.Time{}, false, nil
	}
	return ts[len(ts)-1], true, nil
}

func (s *memStore) OpenTimes(_ context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]time.Time, error) {
	var out []time.Time
	for _, t := range s.times(key, tf, pt) {
		if !t.Before(from) && t.Before(to) {
			out = append(out, t)
		}
	}
	return out, nil
}

// fakeSource serves hourly bars for weekdays only and records requests.
type fakeSource struct {
	requests []broker.BarsRequest
	failAt   time.Time
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Status(context.Context) (market.MarketStatus, error) {
	return market.StatusOpen, nil
}
func (f *fakeSource) Instruments(context.Context) ([]market.Instrument, error) { return nil, nil }
func (f *fakeSource) Ticks(context.Context) ([]market.Tick, error)             { return nil, nil }
func (f *fakeSource) SubscribeTicks(context.Context, []string) (<-chan market.Tick, error) {
	return nil, nil
}

func (f *fakeSource) Bars(_ context.Context, req broker.BarsRequest) ([]market.Bar, error) {
	f.requests = append(f.requests, req)
	if !f.failAt.IsZero() && !f.failAt.Before(req.From) && f.failAt.Before(req.To) {
		return nil, errors.New("boom")
	}
	var bars []market.Bar
	for t := req.From; t.Before(req.To); t = t.Add(time.Hour) {
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		bars = append(bars, market.Bar{
			Key: market.InstrumentKey{Broker: "fake", Symbol: req.Symbol}, Timeframe: req.Timeframe,
			PriceType: req.PriceType, OpenTime: t, Close: decimal.NewFromInt(150),
		})
	}
	if len(bars) == 0 {
		return nil, broker.ErrNoData
	}
	return bars, nil
}

func TestBackfill_StoresBothPriceTypesAndResumes(t *testing.T) {
	src := &fakeSource{}
	store := newMemStore()
	bf := &Backfiller{Source: src, Store: store, Logf: func(string, ...any) {}}
	key := market.InstrumentKey{Broker: "fake", Symbol: "USD_JPY"}

	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) // Monday
	to := from.AddDate(0, 0, 14)
	req := BackfillRequest{Symbol: "USD_JPY", Timeframe: market.TF1Hour,
		PriceTypes: []market.PriceType{market.PriceBid, market.PriceAsk}, From: from, To: to}

	results, err := bf.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 10*24, results[0].Stored) // 10 weekdays
	assert.Equal(t, 10*24, results[1].Stored)
	assert.Len(t, store.times(key, market.TF1Hour, market.PriceAsk), 240)

	// Re-running with a later end only fetches the new range.
	src.requests = nil
	req.To = to.AddDate(0, 0, 1) // one more Monday
	results, err = bf.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 24, results[0].Stored)
	require.NotEmpty(t, src.requests)
	assert.Equal(t, time.Date(2026, 9, 18, 23, 0, 0, 0, time.UTC).Add(time.Hour), src.requests[0].From)
}

func TestBackfill_StopsOnSourceErrorAndKeepsProgress(t *testing.T) {
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	src := &fakeSource{failAt: from.AddDate(0, 0, 8)}
	store := newMemStore()
	bf := &Backfiller{Source: src, Store: store, Logf: func(string, ...any) {}}

	_, err := bf.Run(context.Background(), BackfillRequest{Symbol: "USD_JPY", Timeframe: market.TF1Hour,
		PriceTypes: []market.PriceType{market.PriceBid}, From: from, To: from.AddDate(0, 0, 21)})
	require.Error(t, err)
	// The first 7-day chunk was stored before the failure.
	assert.Len(t, store.bars, 5*24)
}

func TestBackfill_RejectsBadInput(t *testing.T) {
	bf := &Backfiller{Source: &fakeSource{}, Store: newMemStore()}
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	_, err := bf.Run(context.Background(), BackfillRequest{Symbol: "USD_JPY", Timeframe: "2h", From: from, To: from.Add(time.Hour)})
	assert.Error(t, err)
	_, err = bf.Run(context.Background(), BackfillRequest{Symbol: "USD_JPY", Timeframe: market.TF1Hour, From: from, To: from})
	assert.Error(t, err)
}

func hourly(from time.Time, n int) []time.Time {
	out := make([]time.Time, n)
	for i := range out {
		out[i] = from.Add(time.Duration(i) * time.Hour)
	}
	return out
}

func TestFindGaps(t *testing.T) {
	// Summer week: Mon 06:00 JST (Sun 21:00 UTC) to Sat 06:00 JST (Fri 21:00 UTC).
	week1 := hourly(time.Date(2026, 9, 6, 21, 0, 0, 0, time.UTC), 5*24)
	week2 := hourly(time.Date(2026, 9, 13, 21, 0, 0, 0, time.UTC), 5*24)

	t.Run("weekend closure is not a gap", func(t *testing.T) {
		times := append(append([]time.Time{}, week1...), week2...)
		assert.Empty(t, FindGaps(times, time.Hour))
	})

	t.Run("missing hours on a weekday are reported", func(t *testing.T) {
		times := append(append([]time.Time{}, week1[:30]...), week1[33:]...)
		gaps := FindGaps(times, time.Hour)
		require.Len(t, gaps, 1)
		assert.Equal(t, week1[29], gaps[0].After)
		assert.Equal(t, week1[33], gaps[0].Before)
		assert.Equal(t, 3, gaps[0].Missing)
	})

	t.Run("outage longer than a weekend is reported", func(t *testing.T) {
		times := append(append([]time.Time{}, week1[:24]...), week2...)
		assert.Len(t, FindGaps(times, time.Hour), 1)
	})
}
