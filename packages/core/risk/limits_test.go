package risk

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func req() OpenRequest {
	return OpenRequest{Symbol: "USD_JPY", Units: 100, Price: 150, Leverage: 25, AvailableMarginJPY: 30000, MarginBuffer: 0.5}
}

func rule(t *testing.T, err error) (level, name string) {
	t.Helper()
	var r *Rejection
	require.True(t, errors.As(err, &r), "want *Rejection, got %v", err)
	return r.Level, r.Rule
}

func TestCheckOpen_Allows(t *testing.T) {
	account := Limits{MaxUnitsPerPosition: 100, MaxOpenPositions: 1, MaxDailyLossJPY: 500, MaxWeeklyLossJPY: 1500}
	global := Limits{MaxOpenPositions: 3, MaxDailyLossJPY: 1000}
	assert.NoError(t, CheckOpen(req(), account, global, Exposure{DailyPnLJPY: -499}, Exposure{OpenPositions: 2, DailyPnLJPY: 200}))

	// Zero limits disable the checks.
	assert.NoError(t, CheckOpen(req(), Limits{}, Limits{}, Exposure{OpenPositions: 9, DailyPnLJPY: -1e9}, Exposure{}))
}

func TestCheckOpen_Rejections(t *testing.T) {
	account := Limits{MaxUnitsPerPosition: 100, MaxOpenPositions: 1, MaxDailyLossJPY: 500, MaxWeeklyLossJPY: 1500}
	global := Limits{MaxOpenPositions: 3, MaxDailyLossJPY: 1000, MaxWeeklyLossJPY: 3000}

	cases := []struct {
		name                string
		mutate              func(*OpenRequest)
		accountExp, global  Exposure
		wantLevel, wantRule string
	}{
		{"invalid", func(r *OpenRequest) { r.Units = 0 }, Exposure{}, Exposure{}, "account", "invalid_order"},
		// 100 * 150 / 25 = 600 needed; usable = 1000 * 0.5 = 500
		{"margin", func(r *OpenRequest) { r.AvailableMarginJPY = 1000 }, Exposure{}, Exposure{}, "account", "margin"},
		{"units", func(r *OpenRequest) { r.Units = 101 }, Exposure{}, Exposure{}, "account", "max_units"},
		{"account positions", nil, Exposure{OpenPositions: 1}, Exposure{OpenPositions: 1}, "account", "max_open_positions"},
		{"account daily loss", nil, Exposure{DailyPnLJPY: -500}, Exposure{}, "account", "daily_loss"},
		{"account weekly loss", nil, Exposure{WeeklyPnLJPY: -1500}, Exposure{}, "account", "weekly_loss"},
		{"global positions", nil, Exposure{}, Exposure{OpenPositions: 3}, "global", "max_open_positions"},
		{"global daily loss", nil, Exposure{DailyPnLJPY: -100}, Exposure{DailyPnLJPY: -1000}, "global", "daily_loss"},
		{"global weekly loss", nil, Exposure{}, Exposure{WeeklyPnLJPY: -3000}, "global", "weekly_loss"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := req()
			if tc.mutate != nil {
				tc.mutate(&r)
			}
			level, name := rule(t, CheckOpen(r, account, global, tc.accountExp, tc.global))
			assert.Equal(t, tc.wantLevel, level)
			assert.Equal(t, tc.wantRule, name)
		})
	}
}

func TestCheckOpen_MarginBufferDefaultsToFull(t *testing.T) {
	r := req()
	r.MarginBuffer = 0
	r.AvailableMarginJPY = 600 // exactly what is needed
	assert.NoError(t, CheckOpen(r, Limits{}, Limits{}, Exposure{}, Exposure{}))
}

func TestTradingDayAndWeekStart(t *testing.T) {
	// Wednesday 2026-10-07 10:00 JST = 01:00 UTC → trading day began Tue 21:00 UTC.
	at := time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC), TradingDayStart(at))
	// Week began Sunday 2026-10-04 21:00 UTC (Monday 06:00 JST).
	assert.Equal(t, time.Date(2026, 10, 4, 21, 0, 0, 0, time.UTC), TradingWeekStart(at))

	// Exactly at the rollover a new day starts.
	roll := time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC)
	assert.Equal(t, roll, TradingDayStart(roll))

	// Sunday 22:00 UTC is already the new week.
	sun := time.Date(2026, 10, 4, 22, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 10, 4, 21, 0, 0, 0, time.UTC), TradingWeekStart(sun))
	// Sunday 20:00 UTC still belongs to the previous week.
	before := time.Date(2026, 10, 4, 20, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2026, 9, 27, 21, 0, 0, 0, time.UTC), TradingWeekStart(before))
}
