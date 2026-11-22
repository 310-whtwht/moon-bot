// Package driftcmd implements `bot drift`: compare a deployment's closed
// trades with a backtest of the same strategy version over the same period.
//
// Paper (and later live) results are only evidence when they look like the
// backtest: the same trades, at about the same prices. This shows where they
// differ — trades only one side took, and how much worse each fill was.
//
//	bot drift -deployment <id> [-from YYYY-MM-DD] [-to YYYY-MM-DD]
package driftcmd

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/trader"
	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
	"github.com/moomoo-trading/core/series"
	"github.com/moomoo-trading/core/strategy"
)

// warmupBars is how many bars before the first trade the backtest starts, so
// indicators are primed like the bot primes them.
const warmupBars = 500

// Run executes the command.
func Run(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("drift", flag.ContinueOnError)
	fs.SetOutput(out)
	depID := fs.String("deployment", "", "deployment ID (required)")
	from := fs.String("from", "", "only trades opened on or after this date (YYYY-MM-DD, UTC)")
	to := fs.String("to", "", "only trades opened before this date (YYYY-MM-DD, UTC)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *depID == "" {
		return errors.New("usage: bot drift -deployment <id> [-from YYYY-MM-DD] [-to YYYY-MM-DD]")
	}
	var fromT, toT time.Time
	var err error
	if *from != "" {
		if fromT, err = time.Parse("2006-01-02", *from); err != nil {
			return fmt.Errorf("invalid -from: %w", err)
		}
	}
	if *to != "" {
		if toT, err = time.Parse("2006-01-02", *to); err != nil {
			return fmt.Errorf("invalid -to: %w", err)
		}
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()
	store := &trader.MySQLStore{DB: db}

	dep, err := findDeployment(ctx, store, *depID)
	if err != nil {
		return err
	}
	tfDur, err := dep.Timeframe.Duration()
	if err != nil {
		return err
	}
	trades, err := LoadTrades(ctx, db, dep.ID, fromT, toT)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s (%s, %s %s, %s units)\n", dep.Name, dep.Broker, dep.Symbol, dep.Timeframe, dep.Units.String())
	if len(trades) == 0 {
		fmt.Fprintln(out, "no closed trades in the period")
		return nil
	}

	data := &series.Cache{Load: series.MySQLStore{DB: db}.Load}
	bars := marketdata.NewMySQLBarStore(db)
	key := market.InstrumentKey{Broker: gmofx.BrokerName, Symbol: dep.Symbol}

	// Each version is replayed on its own: a position is always run by the
	// version that opened it.
	byVersion := map[string][]Trade{}
	var versions []string
	for _, t := range trades {
		if _, ok := byVersion[t.VersionID]; !ok {
			versions = append(versions, t.VersionID)
		}
		byVersion[t.VersionID] = append(byVersion[t.VersionID], t)
	}
	var all Report
	for _, vid := range versions {
		live := byVersion[vid]
		v, err := store.Version(ctx, vid)
		if err != nil {
			return fmt.Errorf("version %s: %w", vid, err)
		}
		def, err := strategy.Define(v.Type, v.Script)
		if err != nil {
			return fmt.Errorf("version %s: %w", vid, err)
		}
		start, end := live[0].OpenedAt, live[len(live)-1].ClosedAt
		for _, t := range live {
			if t.ClosedAt.After(end) {
				end = t.ClosedAt
			}
		}
		candles, err := backtest.LoadCandles(ctx, bars, key, dep.Timeframe,
			start.Add(-warmupBars*tfDur), end.Add(2*tfDur))
		if err != nil {
			return fmt.Errorf("version %s: candles: %w", vid, err)
		}
		res, err := backtest.Run(candles, backtest.Config{
			Strategy: def.WithData(data), Params: v.Params, Units: dep.Units.InexactFloat64(),
			// Enough money that the backtest never skips an entry the bot took.
			InitialBalance: 1e12, FeeRate: backtest.DefaultFeeRate, Leverage: backtest.DefaultLeverage,
		})
		if err != nil {
			return fmt.Errorf("version %s: backtest: %w", vid, err)
		}
		// Backtest trades that started inside the period the bot was trading.
		// One entered on the bar of the bot's last close is left out: the bot
		// may hold that position now, and open positions are not compared.
		var bt []backtest.Trade
		for _, t := range res.Trades {
			if !t.EntryTime.Before(start.Add(-tfDur)) && t.EntryTime.Before(end.Add(-tfDur/2)) {
				bt = append(bt, t)
			}
		}
		r := Compare(live, bt, tfDur, dep.Units.InexactFloat64())
		fmt.Fprintf(out, "\nversion %s (%s): %s .. %s\n", vid, v.Type,
			start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
		r.Print(out, true)
		all.Add(r)
	}
	if len(versions) > 1 {
		fmt.Fprintln(out, "\nall versions")
		all.Print(out, false)
	}
	return nil
}

func findDeployment(ctx context.Context, store *trader.MySQLStore, id string) (trader.Deployment, error) {
	deps, err := store.Deployments(ctx)
	if err != nil {
		return trader.Deployment{}, err
	}
	for _, d := range deps {
		if d.ID == id {
			return d, nil
		}
	}
	return trader.Deployment{}, fmt.Errorf("deployment %s not found", id)
}

// LoadTrades reads a deployment's closed positions, oldest first.
func LoadTrades(ctx context.Context, db *sql.DB, deploymentID string, from, to time.Time) ([]Trade, error) {
	q := `SELECT side, quantity, open_price, expected_open_price, close_price, expected_close_price,
  realized_pnl, COALESCE(exit_reason, ''), COALESCE(strategy_version_id, ''), opened_at, closed_at
FROM positions WHERE deployment_id = ? AND status = 'closed'`
	args := []any{deploymentID}
	if !from.IsZero() {
		q += ` AND opened_at >= ?`
		args = append(args, from.UTC())
	}
	if !to.IsZero() {
		q += ` AND opened_at < ?`
		args = append(args, to.UTC())
	}
	rows, err := db.QueryContext(ctx, q+` ORDER BY opened_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trade
	for rows.Next() {
		var t Trade
		var side string
		var expOpen, expClose, closePrice, pnl sql.NullFloat64
		if err := rows.Scan(&side, &t.Units, &t.OpenPrice, &expOpen, &closePrice, &expClose,
			&pnl, &t.ExitReason, &t.VersionID, &t.OpenedAt, &t.ClosedAt); err != nil {
			return nil, err
		}
		t.Long = side == "buy"
		t.ExpectedOpen, t.ClosePrice, t.ExpectedClose, t.PnL = expOpen.Float64, closePrice.Float64, expClose.Float64, pnl.Float64
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].OpenedAt.Before(out[j].OpenedAt) })
	return out, rows.Err()
}
