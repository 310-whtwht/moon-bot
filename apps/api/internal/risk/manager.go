package risk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/moomoo-trading/api/internal/broker"
)

// RiskManager manages risk for trading strategies
type RiskManager struct {
	config              *RiskConfig
	positions           map[string]*Position
	circuits            map[string]*CircuitBreaker
	mu                  sync.RWMutex
	dailyLoss           float64
	weeklyLoss          float64
	lastDailyReset      time.Time
	lastWeeklyResetYear int
	lastWeeklyResetWeek int
}

// RiskConfig contains risk management configuration
type RiskConfig struct {
	MaxPositionSize        float64 `json:"max_position_size"`        // Maximum position size as percentage of account
	MaxDailyLoss           float64 `json:"max_daily_loss"`           // Maximum daily loss as percentage of account
	MaxWeeklyLoss          float64 `json:"max_weekly_loss"`          // Maximum weekly loss as percentage of account
	MaxDrawdown            float64 `json:"max_drawdown"`             // Maximum drawdown as percentage of account
	MaxConcurrentPositions int     `json:"max_concurrent_positions"` // Maximum number of concurrent positions
	ATRRiskPerTrade        float64 `json:"atr_risk_per_trade"`       // Risk per trade as percentage (0.25%)
}

// Position represents a trading position
type Position struct {
	Symbol    string    `json:"symbol"`
	Quantity  float64   `json:"quantity"`
	AvgPrice  float64   `json:"avg_price"`
	Side      string    `json:"side"` // "LONG" or "SHORT"
	OpenedAt  time.Time `json:"opened_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CircuitBreaker represents a circuit breaker for risk management
type CircuitBreaker struct {
	Symbol      string    `json:"symbol"`
	Type        string    `json:"type"` // "LOSS_LIMIT", "DRAWDOWN", "VOLATILITY"
	Triggered   bool      `json:"triggered"`
	TriggeredAt time.Time `json:"triggered_at"`
	Reason      string    `json:"reason"`
}

// NewRiskManager creates a new risk manager
func NewRiskManager(config *RiskConfig) *RiskManager {
	now := time.Now()
	year, week := now.ISOWeek()
	return &RiskManager{
		config:              config,
		positions:           make(map[string]*Position),
		circuits:            make(map[string]*CircuitBreaker),
		lastDailyReset:      now,
		lastWeeklyResetYear: year,
		lastWeeklyResetWeek: week,
	}
}

// CheckOrderRisk checks if an order meets risk requirements
func (rm *RiskManager) CheckOrderRisk(ctx context.Context, order *broker.Order, accountBalance float64) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check position size limit
	if err := rm.checkPositionSizeLimit(order, accountBalance); err != nil {
		return fmt.Errorf("position size limit exceeded: %w", err)
	}

	// Check concurrent positions limit
	if err := rm.checkConcurrentPositionsLimit(order); err != nil {
		return fmt.Errorf("concurrent positions limit exceeded: %w", err)
	}

	// Check circuit breakers
	if err := rm.checkCircuitBreakers(order.Symbol); err != nil {
		return fmt.Errorf("circuit breaker triggered: %w", err)
	}

	// Check daily loss limit
	if err := rm.checkDailyLossLimit(accountBalance); err != nil {
		return fmt.Errorf("daily loss limit exceeded: %w", err)
	}

	// Check weekly loss limit
	if err := rm.checkWeeklyLossLimit(accountBalance); err != nil {
		return fmt.Errorf("weekly loss limit exceeded: %w", err)
	}

	return nil
}

// checkPositionSizeLimit checks if the order size is within limits
func (rm *RiskManager) checkPositionSizeLimit(order *broker.Order, accountBalance float64) error {
	orderValue := order.Quantity * getOrderPrice(order)
	positionSizePercent := (orderValue / accountBalance) * 100

	if positionSizePercent > rm.config.MaxPositionSize {
		return fmt.Errorf("position size %.2f%% exceeds limit %.2f%%",
			positionSizePercent, rm.config.MaxPositionSize)
	}

	return nil
}

// checkConcurrentPositionsLimit checks if adding this order would exceed concurrent position limits
func (rm *RiskManager) checkConcurrentPositionsLimit(order *broker.Order) error {
	currentPositions := len(rm.positions)

	// If this is a new position (not closing an existing one)
	if _, exists := rm.positions[order.Symbol]; !exists {
		if currentPositions >= rm.config.MaxConcurrentPositions {
			return fmt.Errorf("concurrent positions limit %d exceeded", rm.config.MaxConcurrentPositions)
		}
	}

	return nil
}

// checkCircuitBreakers checks if any circuit breakers are triggered
func (rm *RiskManager) checkCircuitBreakers(symbol string) error {
	if circuit, exists := rm.circuits[symbol]; exists && circuit.Triggered {
		return fmt.Errorf("circuit breaker triggered for %s: %s", symbol, circuit.Reason)
	}

	return nil
}

// RecordPnL records realized profit and loss, updating loss counters only when
// the PnL represents a loss. Profits are ignored for drawdown calculations.
func (rm *RiskManager) RecordPnL(pnl float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.resetLossCountersIfNeeded(time.Now())
	if pnl < 0 {
		loss := -pnl
		rm.dailyLoss += loss
		rm.weeklyLoss += loss
	}
}

// resetLossCountersIfNeeded resets daily and weekly loss counters when period changes
func (rm *RiskManager) resetLossCountersIfNeeded(now time.Time) {
	if now.YearDay() != rm.lastDailyReset.YearDay() || now.Year() != rm.lastDailyReset.Year() {
		rm.dailyLoss = 0
		rm.lastDailyReset = now
	}

	year, week := now.ISOWeek()
	if year != rm.lastWeeklyResetYear || week != rm.lastWeeklyResetWeek {
		rm.weeklyLoss = 0
		rm.lastWeeklyResetYear = year
		rm.lastWeeklyResetWeek = week
	}
}

// checkDailyLossLimit checks if daily loss limit is exceeded
func (rm *RiskManager) checkDailyLossLimit(accountBalance float64) error {
	rm.resetLossCountersIfNeeded(time.Now())
	dailyLossPercent := (rm.dailyLoss / accountBalance) * 100

	if dailyLossPercent > rm.config.MaxDailyLoss {
		return fmt.Errorf("daily loss %.2f%% exceeds limit %.2f%%",
			dailyLossPercent, rm.config.MaxDailyLoss)
	}

	return nil
}

// checkWeeklyLossLimit checks if weekly loss limit is exceeded
func (rm *RiskManager) checkWeeklyLossLimit(accountBalance float64) error {
	rm.resetLossCountersIfNeeded(time.Now())
	weeklyLossPercent := (rm.weeklyLoss / accountBalance) * 100

	if weeklyLossPercent > rm.config.MaxWeeklyLoss {
		return fmt.Errorf("weekly loss %.2f%% exceeds limit %.2f%%",
			weeklyLossPercent, rm.config.MaxWeeklyLoss)
	}

	return nil
}

// UpdatePosition updates a position after order execution
func (rm *RiskManager) UpdatePosition(symbol string, quantity float64, price float64, side string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	position, exists := rm.positions[symbol]
	if !exists {
		// Create new position
		rm.positions[symbol] = &Position{
			Symbol:    symbol,
			Quantity:  quantity,
			AvgPrice:  price,
			Side:      side,
			OpenedAt:  time.Now(),
			UpdatedAt: time.Now(),
		}
	} else {
		// Update existing position
		if position.Side == side {
			// Same side - add to position
			totalValue := (position.Quantity * position.AvgPrice) + (quantity * price)
			position.Quantity += quantity
			position.AvgPrice = totalValue / position.Quantity
		} else {
			// Opposite side - reduce position
			if quantity >= position.Quantity {
				// Close position
				delete(rm.positions, symbol)
			} else {
				// Reduce position
				position.Quantity -= quantity
			}
		}
		position.UpdatedAt = time.Now()
	}
}

// TriggerCircuitBreaker triggers a circuit breaker for a symbol
func (rm *RiskManager) TriggerCircuitBreaker(symbol, circuitType, reason string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.circuits[symbol] = &CircuitBreaker{
		Symbol:      symbol,
		Type:        circuitType,
		Triggered:   true,
		TriggeredAt: time.Now(),
		Reason:      reason,
	}

	log.Printf("Circuit breaker triggered for %s: %s - %s", symbol, circuitType, reason)
}

// ResetCircuitBreaker resets a circuit breaker for a symbol
func (rm *RiskManager) ResetCircuitBreaker(symbol string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.circuits, symbol)
	log.Printf("Circuit breaker reset for %s", symbol)
}

// GetPositions returns all current positions
func (rm *RiskManager) GetPositions() map[string]*Position {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	positions := make(map[string]*Position)
	for symbol, position := range rm.positions {
		positions[symbol] = position
	}

	return positions
}

// GetCircuitBreakers returns all active circuit breakers
func (rm *RiskManager) GetCircuitBreakers() map[string]*CircuitBreaker {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	circuits := make(map[string]*CircuitBreaker)
	for symbol, circuit := range rm.circuits {
		circuits[symbol] = circuit
	}

	return circuits
}

// CalculatePositionSize calculates the appropriate position size based on ATR
func (rm *RiskManager) CalculatePositionSize(symbol string, atr float64, accountBalance float64) float64 {
	// Calculate risk amount based on ATR
	riskAmount := accountBalance * (rm.config.ATRRiskPerTrade / 100)

	// Calculate position size based on ATR
	positionSize := riskAmount / atr

	return positionSize
}

// getOrderPrice gets the effective price for an order
func getOrderPrice(order *broker.Order) float64 {
	if order.Price != nil {
		return *order.Price
	}
	// For market orders, use a reasonable estimate
	// In practice, this would be the current market price
	return 100.0 // Placeholder
}
