// Package cotcmd keeps the CFTC positioning of speculators (COT) up to date
// for strategy scripts (data("cot_jpy_pct") and so on).
//
//	bot cot   download every report once and store the series
//
// The running bot also does this by itself (Loop), so the command is only
// needed to fill the table without waiting for it.
package cotcmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/core/series"
)

// Interval is how often the running bot refreshes the reports. They come out
// once a week, so this only bounds how late a new one is picked up.
const Interval = 6 * time.Hour

var client = &http.Client{Timeout: 60 * time.Second}

// Run executes the command.
func Run(ctx context.Context, cfg *config.Config, _ []string, out io.Writer) error {
	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()
	weeks, err := series.SyncCOT(ctx, client, series.COTURL, series.MySQLStore{DB: db})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, summary(weeks))
	return nil
}

// Loop refreshes the reports now and then every Interval until ctx ends.
func Loop(ctx context.Context, db *sql.DB, logf func(format string, args ...any)) {
	for {
		weeks, err := series.SyncCOT(ctx, client, series.COTURL, series.MySQLStore{DB: db})
		if err != nil {
			logf("cot: %v (retrying in %s)", err, Interval)
		} else {
			logf("cot: %s", summary(weeks))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(Interval):
		}
	}
}

func summary(weeks map[string]int) string {
	names := make([]string, 0, len(weeks))
	for cur := range weeks {
		names = append(names, cur)
	}
	sort.Strings(names)
	s := "stored"
	for _, cur := range names {
		s += fmt.Sprintf(" %s=%d weeks", cur, weeks[cur])
	}
	return s
}
