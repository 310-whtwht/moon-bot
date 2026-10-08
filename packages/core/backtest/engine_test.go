package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scripted emits a fixed signal on given bar indices.
type scripted struct {
	i       int
	signals map[int]strategy.Signal
	seen    []*strategy.Position
}

func (s *scripted) OnBar(_ strategy.Bar, pos *strategy.Position) strategy.Signal {
	defer func() { s.i++ }()
	s.seen = append(s.seen, pos)
	if sig, ok := s.signals[s.i]; ok {
		return sig
	}
	return strategy.Signal{Action: strategy.Hold}
}

func scriptDef(s *scripted) strategy.Definition {
	return strategy.Definition{Type: "scripted", Factory: func(strategy.Params) strategy.Strategy { return s }}
}

var t0 = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

// candle builds a candle with a fixed 0.01 spread.
func candle(i int, o, h, l, c float64) Candle {
	return Candle{
		Time: t0.Add(time.Duration(i) * time.Hour),
		Bid:  OHLC{o, h, l, c},
		Ask:  OHLC{o + 0.01, h + 0.01, l + 0.01, c + 0.01},
	}
}

func cfg(s *scripted) Config {
	return Config{Strategy: scriptDef(s), Units: 1000, InitialBalance: 100000}
}

func TestRun_LongEntryAtAskExitAtBidWithFees(t *testing.T) {
	s := &scripted{signals: map[int]strategy.Signal{
		0: {Action: strategy.EnterLong, StopLoss: 99},
		2: {Action: strategy.Exit},
	}}
	candles := []Candle{
		candle(0, 100, 100.5, 99.5, 100),
		candle(1, 100.2, 101, 100, 100.8),
		candle(2, 100.8, 101.5, 100.5, 101.2),
		candle(3, 101, 101.2, 100.9, 101.1),
	}

	res, err := Run(candles, cfg(s))
	require.NoError(t, err)
	require.Len(t, res.Trades, 1)
	tr := res.Trades[0]

	assert.Equal(t, strategy.Long, tr.Side)
	assert.Equal(t, candles[1].Time, tr.EntryTime, "executed at the next bar")
	assert.InDelta(t, 100.21, tr.EntryPrice, 1e-9, "buy at ASK open")
	assert.InDelta(t, 101.0, tr.ExitPrice, 1e-9, "sell at BID open")
	assert.Equal(t, "signal", tr.ExitReason)

	entryFee := 100.21 * 1000 * DefaultFeeRate
	exitFee := 101.0 * 1000 * DefaultFeeRate
	assert.InDelta(t, entryFee+exitFee, tr.Fees, 1e-9)
	assert.InDelta(t, 790-entryFee-exitFee, tr.PnL, 1e-9)
	assert.InDelta(t, 100000+tr.PnL, res.Metrics.FinalEquity, 1e-9)

	// The strategy saw the position it opened.
	require.NotNil(t, s.seen[1])
	assert.Equal(t, strategy.Long, s.seen[1].Side)
}

func TestRun_ConvertsProfitFeesAndMarginToYen(t *testing.T) {
	// The same trade as above on a pair quoted in dollars, with the dollar
	// at 150 yen on entry and 152 yen on exit.
	script := func() *scripted {
		return &scripted{signals: map[int]strategy.Signal{
			0: {Action: strategy.EnterLong, StopLoss: 99},
			2: {Action: strategy.Exit},
		}}
	}
	candles := []Candle{
		candle(0, 100, 100.5, 99.5, 100),
		candle(1, 100.2, 101, 100, 100.8),
		candle(2, 100.8, 101.5, 100.5, 101.2),
		candle(3, 101, 101.2, 100.9, 101.1),
	}
	for i, rate := range []float64{150, 150, 151, 152} {
		candles[i].QuoteRate = rate
	}
	c := cfg(script())
	c.InitialBalance = 10_000_000 // margin is in yen too: 1000 units at 100 dollars is 15 million yen of exposure

	res, err := Run(candles, c)
	require.NoError(t, err)
	require.Len(t, res.Trades, 1)
	tr := res.Trades[0]

	assert.InDelta(t, 100.21, tr.EntryPrice, 1e-9, "prices stay in the quote currency")
	assert.InDelta(t, 101.0, tr.ExitPrice, 1e-9)
	entryFee := 100.21 * 1000 * DefaultFeeRate * 150
	exitFee := 101.0 * 1000 * DefaultFeeRate * 152
	assert.InDelta(t, entryFee+exitFee, tr.Fees, 1e-6)
	assert.InDelta(t, 790*152-entryFee-exitFee, tr.PnL, 1e-6, "0.79 dollars a unit, worth 152 yen each at the exit")
	assert.InDelta(t, 10_000_000+tr.PnL, res.Metrics.FinalEquity, 1e-6)

	// The margin check counts yen: 1000 units need 100.21*1000*150/25 = 601,260 yen.
	c = cfg(script())
	c.InitialBalance = 600_000
	res, err = Run(candles, c)
	require.NoError(t, err)
	assert.Empty(t, res.Trades)
	assert.Equal(t, 1, res.SkippedEntries)
}

func TestRun_ShortStopHitIntrabar(t *testing.T) {
	s := &scripted{signals: map[int]strategy.Signal{0: {Action: strategy.EnterShort, StopLoss: 101}}}
	candles := []Candle{
		candle(0, 100, 100.5, 99.5, 100),
		candle(1, 100.2, 100.6, 100, 100.4),   // short at BID 100.2, stop = ASK open 100.21 + 1
		candle(2, 100.8, 101.5, 100.5, 101.2), // ASK high 101.51 >= 101.21
	}

	res, err := Run(candles, cfg(s))
	require.NoError(t, err)
	require.Len(t, res.Trades, 1)
	assert.InDelta(t, 100.2, res.Trades[0].EntryPrice, 1e-9)
	assert.InDelta(t, 101.21, res.Trades[0].ExitPrice, 1e-9)
	assert.Equal(t, "stop", res.Trades[0].ExitReason)
	assert.Nil(t, s.seen[2], "strategy sees no position after the stop")
}

func TestRun_GapThroughStopFillsAtOpen(t *testing.T) {
	s := &scripted{signals: map[int]strategy.Signal{0: {Action: strategy.EnterLong, StopLoss: 99}}}
	candles := []Candle{
		candle(0, 100, 100.5, 99.5, 100),
		candle(1, 100, 100.3, 99.8, 100.1), // long at 100.01, stop 99.0
		candle(2, 98, 98.5, 97.5, 98.2),    // opens below the stop
	}

	res, err := Run(candles, cfg(s))
	require.NoError(t, err)
	require.Len(t, res.Trades, 1)
	assert.InDelta(t, 98.0, res.Trades[0].ExitPrice, 1e-9)
	assert.Equal(t, "stop", res.Trades[0].ExitReason)
}

func TestRun_ReverseAndCloseAtEnd(t *testing.T) {
	s := &scripted{signals: map[int]strategy.Signal{
		0: {Action: strategy.EnterLong, StopLoss: 90},
		1: {Action: strategy.EnterShort, StopLoss: 110},
		3: {Action: strategy.EnterLong, StopLoss: 90}, // last bar: never executed
	}}
	candles := []Candle{
		candle(0, 100, 100.5, 99.5, 100),
		candle(1, 100, 100.5, 99.5, 100),
		candle(2, 101, 101.5, 100.5, 101),
		candle(3, 102, 102.5, 101.5, 102),
	}

	res, err := Run(candles, cfg(s))
	require.NoError(t, err)
	require.Len(t, res.Trades, 2)
	assert.Equal(t, strategy.Long, res.Trades[0].Side)
	assert.Equal(t, "signal", res.Trades[0].ExitReason)
	assert.Equal(t, strategy.Short, res.Trades[1].Side)
	assert.Equal(t, "end", res.Trades[1].ExitReason)
	assert.InDelta(t, 102.01, res.Trades[1].ExitPrice, 1e-9, "short closed at last ASK close")
	assert.InDelta(t, res.Metrics.FinalEquity, res.Equity[len(res.Equity)-1].Equity, 1e-9)
}

func TestRun_SkipsEntryWithoutMargin(t *testing.T) {
	s := &scripted{signals: map[int]strategy.Signal{0: {Action: strategy.EnterLong, StopLoss: 99}}}
	c := cfg(s)
	c.InitialBalance = 1000 // 1000 units * 100 / 25 = 4000 required
	res, err := Run([]Candle{candle(0, 100, 101, 99, 100), candle(1, 100, 101, 99, 100)}, c)
	require.NoError(t, err)
	assert.Empty(t, res.Trades)
	assert.Equal(t, 1, res.SkippedEntries)
}

func TestRun_RejectsBadInput(t *testing.T) {
	s := &scripted{}
	_, err := Run(nil, cfg(s))
	assert.Error(t, err)

	c := cfg(s)
	c.Units = 0
	_, err = Run([]Candle{candle(0, 1, 1, 1, 1)}, c)
	assert.Error(t, err)

	_, err = Run([]Candle{candle(1, 1, 1, 1, 1), candle(0, 1, 1, 1, 1)}, cfg(s))
	assert.Error(t, err, "unsorted candles")
}

func TestRun_WithRegisteredStrategy(t *testing.T) {
	def, err := strategy.Lookup("ema_cross")
	require.NoError(t, err)

	var candles []Candle
	for i := 0; i < 300; i++ {
		p := 150 + 3*math.Sin(float64(i)/15)
		candles = append(candles, candle(i, p, p+0.05, p-0.05, p))
	}
	res, err := Run(candles, Config{Strategy: def, Params: strategy.Params{"fast_period": 5, "slow_period": 20},
		Units: 100, InitialBalance: 30000})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Trades)
	assert.Equal(t, 5.0, res.Params["fast_period"])
	assert.Len(t, res.Equity, 300)
}

func TestMetrics(t *testing.T) {
	eq := []EquityPoint{
		{Time: t0, Equity: 100},
		{Time: t0.AddDate(0, 0, 1), Equity: 120},
		{Time: t0.AddDate(0, 0, 2), Equity: 90}, // 25% below the 120 peak
		{Time: t0.AddDate(0, 0, 3), Equity: 130},
	}
	assert.InDelta(t, 0.25, maxDrawdown(100, eq), 1e-12)

	res := &Result{From: t0, To: t0.Add(time.Duration(365.25*24) * time.Hour), Bars: 4, Equity: []EquityPoint{
		{Time: t0, Equity: 100}, {Time: t0.AddDate(1, 0, 0), Equity: 200},
	}, Trades: []Trade{{PnL: 150, Fees: 1}, {PnL: -50, Fees: 1}}}
	m := computeMetrics(res, 100, 2)
	assert.InDelta(t, 1.0, m.TotalReturn, 1e-12)
	assert.InDelta(t, 1.0, m.CAGR, 1e-9, "doubling in one year")
	assert.Equal(t, 2, m.NumTrades)
	assert.InDelta(t, 0.5, m.WinRate, 1e-12)
	assert.InDelta(t, 3.0, m.ProfitFactor, 1e-12)
	assert.InDelta(t, 50.0, m.AvgTrade, 1e-12)
	assert.InDelta(t, 2.0, m.TotalFees, 1e-12)
	assert.InDelta(t, 0.5, m.Exposure, 1e-12)

	flat := []EquityPoint{{Time: t0, Equity: 1}, {Time: t0.AddDate(0, 0, 1), Equity: 1}, {Time: t0.AddDate(0, 0, 2), Equity: 1}}
	assert.Equal(t, 0.0, sharpe(flat))
}
