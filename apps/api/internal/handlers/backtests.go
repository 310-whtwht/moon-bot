package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
)

// BacktestHandler handles backtest-related HTTP requests
type BacktestHandler struct {
	repo *database.BacktestRepository
}

// NewBacktestHandler creates a new backtest handler
func NewBacktestHandler(repo *database.BacktestRepository) *BacktestHandler {
	return &BacktestHandler{repo: repo}
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

// CreateBacktest creates a new backtest
func (h *BacktestHandler) CreateBacktest(c *gin.Context) {
	var req struct {
		Name       string          `json:"name" binding:"required"`
		StrategyID string          `json:"strategy_id" binding:"required"`
		Symbols    []string        `json:"symbols" binding:"required"`
		StartDate  string          `json:"start_date" binding:"required"`
		EndDate    string          `json:"end_date" binding:"required"`
		Parameters json.RawMessage `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Parse dates
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

	backtest := &database.Backtest{
		Name:       req.Name,
		StrategyID: req.StrategyID,
		Symbols:    req.Symbols,
		StartDate:  startDate,
		EndDate:    endDate,
		Parameters: req.Parameters,
	}

	if err := h.repo.CreateBacktest(c.Request.Context(), backtest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backtest"})
		return
	}

	// TODO: Start backtest execution in background
	// This would typically be done by sending a message to a job queue

	c.JSON(http.StatusCreated, gin.H{"data": backtest})
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

	// TODO: Cancel actual backtest execution

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest cancelled successfully",
		"data": gin.H{"id": id, "status": "cancelled"},
	})
}