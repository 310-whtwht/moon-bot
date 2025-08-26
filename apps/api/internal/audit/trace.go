package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// TradeTrace トレード鎖状トレース
type TradeTrace struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	StrategyID  string                 `json:"strategy_id"`
	Symbol      string                 `json:"symbol"`
	Side        string                 `json:"side"`
	Quantity    int                    `json:"quantity"`
	Price       float64                `json:"price"`
	Timestamp   time.Time              `json:"timestamp"`
	OrderID     string                 `json:"order_id"`
	TradeID     string                 `json:"trade_id"`
	ParentID    *string                `json:"parent_id,omitempty"`
	TraceID     string                 `json:"trace_id"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:json"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TraceManager トレース管理
type TraceManager struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewTraceManager 新しいトレースマネージャーを作成
func NewTraceManager(db *gorm.DB, redis *redis.Client) *TraceManager {
	return &TraceManager{
		db:    db,
		redis: redis,
	}
}

// CreateTrace トレースを作成
func (tm *TraceManager) CreateTrace(ctx context.Context, trace *TradeTrace) error {
	// データベースに保存
	if err := tm.db.WithContext(ctx).Create(trace).Error; err != nil {
		return fmt.Errorf("failed to create trace: %w", err)
	}

	// Redis Streams にイベントを送信
	event := map[string]interface{}{
		"type":       "trade_trace_created",
		"trace_id":   trace.TraceID,
		"strategy_id": trace.StrategyID,
		"symbol":     trace.Symbol,
		"side":       trace.Side,
		"timestamp":  trace.Timestamp.Unix(),
	}

	eventData, _ := json.Marshal(event)
	tm.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "audit_traces",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})

	return nil
}

// GetTraceByID トレースIDでトレースを取得
func (tm *TraceManager) GetTraceByID(ctx context.Context, traceID string) (*TradeTrace, error) {
	var trace TradeTrace
	if err := tm.db.WithContext(ctx).Where("trace_id = ?", traceID).First(&trace).Error; err != nil {
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}
	return &trace, nil
}

// GetTraceChain トレース鎖を取得
func (tm *TraceManager) GetTraceChain(ctx context.Context, traceID string) ([]*TradeTrace, error) {
	var traces []*TradeTrace
	
	// 指定されたトレースIDから開始して、親子関係を辿る
	currentTraceID := traceID
	for {
		var trace TradeTrace
		if err := tm.db.WithContext(ctx).Where("trace_id = ?", currentTraceID).First(&trace).Error; err != nil {
			break
		}
		
		traces = append([]*TradeTrace{&trace}, traces...) // 先頭に追加
		
		if trace.ParentID == nil {
			break
		}
		currentTraceID = *trace.ParentID
	}
	
	return traces, nil
}

// GetTracesByStrategy 戦略IDでトレースを取得
func (tm *TraceManager) GetTracesByStrategy(ctx context.Context, strategyID string, limit int) ([]*TradeTrace, error) {
	var traces []*TradeTrace
	query := tm.db.WithContext(ctx).Where("strategy_id = ?", strategyID).Order("timestamp DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	if err := query.Find(&traces).Error; err != nil {
		return nil, fmt.Errorf("failed to get traces by strategy: %w", err)
	}
	
	return traces, nil
}

// GetTracesBySymbol 銘柄でトレースを取得
func (tm *TraceManager) GetTracesBySymbol(ctx context.Context, symbol string, startTime, endTime time.Time) ([]*TradeTrace, error) {
	var traces []*TradeTrace
	if err := tm.db.WithContext(ctx).
		Where("symbol = ? AND timestamp BETWEEN ? AND ?", symbol, startTime, endTime).
		Order("timestamp DESC").
		Find(&traces).Error; err != nil {
		return nil, fmt.Errorf("failed to get traces by symbol: %w", err)
	}
	
	return traces, nil
}

// GetTraceStatistics トレース統計を取得
func (tm *TraceManager) GetTraceStatistics(ctx context.Context, strategyID string, startTime, endTime time.Time) (map[string]interface{}, error) {
	var stats struct {
		TotalTrades    int64   `json:"total_trades"`
		TotalVolume    int64   `json:"total_volume"`
		TotalValue     float64 `json:"total_value"`
		BuyCount       int64   `json:"buy_count"`
		SellCount      int64   `json:"sell_count"`
		AvgPrice       float64 `json:"avg_price"`
		MaxPrice       float64 `json:"max_price"`
		MinPrice       float64 `json:"min_price"`
	}

	query := tm.db.WithContext(ctx).Model(&TradeTrace{}).
		Where("strategy_id = ? AND timestamp BETWEEN ? AND ?", strategyID, startTime, endTime)

	if err := query.Select(`
		COUNT(*) as total_trades,
		SUM(quantity) as total_volume,
		SUM(quantity * price) as total_value,
		SUM(CASE WHEN side = 'buy' THEN 1 ELSE 0 END) as buy_count,
		SUM(CASE WHEN side = 'sell' THEN 1 ELSE 0 END) as sell_count,
		AVG(price) as avg_price,
		MAX(price) as max_price,
		MIN(price) as min_price
	`).Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get trace statistics: %w", err)
	}

	return map[string]interface{}{
		"total_trades": stats.TotalTrades,
		"total_volume": stats.TotalVolume,
		"total_value":  stats.TotalValue,
		"buy_count":    stats.BuyCount,
		"sell_count":   stats.SellCount,
		"avg_price":    stats.AvgPrice,
		"max_price":    stats.MaxPrice,
		"min_price":    stats.MinPrice,
	}, nil
}

// SearchTraces トレース検索
func (tm *TraceManager) SearchTraces(ctx context.Context, filters map[string]interface{}, page, limit int) ([]*TradeTrace, int64, error) {
	var traces []*TradeTrace
	var total int64

	query := tm.db.WithContext(ctx).Model(&TradeTrace{})

	// フィルター適用
	if strategyID, ok := filters["strategy_id"].(string); ok && strategyID != "" {
		query = query.Where("strategy_id = ?", strategyID)
	}
	if symbol, ok := filters["symbol"].(string); ok && symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	if side, ok := filters["side"].(string); ok && side != "" {
		query = query.Where("side = ?", side)
	}
	if startTime, ok := filters["start_time"].(time.Time); ok {
		query = query.Where("timestamp >= ?", startTime)
	}
	if endTime, ok := filters["end_time"].(time.Time); ok {
		query = query.Where("timestamp <= ?", endTime)
	}

	// 総件数取得
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count traces: %w", err)
	}

	// ページネーション適用
	offset := (page - 1) * limit
	if err := query.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&traces).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search traces: %w", err)
	}

	return traces, total, nil
}

// ExportTraces トレースエクスポート
func (tm *TraceManager) ExportTraces(ctx context.Context, filters map[string]interface{}, format string) ([]byte, error) {
	traces, _, err := tm.SearchTraces(ctx, filters, 1, 10000) // 最大10000件
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return json.Marshal(traces)
	case "csv":
		return tm.exportToCSV(traces)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// exportToCSV CSV形式でエクスポート
func (tm *TraceManager) exportToCSV(traces []*TradeTrace) ([]byte, error) {
	csv := "TraceID,StrategyID,Symbol,Side,Quantity,Price,Timestamp,OrderID,TradeID,ParentID\n"
	
	for _, trace := range traces {
		parentID := ""
		if trace.ParentID != nil {
			parentID = *trace.ParentID
		}
		
		csv += fmt.Sprintf("%s,%s,%s,%s,%d,%.2f,%s,%s,%s,%s\n",
			trace.TraceID,
			trace.StrategyID,
			trace.Symbol,
			trace.Side,
			trace.Quantity,
			trace.Price,
			trace.Timestamp.Format(time.RFC3339),
			trace.OrderID,
			trace.TradeID,
			parentID,
		)
	}
	
	return []byte(csv), nil
}