// Package risk decides whether a new position may be opened. It is a pure
// check: the caller supplies the current price, account state and realised
// losses, so the same rules run in tests, paper trading and live trading.
//
// Limits apply at two levels: per account (one broker account) and globally
// (all accounts together, in JPY).
package risk

import (
	"fmt"
	"time"
)

// Limits are the hard caps for one level (an account, or everything). A zero
// value disables that particular limit.
type Limits struct {
	MaxUnitsPerPosition float64 `json:"max_units_per_position"`
	MaxOpenPositions    int     `json:"max_open_positions"`
	MaxDailyLossJPY     float64 `json:"max_daily_loss_jpy"`
	MaxWeeklyLossJPY    float64 `json:"max_weekly_loss_jpy"`
}

// Exposure is the current state at one level.
type Exposure struct {
	OpenPositions int
	// DailyPnLJPY / WeeklyPnLJPY are realised P&L since the start of the trading
	// day / week (negative = loss).
	DailyPnLJPY  float64
	WeeklyPnLJPY float64
}

// OpenRequest describes the position about to be opened.
type OpenRequest struct {
	Symbol string
	Units  float64
	// Price is the current executable price (ASK for buys, BID for sells), in JPY.
	Price float64
	// Leverage and AvailableMarginJPY come from the broker account.
	Leverage           float64
	AvailableMarginJPY float64
	// MarginBuffer is the share of available margin an order may use (e.g. 0.5).
	MarginBuffer float64
}

// Rejection explains which rule refused the order.
type Rejection struct {
	Level  string // "account" or "global"
	Rule   string
	Detail string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("risk: %s %s: %s", r.Level, r.Rule, r.Detail)
}

// CheckOpen returns a *Rejection if opening the position would break a limit.
func CheckOpen(req OpenRequest, account, global Limits, accountExp, globalExp Exposure) error {
	if req.Units <= 0 || req.Price <= 0 {
		return &Rejection{Level: "account", Rule: "invalid_order", Detail: "units and price must be positive"}
	}

	if req.Leverage > 0 {
		required := req.Units * req.Price / req.Leverage
		buffer := req.MarginBuffer
		if buffer <= 0 || buffer > 1 {
			buffer = 1
		}
		if usable := req.AvailableMarginJPY * buffer; required > usable {
			return &Rejection{Level: "account", Rule: "margin",
				Detail: fmt.Sprintf("need %.0f JPY, usable %.0f JPY (%.0f%% of available)", required, usable, buffer*100)}
		}
	}

	levels := []struct {
		name   string
		limits Limits
		exp    Exposure
	}{{"account", account, accountExp}, {"global", global, globalExp}}

	for _, lv := range levels {
		l, e := lv.limits, lv.exp
		if l.MaxUnitsPerPosition > 0 && req.Units > l.MaxUnitsPerPosition {
			return &Rejection{Level: lv.name, Rule: "max_units",
				Detail: fmt.Sprintf("%.0f units exceeds %.0f", req.Units, l.MaxUnitsPerPosition)}
		}
		if l.MaxOpenPositions > 0 && e.OpenPositions >= l.MaxOpenPositions {
			return &Rejection{Level: lv.name, Rule: "max_open_positions",
				Detail: fmt.Sprintf("%d open, limit %d", e.OpenPositions, l.MaxOpenPositions)}
		}
		if l.MaxDailyLossJPY > 0 && -e.DailyPnLJPY >= l.MaxDailyLossJPY {
			return &Rejection{Level: lv.name, Rule: "daily_loss",
				Detail: fmt.Sprintf("lost %.0f JPY today, limit %.0f", -e.DailyPnLJPY, l.MaxDailyLossJPY)}
		}
		if l.MaxWeeklyLossJPY > 0 && -e.WeeklyPnLJPY >= l.MaxWeeklyLossJPY {
			return &Rejection{Level: lv.name, Rule: "weekly_loss",
				Detail: fmt.Sprintf("lost %.0f JPY this week, limit %.0f", -e.WeeklyPnLJPY, l.MaxWeeklyLossJPY)}
		}
	}
	return nil
}

// tradingDayStartHourUTC is 06:00 JST, where the FX trading day rolls over.
const tradingDayStartHourUTC = 21

// TradingDayStart returns the start (UTC) of the trading day containing t.
func TradingDayStart(t time.Time) time.Time {
	t = t.UTC()
	start := time.Date(t.Year(), t.Month(), t.Day(), tradingDayStartHourUTC, 0, 0, 0, time.UTC)
	if t.Before(start) {
		start = start.AddDate(0, 0, -1)
	}
	return start
}

// TradingWeekStart returns the start (UTC) of the trading week containing t:
// the trading day that begins on Sunday 21:00 UTC (Monday 06:00 JST).
func TradingWeekStart(t time.Time) time.Time {
	start := TradingDayStart(t)
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}
	return start
}
