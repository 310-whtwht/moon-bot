package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// MarketDataRepository reads what price history is stored and manages the
// requests to download more (table data_imports).
type MarketDataRepository struct {
	db *sql.DB
}

func NewMarketDataRepository(db *sql.DB) *MarketDataRepository {
	return &MarketDataRepository{db: db}
}

// Coverage is the stored history of one symbol and timeframe.
type Coverage struct {
	Symbol    string    `json:"symbol"`
	Timeframe string    `json:"timeframe"`
	BidBars   int       `json:"bid_bars"`
	AskBars   int       `json:"ask_bars"`
	First     time.Time `json:"first"`
	Last      time.Time `json:"last"`
	// Usable says whether a backtest can run on it; Missing explains why not.
	Usable  bool   `json:"usable"`
	Missing string `json:"missing"`
}

// Coverage lists the stored history per symbol and timeframe.
func (r *MarketDataRepository) Coverage(ctx context.Context, broker string) ([]Coverage, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT symbol, timeframe, COALESCE(SUM(price_type = 'BID'), 0), COALESCE(SUM(price_type = 'ASK'), 0), MIN(open_time), MAX(open_time)
FROM bars WHERE broker = ? GROUP BY symbol, timeframe ORDER BY symbol, timeframe`, broker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Coverage{}
	for rows.Next() {
		var c Coverage
		if err := rows.Scan(&c.Symbol, &c.Timeframe, &c.BidBars, &c.AskBars, &c.First, &c.Last); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DataImport is one request to download history.
type DataImport struct {
	ID         string     `json:"id"`
	Symbol     string     `json:"symbol"`
	Timeframe  string     `json:"timeframe"`
	FromDate   string     `json:"from_date"` // YYYY-MM-DD
	Status     string     `json:"status"`    // pending, running, completed, failed, cancelled
	BarsStored int        `json:"bars_stored"`
	Progress   string     `json:"progress"`
	Error      *string    `json:"error"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// Imports returns the most recent requests, newest first.
func (r *MarketDataRepository) Imports(ctx context.Context, limit int) ([]DataImport, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, symbol, timeframe, DATE_FORMAT(from_date, '%Y-%m-%d'), status, bars_stored, progress, error, created_at, started_at, finished_at
FROM data_imports ORDER BY created_at DESC, id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DataImport{}
	for rows.Next() {
		var d DataImport
		if err := rows.Scan(&d.ID, &d.Symbol, &d.Timeframe, &d.FromDate, &d.Status, &d.BarsStored, &d.Progress,
			&d.Error, &d.CreatedAt, &d.StartedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ErrImportQueued means the same symbol and timeframe is already waiting or downloading.
var ErrImportQueued = errors.New("an import for this symbol and timeframe is already queued")

// CreateImport queues a download and returns its ID.
func (r *MarketDataRepository) CreateImport(ctx context.Context, broker, symbol, timeframe string, from time.Time) (string, error) {
	var queued int
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM data_imports WHERE symbol = ? AND timeframe = ? AND status IN ('pending', 'running')`,
		symbol, timeframe).Scan(&queued); err != nil {
		return "", err
	}
	if queued > 0 {
		return "", ErrImportQueued
	}
	id := uuid.NewString()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO data_imports (id, broker, symbol, timeframe, from_date, status, created_at)
VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
		id, broker, symbol, timeframe, from.Format("2006-01-02"), time.Now().UTC())
	return id, err
}

// CancelImport withdraws a request that has not started. Returns false when
// there is no such pending request (unknown, already running or finished).
func (r *MarketDataRepository) CancelImport(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE data_imports SET status = 'cancelled', finished_at = ? WHERE id = ? AND status = 'pending'`,
		time.Now().UTC(), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
