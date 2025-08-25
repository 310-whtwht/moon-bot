package data

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moomoo-trading/api/internal/broker"
	"github.com/moomoo-trading/api/internal/redis"
)

// DataManager manages historical and real-time data
type DataManager struct {
	broker        *broker.MoomooAdapter
	streamManager *redis.StreamManager
	storage       DataStorage
}

// DataStorage interface for storing and retrieving data
type DataStorage interface {
	StoreBarData(symbol string, bars []Bar) error
	GetBarData(symbol string, startDate, endDate time.Time, interval string) ([]Bar, error)
	StoreTradeData(symbol string, trades []broker.Trade) error
	GetTradeData(symbol string, startDate, endDate time.Time) ([]broker.Trade, error)
}

// Bar represents a price bar
type Bar struct {
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// NewDataManager creates a new data manager
func NewDataManager(broker *broker.MoomooAdapter, streamManager *redis.StreamManager, storage DataStorage) *DataManager {
	return &DataManager{
		broker:        broker,
		streamManager: streamManager,
		storage:       storage,
	}
}

// SyncHistoricalData synchronizes historical data for a symbol
func (dm *DataManager) SyncHistoricalData(ctx context.Context, symbol string, startDate, endDate time.Time) error {
	log.Printf("Syncing historical data for %s from %s to %s", 
		symbol, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// TODO: Implement actual data synchronization from Moomoo
	// This would involve:
	// 1. Fetching data from Moomoo API
	// 2. Processing and cleaning the data
	// 3. Storing in local database
	// 4. Publishing to Redis streams for real-time access

	// Placeholder implementation
	bars := dm.generateSampleData(symbol, startDate, endDate)
	
	if err := dm.storage.StoreBarData(symbol, bars); err != nil {
		return fmt.Errorf("failed to store bar data: %w", err)
	}

	log.Printf("Successfully synced %d bars for %s", len(bars), symbol)
	return nil
}

// GetHistoricalData retrieves historical data for a symbol
func (dm *DataManager) GetHistoricalData(ctx context.Context, symbol string, startDate, endDate time.Time, interval string) ([]Bar, error) {
	return dm.storage.GetBarData(symbol, startDate, endDate, interval)
}

// SubscribeRealTimeData subscribes to real-time data for a symbol
func (dm *DataManager) SubscribeRealTimeData(ctx context.Context, symbol string) (<-chan broker.MarketData, error) {
	return dm.broker.SubscribeMarketData(ctx, symbol)
}

// StoreTradeData stores trade data
func (dm *DataManager) StoreTradeData(ctx context.Context, symbol string, trades []broker.Trade) error {
	// Store in database
	if err := dm.storage.StoreTradeData(symbol, trades); err != nil {
		return fmt.Errorf("failed to store trade data: %w", err)
	}

	// Publish to Redis stream
	for _, trade := range trades {
		event := map[string]interface{}{
			"trade_id":    trade.ID,
			"symbol":      trade.Symbol,
			"side":        trade.Side,
			"quantity":    trade.Quantity,
			"price":       trade.Price,
			"commission":  trade.Commission,
			"trade_time":  trade.TradeTime.Unix(),
			"timestamp":   time.Now().Unix(),
		}

		if err := dm.streamManager.PublishTradeEvent(ctx, event); err != nil {
			log.Printf("Failed to publish trade event: %v", err)
		}
	}

	return nil
}

// GetTradeData retrieves trade data for a symbol
func (dm *DataManager) GetTradeData(ctx context.Context, symbol string, startDate, endDate time.Time) ([]broker.Trade, error) {
	return dm.storage.GetTradeData(symbol, startDate, endDate)
}

// ArchiveData archives old data to reduce storage costs
func (dm *DataManager) ArchiveData(ctx context.Context, symbol string, cutoffDate time.Time) error {
	log.Printf("Archiving data for %s older than %s", symbol, cutoffDate.Format("2006-01-02"))

	// TODO: Implement data archiving
	// This would involve:
	// 1. Moving old data to cheaper storage (e.g., S3)
	// 2. Compressing data
	// 3. Updating database indexes
	// 4. Cleaning up local storage

	return nil
}

// generateSampleData generates sample data for testing
func (dm *DataManager) generateSampleData(symbol string, startDate, endDate time.Time) []Bar {
	var bars []Bar
	currentDate := startDate
	basePrice := 100.0

	for currentDate.Before(endDate) {
		// Generate random price movement
		change := (rand.Float64() - 0.5) * 2 // -1 to +1
		basePrice += change

		bar := Bar{
			Timestamp: currentDate,
			Symbol:    symbol,
			Open:      basePrice,
			High:      basePrice + rand.Float64()*0.5,
			Low:       basePrice - rand.Float64()*0.5,
			Close:     basePrice + (rand.Float64()-0.5)*0.3,
			Volume:    rand.Float64() * 1000000,
		}

		bars = append(bars, bar)
		currentDate = currentDate.Add(time.Minute)
	}

	return bars
}

// DataQualityCheck performs quality checks on the data
func (dm *DataManager) DataQualityCheck(ctx context.Context, symbol string, startDate, endDate time.Time) (*DataQualityReport, error) {
	bars, err := dm.GetHistoricalData(ctx, symbol, startDate, endDate, "1m")
	if err != nil {
		return nil, fmt.Errorf("failed to get data for quality check: %w", err)
	}

	report := &DataQualityReport{
		Symbol:     symbol,
		StartDate:  startDate,
		EndDate:    endDate,
		TotalBars:  len(bars),
		Issues:     make([]DataIssue, 0),
	}

	// Check for gaps in data
	expectedBars := int(endDate.Sub(startDate).Minutes())
	if len(bars) < expectedBars*0.95 { // Allow 5% missing data
		report.Issues = append(report.Issues, DataIssue{
			Type:        "MISSING_DATA",
			Description: fmt.Sprintf("Expected %d bars, got %d", expectedBars, len(bars)),
			Severity:    "WARNING",
		})
	}

	// Check for price anomalies
	for i, bar := range bars {
		if bar.High < bar.Low {
			report.Issues = append(report.Issues, DataIssue{
				Type:        "PRICE_ANOMALY",
				Description: fmt.Sprintf("Bar %d: High (%.2f) < Low (%.2f)", i, bar.High, bar.Low),
				Severity:    "ERROR",
			})
		}

		if bar.Open < 0 || bar.High < 0 || bar.Low < 0 || bar.Close < 0 {
			report.Issues = append(report.Issues, DataIssue{
				Type:        "NEGATIVE_PRICE",
				Description: fmt.Sprintf("Bar %d: Negative price detected", i),
				Severity:    "ERROR",
			})
		}
	}

	return report, nil
}

// DataQualityReport contains data quality analysis results
type DataQualityReport struct {
	Symbol    string       `json:"symbol"`
	StartDate time.Time    `json:"start_date"`
	EndDate   time.Time    `json:"end_date"`
	TotalBars int          `json:"total_bars"`
	Issues    []DataIssue  `json:"issues"`
}

// DataIssue represents a data quality issue
type DataIssue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "INFO", "WARNING", "ERROR"
}