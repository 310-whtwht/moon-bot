package backtestcmd

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	opts, err := parseFlags([]string{
		"-param", "fast_period=5", "-param", "stop_atr_mult=1.5",
		"-sweep", "slow_period=20,40", "-from", "2024-01-01", "-to", "2025-01-01", "-units", "1000",
	}, time.Now(), io.Discard)
	require.NoError(t, err)

	assert.Equal(t, strategy.Params{"fast_period": 5, "stop_atr_mult": 1.5}, opts.params)
	assert.Equal(t, []float64{20, 40}, opts.sweeps["slow_period"])
	assert.Equal(t, 1000.0, opts.units)
	assert.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), opts.to)

	for _, bad := range [][]string{
		{"-param", "fast_period"},
		{"-param", "fast_period=x"},
		{"-sweep", "slow_period"},
		{"-sweep", "slow_period=1,x"},
		{"-interval", "2h"},
		{"-from", "2025-01-01", "-to", "2024-01-01"},
	} {
		_, err := parseFlags(bad, time.Now(), io.Discard)
		assert.Error(t, err, bad)
	}
}

func TestGrid(t *testing.T) {
	combos := grid(strategy.Params{"atr_period": 14}, map[string][]float64{
		"fast_period": {5, 10},
		"slow_period": {20, 40, 60},
	})
	require.Len(t, combos, 6)
	for _, c := range combos {
		assert.Equal(t, 14.0, c["atr_period"])
	}
	assert.Equal(t, strategy.Params{"atr_period": 14, "fast_period": 5, "slow_period": 20}, combos[0])
	assert.Equal(t, strategy.Params{"atr_period": 14, "fast_period": 10, "slow_period": 60}, combos[5])

	assert.Len(t, grid(strategy.Params{}, nil), 1, "no sweeps = one run")
}

func TestWriteCandlesCSV(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeCandlesCSV(&buf, []backtest.Candle{{
		Time: time.Date(2026, 10, 5, 0, 55, 0, 0, time.UTC),
		Bid:  backtest.OHLC{Open: 150.1, High: 150.2, Low: 150, Close: 150.15},
		Ask:  backtest.OHLC{Open: 150.102, High: 150.202, Low: 150.002, Close: 150.152},
	}}))
	assert.Equal(t, "time,bid_open,bid_high,bid_low,bid_close,ask_open,ask_high,ask_low,ask_close,quote_rate\n"+
		"2026-10-05T00:55:00Z,150.1,150.2,150,150.15,150.102,150.202,150.002,150.152,0\n", buf.String())
}
