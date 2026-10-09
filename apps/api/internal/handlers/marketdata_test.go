package handlers

import (
	"testing"

	"github.com/moomoo-trading/api/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestMarkUsable(t *testing.T) {
	coverage := []database.Coverage{
		{Symbol: "USD_JPY", Timeframe: "1h", BidBars: 100, AskBars: 100},
		{Symbol: "USD_JPY", Timeframe: "15m", BidBars: 400, AskBars: 120}, // ASK still downloading
		{Symbol: "EUR_USD", Timeframe: "1h", BidBars: 100, AskBars: 100},  // USD_JPY 1h is there
		{Symbol: "EUR_USD", Timeframe: "4h", BidBars: 25, AskBars: 25},    // USD_JPY 4h is not
		{Symbol: "EUR_GBP", Timeframe: "1h", BidBars: 100, AskBars: 100},  // GBP_JPY 1h is not
	}
	markUsable(coverage)

	assert.True(t, coverage[0].Usable)
	assert.False(t, coverage[1].Usable)
	assert.Contains(t, coverage[1].Missing, "BID と ASK")
	assert.True(t, coverage[2].Usable, "its yen pair has the same timeframe")
	assert.False(t, coverage[3].Usable)
	assert.Contains(t, coverage[3].Missing, "USD_JPY")
	assert.False(t, coverage[4].Usable)
	assert.Contains(t, coverage[4].Missing, "GBP_JPY")
}
