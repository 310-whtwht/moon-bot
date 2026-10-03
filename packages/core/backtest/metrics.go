package backtest

import (
	"math"
	"time"
)

// Metrics summarises a backtest. Ratios are fractions (0.1 = 10%).
type Metrics struct {
	InitialBalance float64 `json:"initial_balance"`
	FinalEquity    float64 `json:"final_equity"`
	NetProfit      float64 `json:"net_profit"`
	TotalReturn    float64 `json:"total_return"`
	CAGR           float64 `json:"cagr"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	Sharpe         float64 `json:"sharpe"` // annualised from daily returns (252 days)
	NumTrades      int     `json:"num_trades"`
	WinRate        float64 `json:"win_rate"`
	// ProfitFactor is gross profit / gross loss; 0 when there are no losing trades.
	ProfitFactor float64 `json:"profit_factor"`
	AvgTrade     float64 `json:"avg_trade"`
	SQN          float64 `json:"sqn"` // sqrt(N) * mean(PnL) / stdev(PnL)
	TotalFees    float64 `json:"total_fees"`
	Exposure     float64 `json:"exposure"` // share of bars with an open position
}

func computeMetrics(res *Result, initial float64, inBars int) Metrics {
	m := Metrics{InitialBalance: initial, FinalEquity: initial}
	if n := len(res.Equity); n > 0 {
		m.FinalEquity = res.Equity[n-1].Equity
	}
	m.NetProfit = m.FinalEquity - initial
	m.TotalReturn = m.FinalEquity/initial - 1

	years := res.To.Sub(res.From).Hours() / (365.25 * 24)
	if years > 0 && m.FinalEquity > 0 {
		m.CAGR = math.Pow(m.FinalEquity/initial, 1/years) - 1
	}

	m.MaxDrawdown = maxDrawdown(initial, res.Equity)
	m.Sharpe = sharpe(res.Equity)
	if res.Bars > 0 {
		m.Exposure = float64(inBars) / float64(res.Bars)
	}

	m.NumTrades = len(res.Trades)
	if m.NumTrades == 0 {
		return m
	}
	var wins int
	var grossProfit, grossLoss, sum float64
	pnls := make([]float64, 0, m.NumTrades)
	for _, t := range res.Trades {
		pnls = append(pnls, t.PnL)
		sum += t.PnL
		m.TotalFees += t.Fees
		if t.PnL > 0 {
			wins++
			grossProfit += t.PnL
		} else {
			grossLoss -= t.PnL
		}
	}
	m.WinRate = float64(wins) / float64(m.NumTrades)
	if grossLoss > 0 {
		m.ProfitFactor = grossProfit / grossLoss
	}
	m.AvgTrade = sum / float64(m.NumTrades)
	if sd := stdev(pnls); sd > 0 {
		m.SQN = math.Sqrt(float64(m.NumTrades)) * m.AvgTrade / sd
	}
	return m
}

// maxDrawdown is the largest peak-to-trough fall, in one pass.
func maxDrawdown(initial float64, equity []EquityPoint) float64 {
	peak, worst := initial, 0.0
	for _, p := range equity {
		if p.Equity > peak {
			peak = p.Equity
		}
		if peak > 0 {
			if dd := (peak - p.Equity) / peak; dd > worst {
				worst = dd
			}
		}
	}
	return worst
}

// sharpe annualises the mean/stdev of daily returns (last equity per UTC day).
func sharpe(equity []EquityPoint) float64 {
	var daily []float64
	var lastDay time.Time
	for _, p := range equity {
		day := p.Time.UTC().Truncate(24 * time.Hour)
		if len(daily) > 0 && day.Equal(lastDay) {
			daily[len(daily)-1] = p.Equity
			continue
		}
		daily = append(daily, p.Equity)
		lastDay = day
	}
	if len(daily) < 3 {
		return 0
	}
	returns := make([]float64, 0, len(daily)-1)
	for i := 1; i < len(daily); i++ {
		if daily[i-1] != 0 {
			returns = append(returns, daily[i]/daily[i-1]-1)
		}
	}
	sd := stdev(returns)
	if sd == 0 {
		return 0
	}
	return mean(returns) / sd * math.Sqrt(252)
}

func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// stdev is the sample standard deviation.
func stdev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var ss float64
	for _, x := range xs {
		ss += (x - m) * (x - m)
	}
	return math.Sqrt(ss / float64(len(xs)-1))
}
