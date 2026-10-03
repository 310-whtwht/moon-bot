// Package backfill implements the `bot backfill` command: download historical
// bars from GMO Coin FX into the bars table and report gaps.
package backfill

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/marketdata"
)

// GMO klines history starts on this trading date.
const historyStart = "2023-10-28"

type options struct {
	symbol      string
	timeframe   market.Timeframe
	priceTypes  []market.PriceType
	from, to    time.Time
	minInterval time.Duration
}

func parseFlags(args []string, now time.Time, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	symbol := fs.String("symbol", "USD_JPY", "GMO symbol, e.g. USD_JPY")
	interval := fs.String("interval", "1h", "timeframe: 1m,5m,15m,30m,1h,4h,8h,12h,1d")
	priceTypes := fs.String("price-types", "BID,ASK", "comma-separated price types")
	from := fs.String("from", historyStart, "start date (YYYY-MM-DD, UTC)")
	to := fs.String("to", "", "end date, exclusive (YYYY-MM-DD, UTC; default: now)")
	minInterval := fs.Duration("min-interval", time.Second, "minimum spacing between API requests")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	opts := options{symbol: *symbol, timeframe: market.Timeframe(*interval), minInterval: *minInterval}
	if _, err := opts.timeframe.Duration(); err != nil {
		return options{}, err
	}
	for _, p := range strings.Split(*priceTypes, ",") {
		pt := market.PriceType(strings.ToUpper(strings.TrimSpace(p)))
		if pt != market.PriceBid && pt != market.PriceAsk {
			return options{}, fmt.Errorf("invalid price type %q", p)
		}
		opts.priceTypes = append(opts.priceTypes, pt)
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

// Run executes the command and writes a summary to out.
func Run(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	opts, err := parseFlags(args, time.Now(), out)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	client := gmofx.New(gmofx.Options{
		PublicURL:   cfg.GMO.PublicURL,
		PublicWSURL: cfg.GMO.PublicWSURL,
		MinInterval: opts.minInterval,
	})

	// Validate the symbol up front: unknown symbols also answer 404 on klines,
	// which would otherwise look like "no data".
	instruments, err := client.Instruments(ctx)
	if err != nil {
		return fmt.Errorf("load instruments: %w", err)
	}
	if !hasSymbol(instruments, opts.symbol) {
		return fmt.Errorf("unknown symbol %q", opts.symbol)
	}

	store := marketdata.NewMySQLBarStore(db)
	bf := &marketdata.Backfiller{Source: client, Store: store}
	results, err := bf.Run(ctx, marketdata.BackfillRequest{
		Symbol: opts.symbol, Timeframe: opts.timeframe, PriceTypes: opts.priceTypes,
		From: opts.from, To: opts.to,
	})
	for _, r := range results {
		fmt.Fprintf(out, "%s %s %s: resumed at %s, stored %d bars\n",
			opts.symbol, opts.timeframe, r.PriceType, r.ResumedAt.Format(time.RFC3339), r.Stored)
	}
	if err != nil {
		return err
	}

	return reportGaps(ctx, store, opts, out)
}

func hasSymbol(instruments []market.Instrument, symbol string) bool {
	for _, in := range instruments {
		if in.Key.Symbol == symbol {
			return true
		}
	}
	return false
}

func reportGaps(ctx context.Context, store marketdata.BarStore, opts options, out io.Writer) error {
	barLen, _ := opts.timeframe.Duration()
	key := market.InstrumentKey{Broker: gmofx.BrokerName, Symbol: opts.symbol}

	for _, pt := range opts.priceTypes {
		times, err := store.OpenTimes(ctx, key, opts.timeframe, pt, opts.from, opts.to)
		if err != nil {
			return err
		}
		gaps := marketdata.FindGaps(times, barLen)
		fmt.Fprintf(out, "%s %s %s: %d bars stored in range, %d gaps (weekends excluded)\n",
			opts.symbol, opts.timeframe, pt, len(times), len(gaps))
		for _, g := range gaps {
			fmt.Fprintf(out, "  gap: %s -> %s (%d bars missing)\n",
				g.After.Format(time.RFC3339), g.Before.Format(time.RFC3339), g.Missing)
		}
	}
	return nil
}
