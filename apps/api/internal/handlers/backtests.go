package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/market"
)

// BacktestQueue hands a created backtest to the bot.
type BacktestQueue interface {
	PublishBacktestJob(ctx context.Context, backtestID string) error
}

// BacktestHandler handles backtest-related HTTP requests
type BacktestHandler struct {
	repo       *database.BacktestRepository
	strategies *database.StrategyRepository
	queue      BacktestQueue
}

// NewBacktestHandler creates a new backtest handler
func NewBacktestHandler(repo *database.BacktestRepository, strategies *database.StrategyRepository, queue BacktestQueue) *BacktestHandler {
	return &BacktestHandler{repo: repo, strategies: strategies, queue: queue}
}

// GetBacktests retrieves backtests with filtering
func (h *BacktestHandler) GetBacktests(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	strategyID := c.Query("strategy_id")
	status := c.Query("status")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset parameter"})
		return
	}

	var strategyIDPtr, statusPtr *string
	if strategyID != "" {
		strategyIDPtr = &strategyID
	}
	if status != "" {
		statusPtr = &status
	}

	backtests, err := h.repo.ListBacktests(c.Request.Context(), strategyIDPtr, statusPtr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve backtests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": backtests,
		"limit": limit,
		"offset": offset,
	})
}

// CreateBacktest validates the request, stores a snapshot of what will run
// (strategy version, parameters, instrument, size) and queues it for the bot.
func (h *BacktestHandler) CreateBacktest(c *gin.Context) {
	var req struct {
		Name              string   `json:"name" binding:"required"`
		StrategyID        string   `json:"strategy_id" binding:"required"`
		StrategyVersionID string   `json:"strategy_version_id"`
		Symbols           []string `json:"symbols" binding:"required"`
		Timeframe         string   `json:"timeframe"`
		StartDate         string   `json:"start_date" binding:"required"`
		EndDate           string   `json:"end_date" binding:"required"`
		Units             float64  `json:"units"`
		InitialBalance    float64  `json:"initial_balance"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(req.Symbols) != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exactly one symbol is supported"})
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "End date must be after start date"})
		return
	}

	ctx := c.Request.Context()
	version, err := h.resolveVersion(ctx, req.StrategyID, req.StrategyVersionID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy version not found (create an active version first)"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategy version"})
		return
	}
	params, err := h.strategies.NumericParams(ctx, version.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategy parameters"})
		return
	}

	spec := backtest.JobSpec{
		StrategyVersionID: version.ID,
		StrategyType:      version.Code,
		StrategyParams:    params,
		Broker:            gmofx.BrokerName,
		Symbol:            req.Symbols[0],
		Timeframe:         market.Timeframe(defaultString(req.Timeframe, string(market.TF1Hour))),
		Units:             defaultFloat(req.Units, 100),
		InitialBalance:    defaultFloat(req.InitialBalance, 30000),
	}
	if _, err := spec.Resolve(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode parameters"})
		return
	}

	bt := &database.Backtest{
		Name:       req.Name,
		StrategyID: req.StrategyID,
		Symbols:    req.Symbols,
		StartDate:  startDate,
		EndDate:    endDate,
		Parameters: specJSON,
	}
	if err := h.repo.CreateBacktest(ctx, bt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backtest"})
		return
	}

	if err := h.queue.PublishBacktestJob(ctx, bt.ID); err != nil {
		msg := "Failed to queue backtest: " + err.Error()
		_ = h.repo.UpdateBacktestStatus(ctx, bt.ID, "failed", 0, nil, &msg)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to queue backtest"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": bt})
}

// resolveVersion returns the requested version, or the package's active one.
func (h *BacktestHandler) resolveVersion(ctx context.Context, strategyID, versionID string) (*database.StrategyVersion, error) {
	if versionID == "" {
		return h.strategies.GetActiveVersionByPackageID(ctx, strategyID)
	}
	v, err := h.strategies.GetVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v.PackageID != strategyID {
		return nil, sql.ErrNoRows
	}
	return v, nil
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultFloat(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

// GetBacktest retrieves a backtest by ID
func (h *BacktestHandler) GetBacktest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backtest ID is required"})
		return
	}

	backtest, err := h.repo.GetBacktestByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backtest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve backtest"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": backtest})
}

// DeleteBacktest deletes a backtest
func (h *BacktestHandler) DeleteBacktest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backtest ID is required"})
		return
	}

	// Check if backtest exists
	backtest, err := h.repo.GetBacktestByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backtest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve backtest"})
		return
	}

	// Check if backtest can be deleted
	if backtest.Status == "running" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete running backtest"})
		return
	}

	if err := h.repo.DeleteBacktest(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete backtest"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest deleted successfully",
		"data": backtest,
	})
}

// CancelBacktest cancels a running backtest
func (h *BacktestHandler) CancelBacktest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backtest ID is required"})
		return
	}

	// Get existing backtest
	backtest, err := h.repo.GetBacktestByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backtest not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve backtest"})
		return
	}

	// Check if backtest can be cancelled
	if backtest.Status != "running" && backtest.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backtest cannot be cancelled"})
		return
	}

	// Update status to cancelled
	if err := h.repo.UpdateBacktestStatus(c.Request.Context(), id, "cancelled", backtest.Progress, backtest.Results, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel backtest"})
		return
	}

	// The bot checks the status before running and does not overwrite a
	// cancelled backtest with its result.

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest cancelled successfully",
		"data": gin.H{"id": id, "status": "cancelled"},
	})
}