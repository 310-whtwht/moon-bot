package driftcmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

func TestComparePairsTradesOnTheSameBarAndSide(t *testing.T) {
	live := []Trade{
		// Filled 5 s after the 01:00 bar opened, 0.002 worse than the backtest.
		{Long: true, Units: 100, OpenPrice: 150.012, ExpectedOpen: 150.010, ClosePrice: 150.495, ExpectedClose: 150.500,
			PnL: 48, OpenedAt: t0.Add(time.Hour + 5*time.Second), ClosedAt: t0.Add(3 * time.Hour)},
		// The backtest never took this one.
		{Long: false, Units: 100, OpenPrice: 151, ClosePrice: 151.2, PnL: -20,
			OpenedAt: t0.Add(10 * time.Hour), ClosedAt: t0.Add(11 * time.Hour)},
	}
	bt := []backtest.Trade{
		// Same bar but the other side: not a match.
		{Side: strategy.Short, EntryTime: t0.Add(time.Hour), EntryPrice: 150, ExitPrice: 149, PnL: 50},
		{Side: strategy.Long, EntryTime: t0.Add(time.Hour), EntryPrice: 150.010, ExitTime: t0.Add(3 * time.Hour),
			ExitPrice: 150.500, PnL: 24.5, ExitReason: "signal"},
	}
	r := Compare(live, bt, time.Hour, 50)

	require.Len(t, r.Pairs, 1)
	p := r.Pairs[0]
	assert.InDelta(t, 0.002, p.EntryCost, 1e-9, "bought 0.002 higher")
	assert.InDelta(t, 0.005, p.ExitCost, 1e-9, "sold 0.005 lower")
	assert.InDelta(t, 49, p.BTPnL, 1e-9, "backtest P&L scaled from 50 to 100 units")
	require.Len(t, r.LiveOnly, 1)
	assert.Equal(t, 151.0, r.LiveOnly[0].OpenPrice)
	require.Len(t, r.BTOnly, 1)
	assert.Equal(t, strategy.Short, r.BTOnly[0].Side)
	assert.InDelta(t, 1.0/3, r.MatchRate(), 1e-9)
	assert.InDeltaSlice(t, []float64{0.002}, r.EntrySlip, 1e-9)
	assert.InDeltaSlice(t, []float64{0.005}, r.ExitSlip, 1e-9)

	var out bytes.Buffer
	r.Print(&out, true)
	assert.Contains(t, out.String(), "matched 1, bot only 1, backtest only 1 (match rate 33%)")
	assert.Contains(t, out.String(), "(backtest did not trade)")
}

func TestCompareDoesNotPairAcrossBars(t *testing.T) {
	live := []Trade{{Long: true, Units: 100, OpenedAt: t0.Add(2*time.Hour + 5*time.Second)}}
	bt := []backtest.Trade{{Side: strategy.Long, EntryTime: t0}}
	r := Compare(live, bt, time.Hour, 100)
	assert.Empty(t, r.Pairs)
	assert.Len(t, r.LiveOnly, 1)
	assert.Len(t, r.BTOnly, 1)
}

func TestCompareShortSlippageSigns(t *testing.T) {
	// A short sold 0.003 below the backtest and bought back 0.004 above it.
	live := []Trade{{Long: false, Units: 100, OpenPrice: 149.997, ClosePrice: 149.504, OpenedAt: t0.Add(5 * time.Second)}}
	bt := []backtest.Trade{{Side: strategy.Short, EntryTime: t0, EntryPrice: 150, ExitPrice: 149.5}}
	r := Compare(live, bt, time.Hour, 100)
	require.Len(t, r.Pairs, 1)
	assert.InDelta(t, 0.003, r.Pairs[0].EntryCost, 1e-9)
	assert.InDelta(t, 0.004, r.Pairs[0].ExitCost, 1e-9)
}
