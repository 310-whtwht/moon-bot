// Package series stores outside data (not prices) that strategies read by
// name, such as the CFTC positioning of speculators, and serves it to scripts
// as strategy.Data.
package series

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Point is one value of a series. Time is when the value became public: a
// strategy deciding at t sees the latest point with Time <= t.
type Point struct {
	Time  time.Time
	Value float64
}

// MySQLStore keeps series in the data_series table (migration 019).
// The connection must use loc=UTC so DATETIME values stay in UTC.
type MySQLStore struct {
	DB *sql.DB
}

const insertBatchSize = 500

// Replace stores points as the whole of the named series.
func (s MySQLStore) Replace(ctx context.Context, name string, points []Point) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM data_series WHERE name = ?", name); err != nil {
		return fmt.Errorf("replace %s: %w", name, err)
	}
	for start := 0; start < len(points); start += insertBatchSize {
		batch := points[start:min(start+insertBatchSize, len(points))]
		args := make([]any, 0, len(batch)*3)
		for _, p := range batch {
			args = append(args, name, p.Time.UTC(), p.Value)
		}
		q := "INSERT INTO data_series (name, available_at, value) VALUES " +
			strings.TrimSuffix(strings.Repeat("(?, ?, ?),", len(batch)), ",") +
			" ON DUPLICATE KEY UPDATE value = VALUES(value)"
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("replace %s: %w", name, err)
		}
	}
	return tx.Commit()
}

// Load returns the named series, oldest first (empty if it is not stored).
func (s MySQLStore) Load(ctx context.Context, name string) ([]Point, error) {
	rows, err := s.DB.QueryContext(ctx,
		"SELECT available_at, value FROM data_series WHERE name = ? ORDER BY available_at", name)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", name, err)
	}
	defer rows.Close()
	var out []Point
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.Time, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Cache answers strategy.Data from a loader, loading each series on first use
// and again once it is older than TTL (0 = never reload, for backtests).
type Cache struct {
	Load func(ctx context.Context, name string) ([]Point, error)
	TTL  time.Duration
	Logf func(format string, args ...any)
	Now  func() time.Time

	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	points   []Point
	loadedAt time.Time
}

const loadTimeout = 10 * time.Second

// At implements strategy.Data.
func (c *Cache) At(name string, t time.Time) (float64, bool) {
	points := c.points(name)
	i := sort.Search(len(points), func(i int) bool { return points[i].Time.After(t) })
	if i == 0 {
		return 0, false
	}
	return points[i-1].Value, true
}

func (c *Cache) points(name string) []Point {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]*cacheEntry{}
	}
	e := c.entries[name]
	if e != nil && (c.TTL == 0 || now().Sub(e.loadedAt) < c.TTL) {
		return e.points
	}
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	defer cancel()
	points, err := c.Load(ctx, name)
	if err != nil {
		if c.Logf != nil {
			c.Logf("data %s: %v", name, err)
		}
		if e != nil {
			points = e.points // keep serving the last good copy
		}
	}
	// A failed load is also kept until TTL, so it is not retried on every bar.
	c.entries[name] = &cacheEntry{points: points, loadedAt: now()}
	return points
}
