package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Backtest represents a backtest execution
type Backtest struct {
	ID              string          `json:"id" db:"id"`
	Name            string          `json:"name" db:"name"`
	StrategyID      string          `json:"strategy_id" db:"strategy_id"`
	Symbols         []string        `json:"symbols" db:"symbols"`
	StartDate       time.Time       `json:"start_date" db:"start_date"`
	EndDate         time.Time       `json:"end_date" db:"end_date"`
	Parameters      json.RawMessage `json:"parameters" db:"parameters"`
	Status          string          `json:"status" db:"status"`
	Progress        float64         `json:"progress" db:"progress"`
	Results         json.RawMessage `json:"results" db:"results"`
	Error           *string         `json:"error" db:"error"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at" db:"completed_at"`
}

// BacktestRepository handles database operations for backtests
type BacktestRepository struct {
	db *sql.DB
}

// NewBacktestRepository creates a new backtest repository
func NewBacktestRepository(db *sql.DB) *BacktestRepository {
	return &BacktestRepository{db: db}
}

// CreateBacktest creates a new backtest
func (r *BacktestRepository) CreateBacktest(ctx context.Context, backtest *Backtest) error {
	backtest.ID = uuid.New().String()
	backtest.CreatedAt = time.Now()
	backtest.UpdatedAt = time.Now()
	backtest.Status = "pending"
	backtest.Progress = 0

	// Convert symbols slice to JSON
	symbolsJSON, err := json.Marshal(backtest.Symbols)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO backtests (id, name, strategy_id, symbols, start_date, end_date, parameters, status, progress, results, error, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		backtest.ID, backtest.Name, backtest.StrategyID, symbolsJSON, backtest.StartDate, backtest.EndDate,
		backtest.Parameters, backtest.Status, backtest.Progress, backtest.Results, backtest.Error,
		backtest.CreatedAt, backtest.UpdatedAt, backtest.CompletedAt)
	return err
}

// GetBacktestByID retrieves a backtest by ID
func (r *BacktestRepository) GetBacktestByID(ctx context.Context, id string) (*Backtest, error) {
	query := `
		SELECT id, name, strategy_id, symbols, start_date, end_date, parameters, status, progress, results, error, created_at, updated_at, completed_at
		FROM backtests WHERE id = ?
	`

	var backtest Backtest
	var symbolsJSON []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&backtest.ID, &backtest.Name, &backtest.StrategyID, &symbolsJSON, &backtest.StartDate, &backtest.EndDate,
		&backtest.Parameters, &backtest.Status, &backtest.Progress, &backtest.Results, &backtest.Error,
		&backtest.CreatedAt, &backtest.UpdatedAt, &backtest.CompletedAt)
	if err != nil {
		return nil, err
	}

	// Parse symbols JSON
	if err := json.Unmarshal(symbolsJSON, &backtest.Symbols); err != nil {
		return nil, err
	}

	return &backtest, nil
}

// ListBacktests retrieves backtests with filtering
func (r *BacktestRepository) ListBacktests(ctx context.Context, strategyID, status *string, limit, offset int) ([]*Backtest, error) {
	query := `
		SELECT id, name, strategy_id, symbols, start_date, end_date, parameters, status, progress, results, error, created_at, updated_at, completed_at
		FROM backtests WHERE 1=1
	`
	var args []interface{}

	if strategyID != nil {
		query += " AND strategy_id = ?"
		args = append(args, *strategyID)
	}
	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backtests []*Backtest
	for rows.Next() {
		var backtest Backtest
		var symbolsJSON []byte
		err := rows.Scan(
			&backtest.ID, &backtest.Name, &backtest.StrategyID, &symbolsJSON, &backtest.StartDate, &backtest.EndDate,
			&backtest.Parameters, &backtest.Status, &backtest.Progress, &backtest.Results, &backtest.Error,
			&backtest.CreatedAt, &backtest.UpdatedAt, &backtest.CompletedAt)
		if err != nil {
			return nil, err
		}

		// Parse symbols JSON
		if err := json.Unmarshal(symbolsJSON, &backtest.Symbols); err != nil {
			return nil, err
		}

		backtests = append(backtests, &backtest)
	}

	return backtests, nil
}

// UpdateBacktest updates a backtest
func (r *BacktestRepository) UpdateBacktest(ctx context.Context, backtest *Backtest) error {
	backtest.UpdatedAt = time.Now()

	// Convert symbols slice to JSON
	symbolsJSON, err := json.Marshal(backtest.Symbols)
	if err != nil {
		return err
	}

	query := `
		UPDATE backtests
		SET name = ?, strategy_id = ?, symbols = ?, start_date = ?, end_date = ?, parameters = ?, status = ?, progress = ?, results = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		backtest.Name, backtest.StrategyID, symbolsJSON, backtest.StartDate, backtest.EndDate,
		backtest.Parameters, backtest.Status, backtest.Progress, backtest.Results, backtest.Error,
		backtest.UpdatedAt, backtest.CompletedAt, backtest.ID)
	return err
}

// UpdateBacktestStatus updates only the status and progress of a backtest
func (r *BacktestRepository) UpdateBacktestStatus(ctx context.Context, id, status string, progress float64, results json.RawMessage, errMsg *string) error {
	now := time.Now()
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		completedAt = &now
	}

	query := `
		UPDATE backtests
		SET status = ?, progress = ?, results = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query, status, progress, results, errMsg, now, completedAt, id)
	return err
}

// DeleteBacktest deletes a backtest
func (r *BacktestRepository) DeleteBacktest(ctx context.Context, id string) error {
	query := `DELETE FROM backtests WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}