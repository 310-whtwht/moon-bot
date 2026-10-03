package backfill

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLBarStore runs against a migrated database when MOONBOT_TEST_DSN is
// set (use loc=UTC), e.g.
// moomoo:pw@tcp(127.0.0.1:3308)/moomoo_trading?parseTime=true&loc=UTC
func TestMySQLBarStore(t *testing.T) {
	dsn := os.Getenv("MOONBOT_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOT_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	key := market.InstrumentKey{Broker: "test", Symbol: "USD_JPY"}
	_, err = db.ExecContext(ctx, "DELETE FROM bars WHERE broker = ?", key.Broker)
	require.NoError(t, err)

	store := marketdata.NewMySQLBarStore(db)
	t0 := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
	bar := func(at time.Time, close string) market.Bar {
		c := decimal.RequireFromString(close)
		return market.Bar{Key: key, Timeframe: market.TF1Hour, PriceType: market.PriceBid,
			OpenTime: at, Open: c, High: c, Low: c, Close: c}
	}

	_, ok, err := store.LatestOpenTime(ctx, key, market.TF1Hour, market.PriceBid)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, store.UpsertBars(ctx, []market.Bar{bar(t0, "157.123"), bar(t0.Add(time.Hour), "157.456")}))
	// Re-upserting the same open time overwrites instead of duplicating.
	require.NoError(t, store.UpsertBars(ctx, []market.Bar{bar(t0.Add(time.Hour), "157.789")}))

	latest, ok, err := store.LatestOpenTime(ctx, key, market.TF1Hour, market.PriceBid)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, t0.Add(time.Hour), latest)

	times, err := store.OpenTimes(ctx, key, market.TF1Hour, market.PriceBid, t0, t0.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, []time.Time{t0, t0.Add(time.Hour)}, times)

	var close string
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT close FROM bars WHERE broker = ? AND open_time = ?", key.Broker, t0.Add(time.Hour)).Scan(&close))
	assert.Equal(t, "157.78900000", close)

	_, err = db.ExecContext(ctx, "DELETE FROM bars WHERE broker = ?", key.Broker)
	require.NoError(t, err)
}
