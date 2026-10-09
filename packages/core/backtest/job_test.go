package backtest

import (
	"testing"
	"time"

	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSpec() JobSpec {
	return JobSpec{
		StrategyType: "ema_cross", StrategyParams: strategy.Params{"fast_period": 8},
		Broker: "gmo", Symbol: "USD_JPY", Timeframe: market.TF1Hour,
		Units: 100, InitialBalance: 30000,
	}
}

func TestJobSpecResolve(t *testing.T) {
	s := validSpec()
	def, err := s.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "ema_cross", def.Type)
	assert.Equal(t, 8.0, s.StrategyParams["fast_period"])
	assert.Equal(t, 26.0, s.StrategyParams["slow_period"], "defaults are filled in")
	assert.Equal(t, "gmo:USD_JPY", s.Key().String())

	cases := map[string]func(*JobSpec){
		"unknown type":  func(s *JobSpec) { s.StrategyType = "nope" },
		"bad param":     func(s *JobSpec) { s.StrategyParams = strategy.Params{"fast_period": 1} },
		"no symbol":     func(s *JobSpec) { s.Symbol = "" },
		"bad timeframe": func(s *JobSpec) { s.Timeframe = "2h" },
		"zero units":    func(s *JobSpec) { s.Units = 0 },
		"zero balance":  func(s *JobSpec) { s.InitialBalance = 0 },
	}
	// Pairs quoted in another currency are accepted: P&L is converted to yen.
	usd := validSpec()
	usd.Symbol = "EUR_USD"
	_, err = usd.Resolve()
	assert.NoError(t, err)

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validSpec()
			mutate(&s)
			_, err := s.Resolve()
			assert.Error(t, err)
		})
	}
}

func TestDownsample(t *testing.T) {
	var pts []EquityPoint
	for i := 0; i < 1000; i++ {
		pts = append(pts, EquityPoint{Time: t0.Add(time.Duration(i) * time.Hour), Equity: float64(i)})
	}

	out := Downsample(pts, 100)
	require.Len(t, out, 100)
	assert.Equal(t, pts[0], out[0])
	assert.Equal(t, pts[999], out[99], "last point is kept")
	for i := 1; i < len(out); i++ {
		assert.True(t, out[i].Time.After(out[i-1].Time))
	}

	assert.Len(t, Downsample(pts[:10], 100), 10, "short series unchanged")
}
