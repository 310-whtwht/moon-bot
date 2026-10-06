package backtest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moomoo-trading/core/market"
	"github.com/moomoo-trading/core/strategy"
)

// Redis stream and consumer group used to hand backtest jobs from the API to the bot.
const (
	JobStream = "backtest_jobs"
	JobGroup  = "backtest_workers"
	// JobField is the stream entry field holding the backtest ID.
	JobField = "backtest_id"
)

// JobSpec is the snapshot stored in backtests.parameters when a backtest is
// requested. It fully describes the run, so later edits to the strategy
// version do not change a queued or finished backtest.
type JobSpec struct {
	StrategyVersionID string           `json:"strategy_version_id"`
	StrategyType      string           `json:"strategy_type"`
	// StrategyScript is the source for strategy type "script" (empty otherwise).
	StrategyScript string `json:"strategy_script,omitempty"`
	StrategyParams    strategy.Params  `json:"strategy_params"`
	Broker            string           `json:"broker"`
	Symbol            string           `json:"symbol"`
	Timeframe         market.Timeframe `json:"timeframe"`
	Units             float64          `json:"units"`
	InitialBalance    float64          `json:"initial_balance"`
}

// Resolve validates the spec, applies strategy parameter defaults and returns
// the strategy definition to run.
func (s *JobSpec) Resolve() (strategy.Definition, error) {
	def, err := strategy.Define(s.StrategyType, s.StrategyScript)
	if err != nil {
		return strategy.Definition{}, err
	}
	params, err := def.Resolve(s.StrategyParams)
	if err != nil {
		return strategy.Definition{}, err
	}
	s.StrategyParams = params

	if s.Broker == "" || s.Symbol == "" {
		return strategy.Definition{}, errors.New("broker and symbol are required")
	}
	if !strings.HasSuffix(s.Symbol, "_JPY") {
		return strategy.Definition{}, fmt.Errorf("only JPY-quoted symbols are supported (got %s)", s.Symbol)
	}
	if _, err := s.Timeframe.Duration(); err != nil {
		return strategy.Definition{}, err
	}
	if s.Units <= 0 || s.InitialBalance <= 0 {
		return strategy.Definition{}, errors.New("units and initial_balance must be positive")
	}
	return def, nil
}

// Key returns the instrument the job runs on.
func (s JobSpec) Key() market.InstrumentKey {
	return market.InstrumentKey{Broker: s.Broker, Symbol: s.Symbol}
}

// Downsample keeps at most n equity points (always including the last one),
// so results stored in the database stay small.
func Downsample(points []EquityPoint, n int) []EquityPoint {
	if n <= 0 || len(points) <= n {
		return points
	}
	out := make([]EquityPoint, 0, n)
	step := float64(len(points)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		out = append(out, points[int(float64(i)*step+0.5)])
	}
	out[n-1] = points[len(points)-1]
	return out
}
