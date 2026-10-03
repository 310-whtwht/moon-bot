// Package marketdata stores bars and keeps them in sync with a broker.
package marketdata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/moomoo-trading/core/market"
)

// BarStore persists bars. Implementations must treat (key, timeframe,
// price type, open time) as unique and overwrite on conflict.
type BarStore interface {
	UpsertBars(ctx context.Context, bars []market.Bar) error
	// LatestOpenTime returns the newest stored open time, or ok=false if none.
	LatestOpenTime(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType) (t time.Time, ok bool, err error)
	// Bars returns stored bars with open time in [from, to), ascending.
	Bars(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]market.Bar, error)
	// OpenTimes returns stored open times in [from, to), ascending.
	OpenTimes(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]time.Time, error)
}

// MySQLBarStore stores bars in the bars table (migration 007).
// The connection must use loc=UTC so DATETIME values stay in UTC.
type MySQLBarStore struct {
	db *sql.DB
}

func NewMySQLBarStore(db *sql.DB) *MySQLBarStore {
	return &MySQLBarStore{db: db}
}

const upsertBatchSize = 500

func (s *MySQLBarStore) UpsertBars(ctx context.Context, bars []market.Bar) error {
	for start := 0; start < len(bars); start += upsertBatchSize {
		end := start + upsertBatchSize
		if end > len(bars) {
			end = len(bars)
		}
		if err := s.upsertBatch(ctx, bars[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLBarStore) upsertBatch(ctx context.Context, bars []market.Bar) error {
	if len(bars) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(bars))
	args := make([]any, 0, len(bars)*9)
	for _, b := range bars {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			b.Key.Broker, b.Key.Symbol, string(b.Timeframe), string(b.PriceType), b.OpenTime.UTC(),
			b.Open.String(), b.High.String(), b.Low.String(), b.Close.String(),
		)
	}

	query := `INSERT INTO bars (broker, symbol, timeframe, price_type, open_time, open, high, low, close)
VALUES ` + strings.Join(placeholders, ", ") + `
ON DUPLICATE KEY UPDATE open = VALUES(open), high = VALUES(high), low = VALUES(low), close = VALUES(close)`

	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert bars: %w", err)
	}
	return nil
}

func (s *MySQLBarStore) LatestOpenTime(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType) (time.Time, bool, error) {
	var latest sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(open_time) FROM bars WHERE broker = ? AND symbol = ? AND timeframe = ? AND price_type = ?`,
		key.Broker, key.Symbol, string(tf), string(pt),
	).Scan(&latest)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("latest bar: %w", err)
	}
	if !latest.Valid {
		return time.Time{}, false, nil
	}
	return latest.Time.UTC(), true, nil
}

func (s *MySQLBarStore) Bars(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]market.Bar, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT open_time, open, high, low, close FROM bars
WHERE broker = ? AND symbol = ? AND timeframe = ? AND price_type = ? AND open_time >= ? AND open_time < ?
ORDER BY open_time`,
		key.Broker, key.Symbol, string(tf), string(pt), from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("list bars: %w", err)
	}
	defer rows.Close()

	var bars []market.Bar
	for rows.Next() {
		b := market.Bar{Key: key, Timeframe: tf, PriceType: pt}
		if err := rows.Scan(&b.OpenTime, &b.Open, &b.High, &b.Low, &b.Close); err != nil {
			return nil, err
		}
		b.OpenTime = b.OpenTime.UTC()
		bars = append(bars, b)
	}
	return bars, rows.Err()
}

func (s *MySQLBarStore) OpenTimes(ctx context.Context, key market.InstrumentKey, tf market.Timeframe, pt market.PriceType, from, to time.Time) ([]time.Time, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT open_time FROM bars
WHERE broker = ? AND symbol = ? AND timeframe = ? AND price_type = ? AND open_time >= ? AND open_time < ?
ORDER BY open_time`,
		key.Broker, key.Symbol, string(tf), string(pt), from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("list bar times: %w", err)
	}
	defer rows.Close()

	var times []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		times = append(times, t.UTC())
	}
	return times, rows.Err()
}
