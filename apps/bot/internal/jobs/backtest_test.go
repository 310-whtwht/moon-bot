package jobs

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	claim     Claim
	claimable bool
	completed []byte
	failed    string
}

func (f *fakeStore) Claim(context.Context, string) (Claim, bool, error) {
	return f.claim, f.claimable, nil
}
func (f *fakeStore) Complete(_ context.Context, _ string, r []byte) error {
	f.completed = r
	return nil
}
func (f *fakeStore) Fail(_ context.Context, _ string, msg string) error {
	f.failed = msg
	return nil
}

// sineBars serves a synthetic BID/ASK hourly series.
type sineBars struct{ n int }

func (s sineBars) UpsertBars(context.Context, []market.Bar) error { return nil }
func (s sineBars) LatestOpenTime(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (s sineBars) OpenTimes(context.Context, market.InstrumentKey, market.Timeframe, market.PriceType, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (s sineBars) Bars(_ context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]market.Bar, error) {
	var out []market.Bar
	for i := 0; i < s.n; i++ {
		at := from.Add(time.Duration(i) * time.Hour)
		if !at.Before(to) {
			break
		}
		p := 150 + 3*math.Sin(float64(i)/15)
		if pt == market.PriceAsk {
			p += 0.01
		}
		d := decimal.NewFromFloat(p)
		out = append(out, market.Bar{Key: key, Timeframe: tf, PriceType: pt, OpenTime: at, Open: d, High: d, Low: d, Close: d})
	}
	return out, nil
}

func claim() Claim {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	return Claim{
		Spec: backtest.JobSpec{
			StrategyType: "ema_cross", StrategyParams: strategy.Params{"fast_period": 5, "slow_period": 20},
			Broker: "gmo", Symbol: "USD_JPY", Timeframe: market.TF1Hour, Units: 100, InitialBalance: 30000,
		},
		Start: start,
		End:   start.AddDate(0, 0, 40),
	}
}

func TestHandle_CompletesWithResults(t *testing.T) {
	store := &fakeStore{claim: claim(), claimable: true}
	r := &BacktestRunner{Store: store, Bars: sineBars{n: 2000}, Logf: func(string, ...any) {}}

	require.NoError(t, r.Handle(context.Background(), "bt-1"))
	require.NotNil(t, store.completed)
	assert.Empty(t, store.failed)

	var res backtest.Result
	require.NoError(t, json.Unmarshal(store.completed, &res))
	assert.Equal(t, "ema_cross", res.Strategy)
	assert.Equal(t, 20.0, res.Params["slow_period"])
	assert.Equal(t, 14.0, res.Params["atr_period"], "defaults resolved")
	assert.NotEmpty(t, res.Trades)
	assert.LessOrEqual(t, len(res.Equity), maxEquityPoints)
	assert.Equal(t, 41*24, res.Bars, "end date is inclusive")
}

func TestHandle_SkipsWhenNotPending(t *testing.T) {
	store := &fakeStore{claimable: false}
	r := &BacktestRunner{Store: store, Bars: sineBars{n: 10}, Logf: func(string, ...any) {}}

	require.NoError(t, r.Handle(context.Background(), "bt-1"))
	assert.Nil(t, store.completed)
	assert.Empty(t, store.failed)
}

func TestHandle_RecordsFailure(t *testing.T) {
	c := claim()
	c.Spec.StrategyType = "no_such_strategy"
	store := &fakeStore{claim: c, claimable: true}
	r := &BacktestRunner{Store: store, Bars: sineBars{n: 10}, Logf: func(string, ...any) {}}

	require.NoError(t, r.Handle(context.Background(), "bt-1"))
	assert.Contains(t, store.failed, "no_such_strategy")

	store = &fakeStore{claim: claim(), claimable: true}
	r = &BacktestRunner{Store: store, Bars: sineBars{n: 0}, Logf: func(string, ...any) {}}
	require.NoError(t, r.Handle(context.Background(), "bt-2"))
	assert.Contains(t, store.failed, "backfill", "no data tells the user to backfill")
}
