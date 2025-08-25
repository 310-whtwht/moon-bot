package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"moomoo/internal/audit"
)

// AuditHandler 監査ログハンドラー
type AuditHandler struct {
	traceManager *audit.TraceManager
}

// NewAuditHandler 新しい監査ハンドラーを作成
func NewAuditHandler(traceManager *audit.TraceManager) *AuditHandler {
	return &AuditHandler{
		traceManager: traceManager,
	}
}

// CreateTrace トレース作成
func (h *AuditHandler) CreateTrace(c *gin.Context) {
	var req struct {
		StrategyID string                 `json:"strategy_id" binding:"required"`
		Symbol     string                 `json:"symbol" binding:"required"`
		Side       string                 `json:"side" binding:"required"`
		Quantity   int                    `json:"quantity" binding:"required"`
		Price      float64                `json:"price" binding:"required"`
		OrderID    string                 `json:"order_id" binding:"required"`
		TradeID    string                 `json:"trade_id" binding:"required"`
		ParentID   *string                `json:"parent_id"`
		Metadata   map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	trace := &audit.TradeTrace{
		ID:         uuid.New().String(),
		StrategyID: req.StrategyID,
		Symbol:     req.Symbol,
		Side:       req.Side,
		Quantity:   req.Quantity,
		Price:      req.Price,
		Timestamp:  time.Now(),
		OrderID:    req.OrderID,
		TradeID:    req.TradeID,
		ParentID:   req.ParentID,
		TraceID:    uuid.New().String(),
		Metadata:   req.Metadata,
	}

	if err := h.traceManager.CreateTrace(c.Request.Context(), trace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, trace)
}

// GetTraceByID トレースIDでトレース取得
func (h *AuditHandler) GetTraceByID(c *gin.Context) {
	traceID := c.Param("id")
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "trace_id is required"})
		return
	}

	trace, err := h.traceManager.GetTraceByID(c.Request.Context(), traceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Trace not found"})
		return
	}

	c.JSON(http.StatusOK, trace)
}

// GetTraceChain トレース鎖取得
func (h *AuditHandler) GetTraceChain(c *gin.Context) {
	traceID := c.Param("id")
	if traceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "trace_id is required"})
		return
	}

	traces, err := h.traceManager.GetTraceChain(c.Request.Context(), traceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trace_id": traceID,
		"chain":    traces,
		"length":   len(traces),
	})
}

// GetTracesByStrategy 戦略IDでトレース取得
func (h *AuditHandler) GetTracesByStrategy(c *gin.Context) {
	strategyID := c.Param("strategy_id")
	if strategyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "strategy_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	traces, err := h.traceManager.GetTracesByStrategy(c.Request.Context(), strategyID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"strategy_id": strategyID,
		"traces":      traces,
		"count":       len(traces),
	})
}

// GetTracesBySymbol 銘柄でトレース取得
func (h *AuditHandler) GetTracesBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "symbol is required"})
		return
	}

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "Invalid start_time format"})
			return
		}
	} else {
		startTime = time.Now().AddDate(0, 0, -7) // デフォルト: 1週間前
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "Invalid end_time format"})
			return
		}
	} else {
		endTime = time.Now()
	}

	traces, err := h.traceManager.GetTracesBySymbol(c.Request.Context(), symbol, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"traces":    traces,
		"count":     len(traces),
		"start_time": startTime,
		"end_time":   endTime,
	})
}

// GetTraceStatistics トレース統計取得
func (h *AuditHandler) GetTraceStatistics(c *gin.Context) {
	strategyID := c.Param("strategy_id")
	if strategyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "strategy_id is required"})
		return
	}

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "Invalid start_time format"})
			return
		}
	} else {
		startTime = time.Now().AddDate(0, 0, -30) // デフォルト: 30日前
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "Invalid end_time format"})
			return
		}
	} else {
		endTime = time.Now()
	}

	stats, err := h.traceManager.GetTraceStatistics(c.Request.Context(), strategyID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"strategy_id": strategyID,
		"statistics":  stats,
		"start_time":  startTime,
		"end_time":    endTime,
	})
}

// SearchTraces トレース検索
func (h *AuditHandler) SearchTraces(c *gin.Context) {
	var filters map[string]interface{}
	if err := c.ShouldBindJSON(&filters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	traces, total, err := h.traceManager.SearchTraces(c.Request.Context(), filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"traces": traces,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
		"filters": filters,
	})
}

// ExportTraces トレースエクスポート
func (h *AuditHandler) ExportTraces(c *gin.Context) {
	var filters map[string]interface{}
	if err := c.ShouldBindJSON(&filters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "Unsupported format"})
		return
	}

	data, err := h.traceManager.ExportTraces(c.Request.Context(), filters, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	contentType := "application/json"
	filename := "traces.json"
	if format == "csv" {
		contentType = "text/csv"
		filename = "traces.csv"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}