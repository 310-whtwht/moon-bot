// Package backtestcmd implements `bot backtest`: run a strategy over stored
// bars, optionally across a parameter grid, and print the metrics.
package backtestcmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
	"github.com/moomoo-trading/core/strategy"
)

// multiFlag collects repeated -param / -sweep flags.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, " ") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

type options struct {
	strategy  string
	params    strategy.Params
	sweeps    map[string][]float64
	symbol    string
	timeframe market.Timeframe
	from, to  time.Time
	units     float64
	balance   float64
	feeRate   float64
	jsonOut   bool
}

func parseFlags(args []string, now time.Time, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("backtest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	strat := fs.String("strategy", "ema_cross", "strategy type")
	var params, sweeps multiFlag
	fs.Var(&params, "param", "strategy parameter name=value (repeatable)")
	fs.Var(&sweeps, "sweep", "parameter grid name=v1,v2,... (repeatable)")
	symbol := fs.String("symbol", "USD_JPY", "GMO symbol (JPY-quoted)")
	interval := fs.String("interval", "1h", "timeframe")
	from := fs.String("from", "2023-10-28", "start date (YYYY-MM-DD, UTC)")
	to := fs.String("to", "", "end date, exclusive (default: now)")
	units := fs.Float64("units", 100, "position size in currency units")
	balance := fs.Float64("balance", 30000, "initial balance (JPY)")
	feeRate := fs.Float64("fee-rate", backtest.DefaultFeeRate, "fee rate per fill on notional")
	jsonOut := fs.Bool("json", false, "print full results as JSON")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	opts := options{
		strategy: *strat, params: strategy.Params{}, sweeps: map[string][]float64{},
		symbol: *symbol, timeframe: market.Timeframe(*interval),
		units: *units, balance: *balance, feeRate: *feeRate, jsonOut: *jsonOut,
	}
	if _, err := opts.timeframe.Duration(); err != nil {
		return options{}, err
	}
	for _, p := range params {
		name, value, ok := strings.Cut(p, "=")
		v, err := strconv.ParseFloat(value, 64)
		if !ok || err != nil {
			return options{}, fmt.Errorf("invalid -param %q (want name=number)", p)
		}
		opts.params[name] = v
	}
	for _, s := range sweeps {
		name, values, ok := strings.Cut(s, "=")
		if !ok {
			return options{}, fmt.Errorf("invalid -sweep %q (want name=v1,v2)", s)
		}
		for _, raw := range strings.Split(values, ",") {
			v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if err != nil {
				return options{}, fmt.Errorf("invalid -sweep value %q: %w", raw, err)
			}
			opts.sweeps[name] = append(opts.sweeps[name], v)
		}
	}

	var err error
	if opts.from, err = time.Parse("2006-01-02", *from); err != nil {
		return options{}, fmt.Errorf("invalid -from: %w", err)
	}
	if *to == "" {
		opts.to = now.UTC()
	} else if opts.to, err = time.Parse("2006-01-02", *to); err != nil {
		return options{}, fmt.Errorf("invalid -to: %w", err)
	}
	if !opts.from.Before(opts.to) {
		return options{}, fmt.Errorf("-from must be before -to")
	}
	return opts, nil
}

// grid expands base params with every combination of sweep values.
func grid(base strategy.Params, sweeps map[string][]float64) []strategy.Params {
	combos := []strategy.Params{copyParams(base)}
	names := make([]string, 0, len(sweeps))
	for name := range sweeps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var next []strategy.Params
		for _, c := range combos {
			for _, v := range sweeps[name] {
				p := copyParams(c)
				p[name] = v
				next = append(next, p)
			}
		}
		combos = next
	}
	return combos
}

func copyParams(p strategy.Params) strategy.Params {
	out := make(strategy.Params, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// Run executes the command.
func Run(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	opts, err := parseFlags(args, time.Now(), out)
	if err != nil {
		return err
	}
	def, err := strategy.Lookup(opts.strategy)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()

	key := market.InstrumentKey{Broker: gmofx.BrokerName, Symbol: opts.symbol}
	candles, err := backtest.LoadCandles(ctx, marketdata.NewMySQLBarStore(db), key, opts.timeframe, opts.from, opts.to)
	if err != nil {
		return err
	}

	var results []*backtest.Result
	for _, params := range grid(opts.params, opts.sweeps) {
		res, err := backtest.Run(candles, backtest.Config{
			Strategy: def, Params: params, Units: opts.units,
			InitialBalance: opts.balance, FeeRate: opts.feeRate,
		})
		if err != nil {
			// Invalid combinations (e.g. fast >= slow) are reported and skipped.
			fmt.Fprintf(out, "skip %v: %v\n", params, err)
			continue
		}
		results = append(results, res)
	}

	if opts.jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	printTable(out, def, opts, candles, results)
	return nil
}

func printTable(out io.Writer, def strategy.Definition, opts options, candles []backtest.Candle, results []*backtest.Result) {
	fmt.Fprintf(out, "%s %s %s  %s .. %s  (%d bars, %.0f units, balance %.0f JPY, fee %.4f%%)\n\n",
		def.Type, opts.symbol, opts.timeframe,
		candles[0].Time.Format("2006-01-02"), candles[len(candles)-1].Time.Format("2006-01-02"),
		len(candles), opts.units, opts.balance, opts.feeRate*100)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "params\ttrades\twin%\tPF\tnet pips\tnet JPY\treturn%\tCAGR%\tmaxDD%\tSharpe\tSQN\tfees JPY\t")
	for _, r := range results {
		m := r.Metrics
		fmt.Fprintf(w, "%s\t%d\t%.1f\t%.2f\t%.1f\t%.0f\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t%.0f\t\n",
			formatParams(def, r.Params), m.NumTrades, m.WinRate*100, m.ProfitFactor, netPips(m.NetProfit, opts.units),
			m.NetProfit, m.TotalReturn*100, m.CAGR*100, m.MaxDrawdown*100, m.Sharpe, m.SQN, m.TotalFees)
	}
	w.Flush()
}

// netPips converts JPY profit to pips (0.01 JPY) per unit, independent of size.
func netPips(netJPY, units float64) float64 {
	return netJPY / units / 0.01
}

func formatParams(def strategy.Definition, p strategy.Params) string {
	parts := make([]string, 0, len(def.Params))
	for _, spec := range def.Params {
		parts = append(parts, fmt.Sprintf("%s=%s", spec.Name, strconv.FormatFloat(p[spec.Name], 'f', -1, 64)))
	}
	return strings.Join(parts, " ")
}
