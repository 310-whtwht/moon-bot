package imports

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memStore struct {
	queue    []Import
	progress map[string][]string
	stored   map[string]int
	outcome  map[string]string
	reset    int
}

func newMemStore(queue ...Import) *memStore {
	return &memStore{queue: queue, progress: map[string][]string{}, stored: map[string]int{}, outcome: map[string]string{}}
}

func (m *memStore) ResetRunning(context.Context) error { m.reset++; return nil }
func (m *memStore) Claim(context.Context) (*Import, error) {
	if len(m.queue) == 0 {
		return nil, nil
	}
	imp := m.queue[0]
	m.queue = m.queue[1:]
	return &imp, nil
}
func (m *memStore) Progress(_ context.Context, id string, stored int, progress string) error {
	m.progress[id] = append(m.progress[id], progress)
	m.stored[id] = stored
	return nil
}
func (m *memStore) Finish(_ context.Context, id string, stored int, errText string) error {
	m.stored[id] = stored
	m.outcome[id] = "completed"
	if errText != "" {
		m.outcome[id] = "failed: " + errText
	}
	return nil
}

// source serves one bar per hour and fails for symbols listed in broken.
type source struct {
	broken map[string]bool
	calls  []string
}

func (s *source) Name() string                                             { return "gmo" }
func (s *source) Status(context.Context) (market.MarketStatus, error)      { return market.StatusOpen, nil }
func (s *source) Instruments(context.Context) ([]market.Instrument, error) { return nil, nil }
func (s *source) Ticks(context.Context) ([]market.Tick, error)             { return nil, nil }
func (s *source) SubscribeTicks(context.Context, []string) (<-chan market.Tick, error) {
	return nil, nil
}
func (s *source) Bars(_ context.Context, req broker.BarsRequest) ([]market.Bar, error) {
	s.calls = append(s.calls, fmt.Sprintf("%s %s", req.Symbol, req.PriceType))
	if s.broken[req.Symbol] {
		return nil, errors.New("broker is down")
	}
	var out []market.Bar
	one := decimal.NewFromInt(1)
	for t := req.From; t.Before(req.To); t = t.Add(time.Hour) {
		out = append(out, market.Bar{Key: market.InstrumentKey{Broker: "gmo", Symbol: req.Symbol},
			Timeframe: req.Timeframe, PriceType: req.PriceType, OpenTime: t, Open: one, High: one, Low: one, Close: one})
	}
	return out, nil
}

// bars is an in-memory bar store.
type bars struct{ rows map[string]time.Time }

func (b *bars) UpsertBars(_ context.Context, in []market.Bar) error {
	for _, bar := range in {
		k := fmt.Sprintf("%s|%s|%s", bar.Key.Symbol, bar.Timeframe, bar.PriceType)
		if bar.OpenTime.After(b.rows[k]) {
			b.rows[k] = bar.OpenTime
		}
	}
	return nil
}
func (b *bars) LatestOpenTime(_ context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType) (time.Time, bool, error) {
	t, ok := b.rows[fmt.Sprintf("%s|%s|%s", key.Symbol, tf, pt)]
	return t, ok, nil
}
func (b *bars) Bars(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType, time.Time, time.Time) ([]market.Bar, error) {
	return nil, nil
}
func (b *bars) OpenTimes(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}

func TestDrain_DownloadsBothPriceTypesAndRecordsTheOutcome(t *testing.T) {
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	from := now.Add(-10 * 24 * time.Hour) // two weekly chunks per price type
	store := newMemStore(
		Import{ID: "a", Symbol: "GBP_JPY", Timeframe: market.TF1Hour, From: from},
		Import{ID: "b", Symbol: "XXX_JPY", Timeframe: market.TF1Hour, From: from},
		Import{ID: "c", Symbol: "AUD_JPY", Timeframe: market.TF1Hour, From: from},
	)
	src := &source{broken: map[string]bool{"XXX_JPY": true}}
	r := &Runner{Store: store, Source: src, Bars: &bars{rows: map[string]time.Time{}},
		Now: func() time.Time { return now }, Logf: t.Logf}

	r.Drain(context.Background())

	assert.Equal(t, "completed", store.outcome["a"])
	assert.Equal(t, 480, store.stored["a"], "240 hourly bars each for BID and ASK")
	assert.Equal(t, []string{"BID 2026-10-05", "BID 2026-10-08", "ASK 2026-10-05", "ASK 2026-10-08"}, store.progress["a"])

	assert.Contains(t, store.outcome["b"], "failed: ")
	assert.Contains(t, store.outcome["b"], "broker is down")

	assert.Equal(t, "completed", store.outcome["c"], "one failure does not stop the queue")
	assert.Empty(t, store.queue)
}

func TestRun_PutsInterruptedDownloadsBackFirst(t *testing.T) {
	store := newMemStore()
	r := &Runner{Store: store, Source: &source{}, Bars: &bars{rows: map[string]time.Time{}},
		Interval: time.Millisecond, Logf: t.Logf}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.Run(ctx), context.DeadlineExceeded)
	assert.Equal(t, 1, store.reset)
}
