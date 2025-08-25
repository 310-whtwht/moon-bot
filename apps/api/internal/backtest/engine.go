package backtest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moomoo-trading/api/internal/broker"
	"github.com/moomoo-trading/api/internal/strategy"
)

// BacktestEngine represents the backtesting engine
type BacktestEngine struct {
	dataProvider DataProvider
	riskManager  RiskManager
}

// DataProvider provides historical data for backtesting
type DataProvider interface {
	GetHistoricalData(symbol string, startDate, endDate time.Time, interval string) ([]Bar, error)
}

// RiskManager manages risk during backtesting
type RiskManager interface {
	CheckOrderRisk(ctx context.Context, order *broker.Order, accountBalance float64) error
	UpdatePosition(symbol string, quantity float64, price float64, side string)
}

// Bar represents a price bar
type Bar struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// BacktestConfig contains backtest configuration
type BacktestConfig struct {
	Symbol        string                 `json:"symbol"`
	StartDate     time.Time              `json:"start_date"`
	EndDate       time.Time              `json:"end_date"`
	InitialBalance float64               `json:"initial_balance"`
	Strategy      *strategy.Strategy     `json:"strategy"`
	Parameters    map[string]interface{} `json:"parameters"`
}

// BacktestResult contains the results of a backtest
type BacktestResult struct {
	Config           *BacktestConfig `json:"config"`
	Trades           []Trade         `json:"trades"`
	Equity           []EquityPoint   `json:"equity"`
	Performance      *Performance    `json:"performance"`
	CompletedAt      time.Time       `json:"completed_at"`
}

// Trade represents a trade in the backtest
type Trade struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`
	Quantity    float64   `json:"quantity"`
	EntryPrice  float64   `json:"entry_price"`
	ExitPrice   float64   `json:"exit_price"`
	EntryTime   time.Time `json:"entry_time"`
	ExitTime    time.Time `json:"exit_time"`
	PnL         float64   `json:"pnl"`
	Commission  float64   `json:"commission"`
}

// EquityPoint represents an equity point in the backtest
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Equity    float64   `json:"equity"`
	Drawdown  float64   `json:"drawdown"`
}

// Performance contains performance metrics
type Performance struct {
	TotalReturn     float64 `json:"total_return"`
	AnnualizedReturn float64 `json:"annualized_return"`
	SharpeRatio     float64 `json:"sharpe_ratio"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	WinRate         float64 `json:"win_rate"`
	ProfitFactor    float64 `json:"profit_factor"`
	SQN             float64 `json:"sqn"` // System Quality Number
	CAGR            float64 `json:"cagr"` // Compound Annual Growth Rate
}

// NewBacktestEngine creates a new backtest engine
func NewBacktestEngine(dataProvider DataProvider, riskManager RiskManager) *BacktestEngine {
	return &BacktestEngine{
		dataProvider: dataProvider,
		riskManager:  riskManager,
	}
}

// RunBacktest runs a backtest with the given configuration
func (be *BacktestEngine) RunBacktest(ctx context.Context, config *BacktestConfig) (*BacktestResult, error) {
	log.Printf("Starting backtest for %s from %s to %s", 
		config.Symbol, config.StartDate.Format("2006-01-02"), config.EndDate.Format("2006-01-02"))

	// Get historical data
	data, err := be.dataProvider.GetHistoricalData(
		config.Symbol, 
		config.StartDate, 
		config.EndDate, 
		"1m", // Default to 1-minute bars
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data: %w", err)
	}

	// Initialize backtest state
	state := &BacktestState{
		Config:       config,
		Balance:      config.InitialBalance,
		Equity:       config.InitialBalance,
		Position:     nil,
		Trades:       make([]Trade, 0),
		EquityPoints: make([]EquityPoint, 0),
		Bars:         data,
	}

	// Run the backtest
	if err := be.runBacktest(ctx, state); err != nil {
		return nil, fmt.Errorf("backtest execution failed: %w", err)
	}

	// Calculate performance metrics
	performance := be.calculatePerformance(state)

	result := &BacktestResult{
		Config:      config,
		Trades:      state.Trades,
		Equity:      state.EquityPoints,
		Performance: performance,
		CompletedAt: time.Now(),
	}

	log.Printf("Backtest completed. Total return: %.2f%%, Max drawdown: %.2f%%", 
		performance.TotalReturn*100, performance.MaxDrawdown*100)

	return result, nil
}

// BacktestState represents the state during backtesting
type BacktestState struct {
	Config       *BacktestConfig
	Balance      float64
	Equity       float64
	Position     *Position
	Trades       []Trade
	EquityPoints []EquityPoint
	Bars         []Bar
	CurrentBar   int
}

// Position represents a position during backtesting
type Position struct {
	Symbol   string
	Quantity float64
	Side     string
	EntryPrice float64
	EntryTime time.Time
}

// runBacktest executes the backtest
func (be *BacktestEngine) runBacktest(ctx context.Context, state *BacktestState) error {
	// TODO: Implement actual strategy execution
	// This is a placeholder for the actual backtest logic

	for i, bar := range state.Bars {
		state.CurrentBar = i

		// Update equity
		state.updateEquity(bar)

		// Execute strategy logic
		if err := be.executeStrategy(state, bar); err != nil {
			return fmt.Errorf("strategy execution failed: %w", err)
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// executeStrategy executes the strategy logic for a bar
func (be *BacktestEngine) executeStrategy(state *BacktestState, bar Bar) error {
	// TODO: Implement actual strategy execution
	// This would involve:
	// 1. Running the Starlark code
	// 2. Checking for signals
	// 3. Executing orders
	// 4. Updating positions

	return nil
}

// updateEquity updates the equity curve
func (state *BacktestState) updateEquity(bar Bar) {
	// Calculate current equity including unrealized PnL
	equity := state.Balance
	if state.Position != nil {
		unrealizedPnL := state.calculateUnrealizedPnL(bar.Close)
		equity += unrealizedPnL
	}

	state.Equity = equity

	// Add equity point
	state.EquityPoints = append(state.EquityPoints, EquityPoint{
		Timestamp: bar.Timestamp,
		Equity:    equity,
		Drawdown:  state.calculateDrawdown(equity),
	})
}

// calculateUnrealizedPnL calculates unrealized PnL for current position
func (state *BacktestState) calculateUnrealizedPnL(currentPrice float64) float64 {
	if state.Position == nil {
		return 0
	}

	if state.Position.Side == "LONG" {
		return (currentPrice - state.Position.EntryPrice) * state.Position.Quantity
	} else {
		return (state.Position.EntryPrice - currentPrice) * state.Position.Quantity
	}
}

// calculateDrawdown calculates the current drawdown
func (state *BacktestState) calculateDrawdown(currentEquity float64) float64 {
	// Find peak equity
	peak := state.Config.InitialBalance
	for _, point := range state.EquityPoints {
		if point.Equity > peak {
			peak = point.Equity
		}
	}

	if peak == 0 {
		return 0
	}

	return (peak - currentEquity) / peak
}

// calculatePerformance calculates performance metrics
func (be *BacktestEngine) calculatePerformance(state *BacktestState) *Performance {
	if len(state.EquityPoints) == 0 {
		return &Performance{}
	}

	initialEquity := state.Config.InitialBalance
	finalEquity := state.EquityPoints[len(state.EquityPoints)-1].Equity

	// Calculate basic metrics
	totalReturn := (finalEquity - initialEquity) / initialEquity

	// Calculate max drawdown
	maxDrawdown := 0.0
	peak := initialEquity
	for _, point := range state.EquityPoints {
		if point.Equity > peak {
			peak = point.Equity
		}
		drawdown := (peak - point.Equity) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	// Calculate win rate
	wins := 0
	totalTrades := len(state.Trades)
	for _, trade := range state.Trades {
		if trade.PnL > 0 {
			wins++
		}
	}
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades)
	}

	// Calculate profit factor
	grossProfit := 0.0
	grossLoss := 0.0
	for _, trade := range state.Trades {
		if trade.PnL > 0 {
			grossProfit += trade.PnL
		} else {
			grossLoss += -trade.PnL
		}
	}
	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	// Calculate annualized return
	duration := state.EquityPoints[len(state.EquityPoints)-1].Timestamp.Sub(state.EquityPoints[0].Timestamp)
	years := duration.Hours() / 8760 // 8760 hours in a year
	annualizedReturn := 0.0
	if years > 0 {
		annualizedReturn = (1 + totalReturn) / years
	}

	// Calculate CAGR
	cagr := 0.0
	if years > 0 {
		cagr = (finalEquity / initialEquity) - 1
		cagr = (1 + cagr) / years
	}

	return &Performance{
		TotalReturn:      totalReturn,
		AnnualizedReturn: annualizedReturn,
		MaxDrawdown:      maxDrawdown,
		WinRate:          winRate,
		ProfitFactor:     profitFactor,
		CAGR:             cagr,
		// TODO: Calculate Sharpe ratio and SQN
	}
}