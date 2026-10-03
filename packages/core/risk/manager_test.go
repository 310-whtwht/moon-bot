package risk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRiskManager_RecordPnL_TracksLossAndIgnoresProfit(t *testing.T) {
	cfg := &RiskConfig{}
	rm := NewRiskManager(cfg)

	rm.RecordPnL(-100)
	assert.Equal(t, 100.0, rm.dailyLoss)
	assert.Equal(t, 100.0, rm.weeklyLoss)

	rm.RecordPnL(50)
	assert.Equal(t, 100.0, rm.dailyLoss)
	assert.Equal(t, 100.0, rm.weeklyLoss)
}

func TestRiskManager_resetLossCountersIfNeeded_DailyAndWeeklyReset(t *testing.T) {
	cfg := &RiskConfig{}
	rm := NewRiskManager(cfg)

	base := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC) // Friday
	rm.lastDailyReset = base
	rm.lastWeeklyResetYear, rm.lastWeeklyResetWeek = base.ISOWeek()
	rm.dailyLoss = 100
	rm.weeklyLoss = 500

	// next day, same week
	rm.resetLossCountersIfNeeded(base.AddDate(0, 0, 1))
	assert.Equal(t, 0.0, rm.dailyLoss)
	assert.Equal(t, 500.0, rm.weeklyLoss)

	// next week
	rm.resetLossCountersIfNeeded(base.AddDate(0, 0, 7))
	assert.Equal(t, 0.0, rm.weeklyLoss)
}

func TestRiskManager_CheckLossLimits(t *testing.T) {
	cfg := &RiskConfig{MaxDailyLoss: 1, MaxWeeklyLoss: 3}
	rm := NewRiskManager(cfg)
	rm.dailyLoss = 200  // represents 2% loss on 10,000 balance
	rm.weeklyLoss = 400 // represents 4% loss on 10,000 balance

	accountBalance := 10000.0
	assert.Error(t, rm.checkDailyLossLimit(accountBalance))
	assert.Error(t, rm.checkWeeklyLossLimit(accountBalance))
}
