// Package jobs runs work queued by the API, currently backtests.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/marketdata"
)

// maxEquityPoints keeps stored results small enough for the UI.
const maxEquityPoints = 500

// Claim is a backtest that this worker has moved from pending to running.
type Claim struct {
	Spec  backtest.JobSpec
	Start time.Time
	End   time.Time // inclusive date
}

// BacktestStore is the backtests table as seen by the worker.
type BacktestStore interface {
	// Claim marks a pending backtest as running. ok is false when it is not
	// pending any more (cancelled, deleted, already handled).
	Claim(ctx context.Context, id string) (c Claim, ok bool, err error)
	// Complete stores results unless the backtest was cancelled meanwhile.
	Complete(ctx context.Context, id string, results []byte) error
	Fail(ctx context.Context, id string, msg string) error
}

// BacktestRunner executes queued backtests.
type BacktestRunner struct {
	Store BacktestStore
	Bars  marketdata.BarStore
	Logf  func(format string, args ...any)
}

func (r *BacktestRunner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Handle runs one backtest. Failures of the backtest itself are recorded on
// the row; only storage errors are returned.
func (r *BacktestRunner) Handle(ctx context.Context, id string) error {
	claim, ok, err := r.Store.Claim(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		r.logf("backtest %s: not pending, skipped", id)
		return nil
	}

	started := time.Now()
	results, err := r.run(ctx, claim)
	if err != nil {
		r.logf("backtest %s failed: %v", id, err)
		return r.Store.Fail(ctx, id, err.Error())
	}
	if err := r.Store.Complete(ctx, id, results); err != nil {
		return err
	}
	r.logf("backtest %s completed in %s", id, time.Since(started).Round(time.Millisecond))
	return nil
}

func (r *BacktestRunner) run(ctx context.Context, c Claim) ([]byte, error) {
	def, err := c.Spec.Resolve()
	if err != nil {
		return nil, err
	}
	to := c.End.AddDate(0, 0, 1) // end_date is inclusive
	candles, err := backtest.LoadCandles(ctx, r.Bars, c.Spec.Key(), c.Spec.Timeframe, c.Start, to)
	if err != nil {
		return nil, err
	}
	res, err := backtest.Run(candles, backtest.Config{
		Strategy: def, Params: c.Spec.StrategyParams,
		Units: c.Spec.Units, InitialBalance: c.Spec.InitialBalance,
	})
	if err != nil {
		return nil, err
	}
	res.Equity = backtest.Downsample(res.Equity, maxEquityPoints)
	return json.Marshal(res)
}

// MySQLBacktestStore implements BacktestStore on the backtests table.
type MySQLBacktestStore struct {
	DB *sql.DB
}

func (s *MySQLBacktestStore) Claim(ctx context.Context, id string) (Claim, bool, error) {
	now := time.Now().UTC()
	res, err := s.DB.ExecContext(ctx,
		`UPDATE backtests SET status = 'running', progress = 0, updated_at = ? WHERE id = ? AND status = 'pending'`, now, id)
	if err != nil {
		return Claim{}, false, fmt.Errorf("claim backtest: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Claim{}, false, nil
	}

	var c Claim
	var params []byte
	err = s.DB.QueryRowContext(ctx,
		`SELECT parameters, start_date, end_date FROM backtests WHERE id = ?`, id).Scan(&params, &c.Start, &c.End)
	if err != nil {
		return Claim{}, false, fmt.Errorf("load backtest: %w", err)
	}
	if err := json.Unmarshal(params, &c.Spec); err != nil {
		// Mark it failed here: a malformed row would otherwise stay running forever.
		_ = s.Fail(ctx, id, "invalid parameters: "+err.Error())
		return Claim{}, false, nil
	}
	c.Start, c.End = c.Start.UTC(), c.End.UTC()
	return c, true, nil
}

func (s *MySQLBacktestStore) Complete(ctx context.Context, id string, results []byte) error {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx,
		`UPDATE backtests SET status = 'completed', progress = 100, results = ?, error = NULL, updated_at = ?, completed_at = ?
WHERE id = ? AND status = 'running'`, results, now, now, id)
	if err != nil {
		return fmt.Errorf("complete backtest: %w", err)
	}
	return nil
}

func (s *MySQLBacktestStore) Fail(ctx context.Context, id string, msg string) error {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx,
		`UPDATE backtests SET status = 'failed', error = ?, updated_at = ?, completed_at = ?
WHERE id = ? AND status IN ('pending', 'running')`, msg, now, now, id)
	if err != nil {
		return fmt.Errorf("fail backtest: %w", err)
	}
	return nil
}

var errNoID = errors.New("job has no backtest id")
