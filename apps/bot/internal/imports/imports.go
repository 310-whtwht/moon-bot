// Package imports carries out the price-history downloads requested from the
// UI (table data_imports), one at a time.
package imports

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
)

// Import is one requested download.
type Import struct {
	ID        string
	Symbol    string
	Timeframe market.Timeframe
	From      time.Time
}

// Store is the queue of requests.
type Store interface {
	// ResetRunning puts downloads that were interrupted (the bot stopped)
	// back in the queue. Downloads resume where they left off.
	ResetRunning(ctx context.Context) error
	// Claim takes the oldest pending request, or returns nil when there is none.
	Claim(ctx context.Context) (*Import, error)
	Progress(ctx context.Context, id string, stored int, progress string) error
	// Finish records the outcome; errText is empty on success.
	Finish(ctx context.Context, id string, stored int, errText string) error
}

// Runner works through the queue.
type Runner struct {
	Store  Store
	Source broker.MarketData
	Bars   marketdata.BarStore
	// Interval is how often the queue is checked.
	Interval time.Duration
	Now      func() time.Time
	Logf     func(format string, args ...any)
}

// Run processes requests until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Interval <= 0 {
		r.Interval = 5 * time.Second
	}
	if err := r.Store.ResetRunning(ctx); err != nil {
		r.Logf("imports: reset interrupted downloads: %v", err)
	}
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		r.Drain(ctx)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Drain processes every pending request, oldest first.
func (r *Runner) Drain(ctx context.Context) {
	for ctx.Err() == nil {
		imp, err := r.Store.Claim(ctx)
		if err != nil {
			r.Logf("imports: claim: %v", err)
			return
		}
		if imp == nil {
			return
		}
		r.process(ctx, *imp)
	}
}

func (r *Runner) process(ctx context.Context, imp Import) {
	r.Logf("imports: %s %s from %s: started", imp.Symbol, imp.Timeframe, imp.From.Format("2006-01-02"))

	// The count shown is the bars stored so far, BID and ASK together.
	stored := map[market.PriceType]int{}
	bf := &marketdata.Backfiller{
		Source: r.Source, Store: r.Bars,
		Logf: func(string, ...any) {}, // progress goes to the table, not the log
		OnProgress: func(pt market.PriceType, upTo time.Time, n int) {
			stored[pt] = n
			text := fmt.Sprintf("%s %s", pt, upTo.Format("2006-01-02"))
			if err := r.Store.Progress(ctx, imp.ID, stored[market.PriceBid]+stored[market.PriceAsk], text); err != nil {
				r.Logf("imports: record progress: %v", err)
			}
		},
	}
	results, err := bf.Run(ctx, marketdata.BackfillRequest{
		Symbol: imp.Symbol, Timeframe: imp.Timeframe,
		PriceTypes: []market.PriceType{market.PriceBid, market.PriceAsk},
		From:       imp.From, To: r.Now().UTC(),
	})
	total := 0
	for _, res := range results {
		total += res.Stored
	}

	if ctx.Err() != nil {
		// Shutting down: leave it running so the next start resumes it.
		return
	}
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	// The bot is stopping only if ctx is done; otherwise always record the outcome.
	if ferr := r.Store.Finish(context.WithoutCancel(ctx), imp.ID, total, errText); ferr != nil {
		r.Logf("imports: record outcome: %v", ferr)
	}
	if err != nil {
		r.Logf("imports: %s %s: failed after %d bars: %v", imp.Symbol, imp.Timeframe, total, err)
		return
	}
	r.Logf("imports: %s %s: completed, %d bars stored", imp.Symbol, imp.Timeframe, total)
}

// MySQLStore is the queue in the data_imports table.
type MySQLStore struct {
	DB *sql.DB
}

func (s *MySQLStore) ResetRunning(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE data_imports SET status = 'pending' WHERE status = 'running'`)
	return err
}

func (s *MySQLStore) Claim(ctx context.Context) (*Import, error) {
	for {
		var imp Import
		var tf string
		err := s.DB.QueryRowContext(ctx, `
SELECT id, symbol, timeframe, from_date FROM data_imports
WHERE status = 'pending' ORDER BY created_at, id LIMIT 1`).Scan(&imp.ID, &imp.Symbol, &tf, &imp.From)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		// Only one worker wins the row; a loser (or a cancelled row) tries the next.
		res, err := s.DB.ExecContext(ctx,
			`UPDATE data_imports SET status = 'running', started_at = ? WHERE id = ? AND status = 'pending'`,
			time.Now().UTC(), imp.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			imp.Timeframe = market.Timeframe(tf)
			imp.From = imp.From.UTC()
			return &imp, nil
		}
	}
}

func (s *MySQLStore) Progress(ctx context.Context, id string, stored int, progress string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE data_imports SET bars_stored = ?, progress = ? WHERE id = ?`, stored, progress, id)
	return err
}

func (s *MySQLStore) Finish(ctx context.Context, id string, stored int, errText string) error {
	status, errValue := "completed", sql.NullString{}
	if errText != "" {
		status, errValue = "failed", sql.NullString{String: errText, Valid: true}
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE data_imports SET status = ?, bars_stored = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, stored, errValue, time.Now().UTC(), id)
	return err
}
