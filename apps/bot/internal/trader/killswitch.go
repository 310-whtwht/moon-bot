package trader

import (
	"context"
	"fmt"
	"time"
)

// GlobalScope is the kill switch that covers every broker.
const GlobalScope = "global"

// KillSwitch is one row of kill_switches. Scope is GlobalScope or a broker name.
type KillSwitch struct {
	Scope          string
	Active         bool
	ClosePositions bool
	Reason         string
	UpdatedBy      string
	UpdatedAt      time.Time
}

// covers reports whether an active switch applies to the broker.
func (k KillSwitch) covers(brokerName string) bool {
	return k.Active && (k.Scope == GlobalScope || k.Scope == brokerName)
}

// KillStore reads the kill switches.
type KillStore interface {
	KillSwitches(ctx context.Context) ([]KillSwitch, error)
}

// SwitchGuard blocks new entries while a kill switch covers the broker.
// EnvActive is the switch set through the bot's environment (KILL_SWITCH=true),
// which works even when the database is unreachable.
type SwitchGuard struct {
	Store     KillStore
	EnvActive bool
}

func (g SwitchGuard) EntriesAllowed(ctx context.Context, brokerName string) (bool, string) {
	if g.EnvActive {
		return false, "kill switch (KILL_SWITCH environment variable)"
	}
	switches, err := g.Store.KillSwitches(ctx)
	if err != nil {
		// Fail closed: not knowing the switch state must not allow trading.
		return false, fmt.Sprintf("kill switch state unknown: %v", err)
	}
	for _, k := range switches {
		if k.covers(brokerName) {
			reason := k.Reason
			if reason == "" {
				reason = "no reason given"
			}
			return false, fmt.Sprintf("kill switch %s (%s)", k.Scope, reason)
		}
	}
	return true, ""
}
