package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// BotRepository reads and controls the trading bot's state: heartbeats, kill
// switches, deployments and positions.
type BotRepository struct {
	db *sql.DB
}

func NewBotRepository(db *sql.DB) *BotRepository {
	return &BotRepository{db: db}
}

type BotHeartbeat struct {
	Instance            string    `json:"instance"`
	StartedAt           time.Time `json:"started_at"`
	LastSeenAt          time.Time `json:"last_seen_at"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
	Runners             int       `json:"runners"`
	// Alive is true while the last heartbeat is recent (within 3 poll intervals).
	Alive bool `json:"alive"`
}

func (r *BotRepository) Heartbeats(ctx context.Context, now time.Time) ([]BotHeartbeat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT instance, started_at, last_seen_at, poll_interval_seconds, runners FROM bot_heartbeats ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []BotHeartbeat{}
	for rows.Next() {
		var h BotHeartbeat
		if err := rows.Scan(&h.Instance, &h.StartedAt, &h.LastSeenAt, &h.PollIntervalSeconds, &h.Runners); err != nil {
			return nil, err
		}
		grace := time.Duration(h.PollIntervalSeconds*3) * time.Second
		if grace < time.Minute {
			grace = time.Minute
		}
		h.Alive = now.Sub(h.LastSeenAt) <= grace
		out = append(out, h)
	}
	return out, rows.Err()
}

type KillSwitch struct {
	Scope          string    `json:"scope"`
	Active         bool      `json:"active"`
	ClosePositions bool      `json:"close_positions"`
	Reason         *string   `json:"reason"`
	UpdatedBy      *string   `json:"updated_by"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (r *BotRepository) KillSwitches(ctx context.Context) ([]KillSwitch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT scope, active, close_positions, reason, updated_by, updated_at FROM kill_switches ORDER BY scope = 'global' DESC, scope`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []KillSwitch{}
	for rows.Next() {
		var k KillSwitch
		if err := rows.Scan(&k.Scope, &k.Active, &k.ClosePositions, &k.Reason, &k.UpdatedBy, &k.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// SetKillSwitch turns a switch on or off. Turning it off clears close_positions.
func (r *BotRepository) SetKillSwitch(ctx context.Context, k KillSwitch) error {
	if !k.Active {
		k.ClosePositions = false
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO kill_switches (scope, active, close_positions, reason, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE active = VALUES(active), close_positions = VALUES(close_positions),
  reason = VALUES(reason), updated_by = VALUES(updated_by), updated_at = VALUES(updated_at)`,
		k.Scope, k.Active, k.ClosePositions, k.Reason, k.UpdatedBy, time.Now().UTC())
	return err
}

type Deployment struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	StrategyID    string  `json:"strategy_id"`
	StrategyName  string  `json:"strategy_name"`
	ActiveVersion *string `json:"active_version"`
	Broker        string  `json:"broker"`
	AccountID     string  `json:"account_id"`
	Symbol        string  `json:"symbol"`
	Timeframe     string  `json:"timeframe"`
	Units         float64 `json:"units"`
	Enabled       bool    `json:"enabled"`
}

func (r *BotRepository) Deployments(ctx context.Context) ([]Deployment, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT d.id, d.name, d.strategy_id, p.name,
  (SELECT v.version FROM strategy_versions v WHERE v.package_id = d.strategy_id AND v.is_active = TRUE ORDER BY v.created_at DESC LIMIT 1),
  d.broker, d.account_id, d.symbol, d.timeframe, d.units, d.enabled
FROM deployments d JOIN strategy_packages p ON p.id = d.strategy_id
ORDER BY d.created_at, d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Deployment{}
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.Name, &d.StrategyID, &d.StrategyName, &d.ActiveVersion,
			&d.Broker, &d.AccountID, &d.Symbol, &d.Timeframe, &d.Units, &d.Enabled); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type Position struct {
	ID           string   `json:"id"`
	DeploymentID *string  `json:"deployment_id"`
	Broker       string   `json:"broker"`
	AccountID    string   `json:"account_id"`
	Symbol       string   `json:"symbol"`
	Side         string   `json:"side"`
	Quantity     float64  `json:"quantity"`
	OpenPrice    float64  `json:"open_price"`
	StopPrice    *float64 `json:"stop_price"`
	// StopOrderID is set when the stop is also held at the broker.
	StopOrderID *string    `json:"stop_order_id"`
	ClosePrice  *float64   `json:"close_price"`
	RealizedPnL *float64   `json:"realized_pnl"`
	Fees        float64    `json:"fees"`
	Status      string     `json:"status"`
	OpenedAt    time.Time  `json:"opened_at"`
	ClosedAt    *time.Time `json:"closed_at"`
}

// Positions returns open positions, or the most recently closed ones.
func (r *BotRepository) Positions(ctx context.Context, status string, limit int) ([]Position, error) {
	order := "opened_at DESC"
	if status == "closed" {
		order = "closed_at DESC"
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, deployment_id, broker, account_id, symbol, side, quantity, open_price, stop_price, stop_order_id, close_price,
  realized_pnl, fees, status, opened_at, closed_at
FROM positions WHERE status = ? ORDER BY `+order+` LIMIT ?`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Position{}
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.DeploymentID, &p.Broker, &p.AccountID, &p.Symbol, &p.Side, &p.Quantity,
			&p.OpenPrice, &p.StopPrice, &p.StopOrderID, &p.ClosePrice, &p.RealizedPnL, &p.Fees, &p.Status, &p.OpenedAt, &p.ClosedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RealizedPnL sums the net realised P&L and counts positions closed since `since`.
func (r *BotRepository) RealizedPnL(ctx context.Context, since time.Time) (pnl float64, closed int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(realized_pnl), 0), COUNT(*) FROM positions WHERE status = 'closed' AND closed_at >= ?`,
		since.UTC()).Scan(&pnl, &closed)
	return pnl, closed, err
}

// ChartPositions returns a symbol's open positions and those closed since
// `since`, oldest first, for drawing on a price chart.
func (r *BotRepository) ChartPositions(ctx context.Context, symbol string, since time.Time) ([]Position, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, deployment_id, broker, account_id, symbol, side, quantity, open_price, stop_price, stop_order_id, close_price,
  realized_pnl, fees, status, opened_at, closed_at
FROM positions WHERE symbol = ? AND (status = 'open' OR closed_at >= ?) ORDER BY opened_at LIMIT 500`, symbol, since.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Position{}
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.DeploymentID, &p.Broker, &p.AccountID, &p.Symbol, &p.Side, &p.Quantity,
			&p.OpenPrice, &p.StopPrice, &p.StopOrderID, &p.ClosePrice, &p.RealizedPnL, &p.Fees, &p.Status, &p.OpenedAt, &p.ClosedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeployedStrategy is the strategy a deployment currently trades with.
type DeployedStrategy struct {
	Type    string             `json:"type"`
	Version string             `json:"version"`
	Enabled bool               `json:"enabled"`
	Params  map[string]float64 `json:"params"`
	// Script is the source of a `script` strategy (not sent to the chart).
	Script string `json:"-"`
}

// DeployedStrategy returns the active strategy version deployed on a symbol
// and timeframe (an enabled deployment first), or nil when there is none.
func (r *BotRepository) DeployedStrategy(ctx context.Context, symbol, timeframe string) (*DeployedStrategy, error) {
	var (
		s         DeployedStrategy
		versionID string
	)
	err := r.db.QueryRowContext(ctx, `
SELECT v.id, v.code, COALESCE(v.script, ''), v.version, d.enabled
FROM deployments d JOIN strategy_versions v ON v.package_id = d.strategy_id AND v.is_active = TRUE
WHERE d.symbol = ? AND d.timeframe = ?
ORDER BY d.enabled DESC, v.created_at DESC LIMIT 1`, symbol, timeframe).Scan(&versionID, &s.Type, &s.Script, &s.Version, &s.Enabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT param_name, default_value FROM strategy_params WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s.Params = map[string]float64{}
	for rows.Next() {
		var (
			name  string
			value sql.NullString
		)
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		if v, err := strconv.ParseFloat(value.String, 64); err == nil {
			s.Params[name] = v
		}
	}
	return &s, rows.Err()
}

// BarDecision is what the strategy decided on one closed bar.
type BarDecision struct {
	BarTime   time.Time `json:"bar_time"`
	Close     float64   `json:"close"`
	Action    string    `json:"action"`  // HOLD, ENTER_LONG, ENTER_SHORT or EXIT
	Holding   string    `json:"holding"` // "", "BUY" or "SELL"
	Detail    string    `json:"detail"`
	DecidedAt time.Time `json:"decided_at"`
}

// BarDecisions returns the latest decisions of the deployments trading a
// symbol on a timeframe, newest first.
func (r *BotRepository) BarDecisions(ctx context.Context, symbol, timeframe string, limit int) ([]BarDecision, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT b.bar_time, b.close, b.action, b.holding, b.detail, b.decided_at
FROM bar_decisions b JOIN deployments d ON d.id = b.deployment_id
WHERE d.symbol = ? AND d.timeframe = ?
ORDER BY b.bar_time DESC LIMIT ?`, symbol, timeframe, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []BarDecision{}
	for rows.Next() {
		var d BarDecision
		if err := rows.Scan(&d.BarTime, &d.Close, &d.Action, &d.Holding, &d.Detail, &d.DecidedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ErrDeploymentTaken means the account already trades that symbol.
var ErrDeploymentTaken = errors.New("a deployment for this account and symbol already exists")

// ErrDeploymentInUse means the deployment still has an open position.
var ErrDeploymentInUse = errors.New("the deployment has an open position")

// NewDeployment is what a deployment is created from. It starts disabled.
type NewDeployment struct {
	Name       string
	StrategyID string
	Broker     string
	AccountID  string
	Symbol     string
	Timeframe  string
	Units      float64
}

// StrategyExists reports whether a strategy package exists.
func (r *BotRepository) StrategyExists(ctx context.Context, id string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM strategy_packages WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

// CreateDeployment adds a disabled deployment and returns its ID.
func (r *BotRepository) CreateDeployment(ctx context.Context, d NewDeployment) (string, error) {
	var taken int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deployments WHERE broker = ? AND account_id = ? AND symbol = ?`,
		d.Broker, d.AccountID, d.Symbol).Scan(&taken); err != nil {
		return "", err
	}
	if taken > 0 {
		return "", ErrDeploymentTaken
	}
	id := uuid.NewString()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO deployments (id, name, strategy_id, broker, account_id, symbol, timeframe, units, enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, FALSE)`,
		id, d.Name, d.StrategyID, d.Broker, d.AccountID, d.Symbol, d.Timeframe, d.Units)
	return id, err
}

// DeploymentPatch changes a deployment; nil fields are left as they are.
// The instrument, timeframe and strategy are fixed: the bot's state (warm-up,
// open position, order IDs) is tied to them, so those need a new deployment.
type DeploymentPatch struct {
	Name    *string
	Units   *float64
	Enabled *bool
}

// UpdateDeployment returns false when the deployment does not exist.
func (r *BotRepository) UpdateDeployment(ctx context.Context, id string, p DeploymentPatch) (bool, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deployments WHERE id = ?`, id).Scan(&exists); err != nil {
		return false, err
	}
	if exists == 0 {
		return false, nil
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE deployments SET name = COALESCE(?, name), units = COALESCE(?, units), enabled = COALESCE(?, enabled)
WHERE id = ?`, p.Name, p.Units, p.Enabled, id)
	return true, err
}

// DeploymentSymbol returns the symbol a deployment trades ("" when it does not exist).
func (r *BotRepository) DeploymentSymbol(ctx context.Context, id string) (string, error) {
	var symbol string
	err := r.db.QueryRowContext(ctx, `SELECT symbol FROM deployments WHERE id = ?`, id).Scan(&symbol)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return symbol, err
}

// DeleteDeployment removes a deployment that holds no position. Its past
// positions and orders are kept. Returns false when it does not exist.
func (r *BotRepository) DeleteDeployment(ctx context.Context, id string) (bool, error) {
	var open int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM positions WHERE deployment_id = ? AND status = 'open'`, id).Scan(&open); err != nil {
		return false, err
	}
	if open > 0 {
		return false, ErrDeploymentInUse
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM deployments WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
