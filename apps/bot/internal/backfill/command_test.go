package backfill

import (
	"io"
	"testing"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlags_Defaults(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	opts, err := parseFlags(nil, now, io.Discard)
	require.NoError(t, err)

	assert.Equal(t, "USD_JPY", opts.symbol)
	assert.Equal(t, market.TF1Hour, opts.timeframe)
	assert.Equal(t, []market.PriceType{market.PriceBid, market.PriceAsk}, opts.priceTypes)
	assert.Equal(t, time.Date(2023, 10, 28, 0, 0, 0, 0, time.UTC), opts.from)
	assert.Equal(t, now, opts.to)
	assert.Equal(t, time.Second, opts.minInterval)
}

func TestParseFlags_Custom(t *testing.T) {
	opts, err := parseFlags([]string{
		"-symbol", "EUR_JPY", "-interval", "4h", "-price-types", "ask",
		"-from", "2025-01-01", "-to", "2025-02-01", "-min-interval", "500ms",
	}, time.Now(), io.Discard)
	require.NoError(t, err)

	assert.Equal(t, "EUR_JPY", opts.symbol)
	assert.Equal(t, market.TF4Hour, opts.timeframe)
	assert.Equal(t, []market.PriceType{market.PriceAsk}, opts.priceTypes)
	assert.Equal(t, time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), opts.to)
	assert.Equal(t, 500*time.Millisecond, opts.minInterval)
}

func TestParseFlags_Errors(t *testing.T) {
	cases := [][]string{
		{"-interval", "2h"},
		{"-price-types", "MID"},
		{"-from", "2025/01/01"},
		{"-from", "2025-02-01", "-to", "2025-01-01"},
	}
	for _, args := range cases {
		_, err := parseFlags(args, time.Now(), io.Discard)
		assert.Error(t, err, args)
	}
}
