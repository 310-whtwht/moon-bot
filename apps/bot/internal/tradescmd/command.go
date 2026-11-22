// Package tradescmd implements `bot trades`: write closed positions as CSV,
// one row per trade, for review and tax records.
//
//	bot trades [-from YYYY-MM-DD] [-to YYYY-MM-DD] [-broker paper|gmo] [-o file.csv]
//
// A value that was not recorded is left blank, never estimated.
package tradescmd

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
)

// Header is the CSV header.
var Header = []string{"trade_id", "deployment", "strategy_version", "mode", "account", "market", "side",
	"entry_time", "entry_price", "expected_entry_price", "entry_slippage", "size", "initial_stop", "risk_jpy",
	"exit_time", "exit_price", "expected_exit_price", "exit_slippage", "fees", "pnl", "r_multiple", "exit_reason"}

// Row is one closed position.
type Row struct {
	ID, Deployment, Version, Broker, Account, Symbol, Side string
	OpenedAt, ClosedAt                                     time.Time
	Units, OpenPrice, ClosePrice, Fees, PnL                float64
	ExpectedOpen, ExpectedClose, InitialStop               sql.NullFloat64
	ExitReason                                             string
}

// Run executes the command.
func Run(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("trades", flag.ContinueOnError)
	fs.SetOutput(out)
	from := fs.String("from", "", "trades closed on or after this date (YYYY-MM-DD, UTC)")
	to := fs.String("to", "", "trades closed before this date (YYYY-MM-DD, UTC)")
	brokerName := fs.String("broker", "", "only this broker (paper, gmo)")
	file := fs.String("o", "", "write to this file instead of standard output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	q := `SELECT p.id, COALESCE(d.name, ''), COALESCE(p.strategy_version_id, ''), p.broker, p.account_id, p.symbol, p.side,
  p.opened_at, p.closed_at, p.quantity, p.open_price, p.close_price, p.fees, p.realized_pnl,
  p.expected_open_price, p.expected_close_price, p.initial_stop_price, COALESCE(p.exit_reason, '')
FROM positions p LEFT JOIN deployments d ON d.id = p.deployment_id
WHERE p.status = 'closed'`
	var qargs []any
	for _, f := range []struct {
		raw, cond string
	}{{*from, ` AND p.closed_at >= ?`}, {*to, ` AND p.closed_at < ?`}} {
		if f.raw == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", f.raw)
		if err != nil {
			return fmt.Errorf("invalid date %q: %w", f.raw, err)
		}
		q += f.cond
		qargs = append(qargs, t.UTC())
	}
	if *brokerName != "" {
		q += ` AND p.broker = ?`
		qargs = append(qargs, *brokerName)
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, q+` ORDER BY p.closed_at`, qargs...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var list []Row
	for rows.Next() {
		var r Row
		var closePrice, pnl sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Deployment, &r.Version, &r.Broker, &r.Account, &r.Symbol, &r.Side,
			&r.OpenedAt, &r.ClosedAt, &r.Units, &r.OpenPrice, &closePrice, &r.Fees, &pnl,
			&r.ExpectedOpen, &r.ExpectedClose, &r.InitialStop, &r.ExitReason); err != nil {
			return err
		}
		r.ClosePrice, r.PnL = closePrice.Float64, pnl.Float64
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	w := out
	if *file != "" {
		f, err := os.Create(*file)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	return Write(w, list)
}

// Write writes rows as CSV.
func Write(w io.Writer, rows []Row) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(Header); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write(r.record()); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (r Row) record() []string {
	buy := r.Side == "buy"
	entrySlip, exitSlip := "", ""
	if r.ExpectedOpen.Valid {
		entrySlip = fixed(worse(buy, r.OpenPrice, r.ExpectedOpen.Float64), 6)
	}
	if r.ExpectedClose.Valid {
		exitSlip = fixed(worse(!buy, r.ClosePrice, r.ExpectedClose.Float64), 6)
	}
	// Risk and R are in yen, so only for yen-quoted pairs; other pairs would
	// need the conversion rate at the time, which is not stored.
	risk, rMultiple := "", ""
	if r.InitialStop.Valid && strings.HasSuffix(r.Symbol, "_JPY") {
		if v := abs(r.OpenPrice-r.InitialStop.Float64) * r.Units; v > 0 {
			risk = fixed(v, 2)
			rMultiple = strconv.FormatFloat(r.PnL/v, 'f', 2, 64)
		}
	}
	return []string{r.ID, r.Deployment, r.Version, r.Broker, r.Account, r.Symbol, r.Side,
		r.OpenedAt.UTC().Format(time.RFC3339), num(r.OpenPrice), nullNum(r.ExpectedOpen), entrySlip, num(r.Units),
		nullNum(r.InitialStop), risk,
		r.ClosedAt.UTC().Format(time.RFC3339), num(r.ClosePrice), nullNum(r.ExpectedClose), exitSlip,
		num(r.Fees), num(r.PnL), rMultiple, r.ExitReason}
}

// worse is how much worse price is than ref for a buy or a sell (positive = worse).
func worse(buy bool, price, ref float64) float64 {
	if buy {
		return price - ref
	}
	return ref - price
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// fixed rounds away float noise (0.0020000000000095 -> 0.002).
func fixed(v float64, places int) string {
	return strconv.FormatFloat(v, 'f', places, 64)
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func nullNum(v sql.NullFloat64) string {
	if !v.Valid {
		return ""
	}
	return num(v.Float64)
}
