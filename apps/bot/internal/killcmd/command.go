// Package killcmd implements `bot kill`: show or change the kill switches
// without going through the web UI.
//
//	bot kill status
//	bot kill on  [-scope global|paper|gmo] [-close] [-reason "..."]
//	bot kill off [-scope global|paper|gmo]
package killcmd

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/trader"
)

// Run executes the command.
func Run(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bot kill status|on|off [-scope global] [-close] [-reason text]")
	}
	action := args[0]

	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	fs.SetOutput(out)
	scope := fs.String("scope", trader.GlobalScope, "global, or a broker name (paper, gmo)")
	closeAll := fs.Bool("close", false, "also close open positions (with on)")
	reason := fs.String("reason", "", "why (shown in the UI and Slack)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer db.Close()
	store := &trader.MySQLStore{DB: db}

	switch action {
	case "status":
	case "on":
		err = store.SetKillSwitch(ctx, trader.KillSwitch{Scope: *scope, Active: true, ClosePositions: *closeAll, Reason: *reason, UpdatedBy: "cli"})
	case "off":
		err = store.SetKillSwitch(ctx, trader.KillSwitch{Scope: *scope, Active: false, Reason: *reason, UpdatedBy: "cli"})
	default:
		return fmt.Errorf("unknown action %q (want status, on or off)", action)
	}
	if err != nil {
		return err
	}

	switches, err := store.KillSwitches(ctx)
	if err != nil {
		return err
	}
	for _, k := range switches {
		state := "off"
		if k.Active {
			state = "ON"
			if k.ClosePositions {
				state += " (close positions)"
			}
		}
		fmt.Fprintf(out, "%-8s %-22s %s\n", k.Scope, state, k.Reason)
	}
	return nil
}
