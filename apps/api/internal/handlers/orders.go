package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
)

// OrderHandler handles order-related HTTP requests
type OrderHandler struct {
	repo *database.OrderRepository
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(repo *database.OrderRepository) *OrderHandler {
	return &OrderHandler{repo: repo}
}

// GetOrders retrieves orders with filtering
func (h *OrderHandler) GetOrders(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	strategyID := c.Query("strategy_id")
	symbol := c.Query("symbol")
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

	var strategyIDPtr, symbolPtr, statusPtr *string
	if strategyID != "" {
		strategyIDPtr = &strategyID
	}
	if symbol != "" {
		symbolPtr = &symbol
	}
	if status != "" {
		statusPtr = &status
	}

	orders, err := h.repo.ListOrders(c.Request.Context(), strategyIDPtr, symbolPtr, statusPtr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": orders,
		"limit": limit,
		"offset": offset,
	})
}

// CreateOrder creates a new order
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req struct {
		ClientOrderID string   `json:"client_order_id" binding:"required"`
		StrategyID    *string  `json:"strategy_id"`
		Symbol        string   `json:"symbol" binding:"required"`
		Side          string   `json:"side" binding:"required"`
		OrderType     string   `json:"order_type" binding:"required"`
		Quantity      float64  `json:"quantity" binding:"required"`
		Price         *float64 `json:"price"`
		StopPrice     *float64 `json:"stop_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if order with same client_order_id already exists (idempotency)
	existingOrder, err := h.repo.GetOrderByClientOrderID(c.Request.Context(), req.ClientOrderID)
	if err == nil && existingOrder != nil {
		c.JSON(http.StatusOK, gin.H{"data": existingOrder, "message": "Order already exists"})
		return
	}

	order := &database.Order{
		ClientOrderID: req.ClientOrderID,
		StrategyID:    req.StrategyID,
		Symbol:        req.Symbol,
		Side:          req.Side,
		OrderType:     req.OrderType,
		Quantity:      req.Quantity,
		Price:         req.Price,
		StopPrice:     req.StopPrice,
		Status:        "pending",
		FilledQuantity: 0,
		Commission:    0,
	}

	if err := h.repo.CreateOrder(c.Request.Context(), order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": order})
}

// GetOrder retrieves an order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	order, err := h.repo.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve order"})
		return
	}

	// Get trades for this order
	trades, err := h.repo.GetTradesByOrderID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve order trades"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"order": order,
			"trades": trades,
		},
	})
}

// UpdateOrder updates an order
func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	var req struct {
		Status         string   `json:"status"`
		FilledQuantity *float64 `json:"filled_quantity"`
		AvgFillPrice   *float64 `json:"avg_fill_price"`
		Commission     *float64 `json:"commission"`
		BrokerOrderID  *string  `json:"broker_order_id"`
		ErrorMessage   *string  `json:"error_message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing order
	order, err := h.repo.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve order"})
		return
	}

	// Update fields if provided
	if req.Status != "" {
		order.Status = req.Status
	}
	if req.FilledQuantity != nil {
		order.FilledQuantity = *req.FilledQuantity
	}
	if req.AvgFillPrice != nil {
		order.AvgFillPrice = req.AvgFillPrice
	}
	if req.Commission != nil {
		order.Commission = *req.Commission
	}
	if req.BrokerOrderID != nil {
		order.BrokerOrderID = req.BrokerOrderID
	}
	if req.ErrorMessage != nil {
		order.ErrorMessage = req.ErrorMessage
	}

	if err := h.repo.UpdateOrder(c.Request.Context(), order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

// CancelOrder cancels an order
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID is required"})
		return
	}

	// Get existing order
	order, err := h.repo.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve order"})
		return
	}

	// Check if order can be cancelled
	if order.Status == "filled" || order.Status == "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
		return
	}

	order.Status = "cancelled"
	if err := h.repo.UpdateOrder(c.Request.Context(), order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order, "message": "Order cancelled successfully"})
}

// GetTrades retrieves trades with filtering
func (h *OrderHandler) GetTrades(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	strategyID := c.Query("strategy_id")
	symbol := c.Query("symbol")

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

	var strategyIDPtr, symbolPtr *string
	if strategyID != "" {
		strategyIDPtr = &strategyID
	}
	if symbol != "" {
		symbolPtr = &symbol
	}

	trades, err := h.repo.ListTrades(c.Request.Context(), strategyIDPtr, symbolPtr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve trades"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": trades,
		"limit": limit,
		"offset": offset,
	})
}