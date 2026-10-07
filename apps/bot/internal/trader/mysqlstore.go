package trader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/risk"
	"github.com/moomoo-trading/core/strategy"
	"github.com/shopspring/decimal"
)

// MySQLStore implements Store on the application database (loc=UTC).
type MySQLStore struct {
	DB *sql.DB
}

var _ Store = (*MySQLStore)(nil)

func (s *MySQLStore) Deployments(ctx context.Context) ([]Deployment, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, name, strategy_id, broker, account_id, symbol, timeframe, units, enabled,
  entry_order, limit_wait_seconds, limit_fallback, max_spread
FROM deployments ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Deployment
	for rows.Next() {
		var d Deployment
		var tf string
		var waitSeconds int
		var maxSpread decimal.NullDecimal
		if err := rows.Scan(&d.ID, &d.Name, &d.StrategyID, &d.Broker, &d.AccountID, &d.Symbol, &tf, &d.Units, &d.Enabled,
			&d.EntryOrder, &waitSeconds, &d.LimitFallback, &maxSpread); err != nil {
			return nil, err
		}
		d.Timeframe = market.Timeframe(tf)
		d.LimitWait = time.Duration(waitSeconds) * time.Second
		d.MaxSpread = maxSpread.Decimal
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *MySQLStore) ActiveVersion(ctx context.Context, strategyID string) (Version, error) {
	var v Version
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, code, COALESCE(script, '') FROM strategy_versions WHERE package_id = ? AND is_active = TRUE ORDER BY created_at DESC LIMIT 1`,
		strategyID).Scan(&v.ID, &v.Type, &v.Script)
	if err != nil {
		return Version{}, fmt.Errorf("active version of %s: %w", strategyID, err)
	}
	return s.withParams(ctx, v)
}

func (s *MySQLStore) Version(ctx context.Context, versionID string) (Version, error) {
	v := Version{ID: versionID}
	err := s.DB.QueryRowContext(ctx, `SELECT code, COALESCE(script, '') FROM strategy_versions WHERE id = ?`, versionID).Scan(&v.Type, &v.Script)
	if err != nil {
		return Version{}, fmt.Errorf("version %s: %w", versionID, err)
	}
	return s.withParams(ctx, v)
}

func (s *MySQLStore) withParams(ctx context.Context, v Version) (Version, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT param_name, default_value FROM strategy_params WHERE version_id = ?`, v.ID)
	if err != nil {
		return Version{}, err
	}
	defer rows.Close()

	v.Params = strategy.Params{}
	for rows.Next() {
		var name string
		var value sql.NullString
		if err := rows.Scan(&name, &value); err != nil {
			return Version{}, err
		}
		if f, err := strconv.ParseFloat(value.String, 64); value.Valid && err == nil {
			v.Params[name] = f
		}
	}
	return v, rows.Err()
}

const positionColumns = `id, deployment_id, broker, account_id, broker_position_id, symbol, side, quantity,
open_price, stop_price, stop_order_id, fees, strategy_id, strategy_version_id, opened_at`

func scanPosition(scan func(...any) error) (Position, error) {
	var p Position
	var deploymentID, brokerPositionID, strategyID, versionID, stopOrderID sql.NullString
	var side string
	var stop decimal.NullDecimal
	err := scan(&p.ID, &deploymentID, &p.Broker, &p.AccountID, &brokerPositionID, &p.Symbol, &side, &p.Units,
		&p.OpenPrice, &stop, &stopOrderID, &p.Fees, &strategyID, &versionID, &p.OpenedAt)
	if err != nil {
		return Position{}, err
	}
	p.DeploymentID, p.BrokerPositionID = deploymentID.String, brokerPositionID.String
	p.StrategyID, p.StrategyVersionID = strategyID.String, versionID.String
	p.StopPrice = stop.Decimal
	p.StopOrderID = stopOrderID.String
	p.Side = sideFromDB(side)
	p.OpenedAt = p.OpenedAt.UTC()
	return p, nil
}

func (s *MySQLStore) OpenPosition(ctx context.Context, deploymentID string) (*Position, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT `+positionColumns+` FROM positions WHERE deployment_id = ? AND status = 'open' ORDER BY opened_at DESC LIMIT 1`,
		deploymentID)
	p, err := scanPosition(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *MySQLStore) OpenPositions(ctx context.Context, brokerName, accountID string) ([]Position, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+positionColumns+` FROM positions WHERE broker = ? AND account_id = ? AND status = 'open' ORDER BY opened_at`,
		brokerName, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Position
	for rows.Next() {
		p, err := scanPosition(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func sideToDB(side broker.Side) string { return strings.ToLower(string(side)) }

func sideFromDB(side string) broker.Side { return broker.Side(strings.ToUpper(side)) }

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// mysqlDuplicateEntry is the error number for a unique key violation.
const mysqlDuplicateEntry = 1062

func (s *MySQLStore) CreateOrder(ctx context.Context, o Order) (bool, error) {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO orders (id, client_order_id, broker, account_id, strategy_id, strategy_version_id, deployment_id,
  symbol, side, order_type, settle_type, quantity, status, broker_position_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'market', ?, ?, 'pending', ?, ?, ?)`,
		uuid.NewString(), o.ClientOrderID, o.Broker, o.AccountID, nullable(o.StrategyID), nullable(o.StrategyVersionID),
		nullable(o.DeploymentID), o.Symbol, sideToDB(o.Side), o.SettleType, o.Units.String(),
		nullable(o.BrokerPositionID), now, now)
	var dup *mysql.MySQLError
	if errors.As(err, &dup) && dup.Number == mysqlDuplicateEntry {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// MarkOrderFilled updates the order and records the trade in one transaction.
func (s *MySQLStore) MarkOrderFilled(ctx context.Context, o Order, f Fill) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var orderID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM orders WHERE client_order_id = ?`, o.ClientOrderID).Scan(&orderID); err != nil {
		return fmt.Errorf("find order %s: %w", o.ClientOrderID, err)
	}
	positionID := f.BrokerPositionID
	if positionID == "" {
		positionID = o.BrokerPositionID
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE orders SET status = 'filled', filled_quantity = ?, avg_fill_price = ?, commission = ?,
  broker_order_id = ?, broker_position_id = ?, updated_at = ?
WHERE id = ?`,
		o.Units.String(), f.Price.String(), f.Fee.String(), f.BrokerOrderID, nullable(positionID), time.Now().UTC(), orderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO trades (id, order_id, broker, account_id, strategy_id, symbol, side, quantity, price, commission, broker_trade_id, trade_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), orderID, o.Broker, o.AccountID, nullable(o.StrategyID), o.Symbol, sideToDB(o.Side),
		o.Units.String(), f.Price.String(), f.Fee.String(), f.BrokerOrderID, f.At.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLStore) MarkOrderRejected(ctx context.Context, clientOrderID, reason string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE orders SET status = 'rejected', error_message = ?, updated_at = ? WHERE client_order_id = ?`,
		reason, time.Now().UTC(), clientOrderID)
	return err
}

// MarkOrderWorking marks the order as an accepted limit order waiting for a fill.
func (s *MySQLStore) MarkOrderWorking(ctx context.Context, clientOrderID, brokerOrderID string, limit, stop decimal.Decimal) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE orders SET status = 'submitted', order_type = 'limit', broker_order_id = ?, price = ?, stop_price = ?, updated_at = ?
WHERE client_order_id = ?`,
		brokerOrderID, limit.String(), stop.String(), time.Now().UTC(), clientOrderID)
	return err
}

func (s *MySQLStore) MarkOrderCancelled(ctx context.Context, clientOrderID, reason string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE orders SET status = 'cancelled', error_message = ?, updated_at = ? WHERE client_order_id = ?`,
		reason, time.Now().UTC(), clientOrderID)
	return err
}

// WorkingOrders returns limit entries still marked as waiting. Orders whose
// outcome is unknown carry an error message and are left to a human.
func (s *MySQLStore) WorkingOrders(ctx context.Context, deploymentID string) ([]WorkingOrder, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT client_order_id, broker_order_id FROM orders
WHERE deployment_id = ? AND status = 'submitted' AND order_type = 'limit'
  AND broker_order_id IS NOT NULL AND error_message IS NULL`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkingOrder
	for rows.Next() {
		var w WorkingOrder
		if err := rows.Scan(&w.ClientOrderID, &w.BrokerOrderID); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// MarkOrderUnknown keeps the order as 'submitted' (sent, outcome unknown) with the reason.
func (s *MySQLStore) MarkOrderUnknown(ctx context.Context, clientOrderID, reason string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE orders SET status = 'submitted', error_message = ?, updated_at = ? WHERE client_order_id = ?`,
		"outcome unknown: "+reason, time.Now().UTC(), clientOrderID)
	return err
}

func (s *MySQLStore) InsertPosition(ctx context.Context, p Position) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO positions (id, deployment_id, broker, account_id, broker_position_id, symbol, side, quantity,
  open_price, stop_price, stop_order_id, fees, strategy_id, strategy_version_id, status, opened_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?)`,
		p.ID, nullable(p.DeploymentID), p.Broker, p.AccountID, nullable(p.BrokerPositionID), p.Symbol, sideToDB(p.Side),
		p.Units.String(), p.OpenPrice.String(), p.StopPrice.String(), nullable(p.StopOrderID), p.Fees.String(),
		nullable(p.StrategyID), nullable(p.StrategyVersionID), p.OpenedAt.UTC())
	return err
}

func (s *MySQLStore) SetPositionStopOrder(ctx context.Context, positionID, stopOrderID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE positions SET stop_order_id = ? WHERE id = ?`, nullable(stopOrderID), positionID)
	return err
}

func (s *MySQLStore) ClosePosition(ctx context.Context, positionID string, closePrice, realizedPnL, closeFee decimal.Decimal, at time.Time) error {
	res, err := s.DB.ExecContext(ctx, `
UPDATE positions SET status = 'closed', close_price = ?, realized_pnl = ?, fees = fees + ?, closed_at = ?
WHERE id = ? AND status = 'open'`,
		closePrice.String(), realizedPnL.String(), closeFee.String(), at.UTC(), positionID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("position %s is not open", positionID)
	}
	return nil
}

func (s *MySQLStore) Exposure(ctx context.Context, brokerName, accountID string, dayStart, weekStart time.Time) (risk.Exposure, risk.Exposure, error) {
	var account, global risk.Exposure
	// mine marks the rows of this account; sums over everything are the global level.
	err := s.DB.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(status = 'open' AND broker = ? AND account_id = ?), 0),
  COALESCE(SUM(status = 'open'), 0),
  COALESCE(SUM(IF(status = 'closed' AND closed_at >= ? AND broker = ? AND account_id = ?, realized_pnl, 0)), 0),
  COALESCE(SUM(IF(status = 'closed' AND closed_at >= ?, realized_pnl, 0)), 0),
  COALESCE(SUM(IF(status = 'closed' AND closed_at >= ? AND broker = ? AND account_id = ?, realized_pnl, 0)), 0),
  COALESCE(SUM(IF(status = 'closed' AND closed_at >= ?, realized_pnl, 0)), 0)
FROM positions
WHERE status = 'open' OR closed_at >= ?`,
		brokerName, accountID,
		dayStart.UTC(), brokerName, accountID, dayStart.UTC(),
		weekStart.UTC(), brokerName, accountID, weekStart.UTC(),
		weekStart.UTC(),
	).Scan(&account.OpenPositions, &global.OpenPositions,
		&account.DailyPnLJPY, &global.DailyPnLJPY, &account.WeeklyPnLJPY, &global.WeeklyPnLJPY)
	if err != nil {
		return risk.Exposure{}, risk.Exposure{}, err
	}

	// A limit entry that is waiting for its fill will become a position: it
	// takes a slot now, so two waiting orders cannot both slip past the limit.
	var mine, all int
	err = s.DB.QueryRowContext(ctx, `
SELECT COALESCE(SUM(broker = ? AND account_id = ?), 0), COUNT(*) FROM orders
WHERE status = 'submitted' AND order_type = 'limit' AND settle_type = 'open'
  AND broker_order_id IS NOT NULL AND error_message IS NULL`, brokerName, accountID).Scan(&mine, &all)
	if err != nil {
		return risk.Exposure{}, risk.Exposure{}, err
	}
	account.OpenPositions += mine
	global.OpenPositions += all
	return account, global, nil
}

func (s *MySQLStore) RealizedPnL(ctx context.Context, brokerName, accountID string) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(realized_pnl), 0) FROM positions WHERE broker = ? AND account_id = ? AND status = 'closed'`,
		brokerName, accountID).Scan(&total)
	return total, err
}

var (
	_ KillStore = (*MySQLStore)(nil)
	_ OpsStore  = (*MySQLStore)(nil)
)

func (s *MySQLStore) KillSwitches(ctx context.Context) ([]KillSwitch, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT scope, active, close_positions, COALESCE(reason, ''), COALESCE(updated_by, ''), updated_at FROM kill_switches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KillSwitch
	for rows.Next() {
		var k KillSwitch
		if err := rows.Scan(&k.Scope, &k.Active, &k.ClosePositions, &k.Reason, &k.UpdatedBy, &k.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// SetKillSwitch turns a switch on or off. Turning it off also clears close_positions.
func (s *MySQLStore) SetKillSwitch(ctx context.Context, k KillSwitch) error {
	if !k.Active {
		k.ClosePositions = false
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO kill_switches (scope, active, close_positions, reason, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE active = VALUES(active), close_positions = VALUES(close_positions),
  reason = VALUES(reason), updated_by = VALUES(updated_by), updated_at = VALUES(updated_at)`,
		k.Scope, k.Active, k.ClosePositions, nullable(k.Reason), nullable(k.UpdatedBy), time.Now().UTC())
	return err
}

func (s *MySQLStore) Heartbeat(ctx context.Context, instance string, startedAt, now time.Time, pollInterval time.Duration, runners int) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO bot_heartbeats (instance, started_at, last_seen_at, poll_interval_seconds, runners)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE started_at = VALUES(started_at), last_seen_at = VALUES(last_seen_at),
  poll_interval_seconds = VALUES(poll_interval_seconds), runners = VALUES(runners)`,
		instance, startedAt.UTC(), now.UTC(), int(pollInterval.Seconds()), runners)
	return err
}

func (s *MySQLStore) ClosedSummary(ctx context.Context, from, to time.Time) (int, float64, error) {
	var closed int
	var pnl float64
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(realized_pnl), 0) FROM positions WHERE status = 'closed' AND closed_at >= ? AND closed_at < ?`,
		from.UTC(), to.UTC()).Scan(&closed, &pnl)
	return closed, pnl, err
}

// RecordBar keeps what the strategy decided on a bar (one row per bar).
func (s *MySQLStore) RecordBar(ctx context.Context, d BarDecision) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO bar_decisions (deployment_id, bar_time, close, action, holding, detail, result, decided_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE close = VALUES(close), action = VALUES(action), holding = VALUES(holding),
  detail = VALUES(detail), result = VALUES(result), decided_at = VALUES(decided_at)`,
		d.DeploymentID, d.BarTime.UTC(), d.Close, d.Action, string(d.Holding), d.Detail, d.Result, d.DecidedAt.UTC())
	return err
}
