package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
)

// UniverseHandler handles universe-related HTTP requests
type UniverseHandler struct {
	repo *database.UniverseRepository
}

// NewUniverseHandler creates a new universe handler
func NewUniverseHandler(repo *database.UniverseRepository) *UniverseHandler {
	return &UniverseHandler{repo: repo}
}

// GetUniverse retrieves universe symbols with filtering
func (h *UniverseHandler) GetUniverse(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	exchange := c.Query("exchange")
	assetType := c.Query("asset_type")
	isActiveStr := c.Query("is_active")

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

	var exchangePtr, assetTypePtr *string
	var isActivePtr *bool

	if exchange != "" {
		exchangePtr = &exchange
	}
	if assetType != "" {
		assetTypePtr = &assetType
	}
	if isActiveStr != "" {
		isActive, err := strconv.ParseBool(isActiveStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid is_active parameter"})
			return
		}
		isActivePtr = &isActive
	}

	symbols, err := h.repo.ListSymbols(c.Request.Context(), exchangePtr, assetTypePtr, isActivePtr, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve universe symbols"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": symbols,
		"limit": limit,
		"offset": offset,
	})
}

// AddSymbol adds a new symbol to the universe
func (h *UniverseHandler) AddSymbol(c *gin.Context) {
	var req struct {
		Symbol     string  `json:"symbol" binding:"required"`
		Name       *string `json:"name"`
		Exchange   string  `json:"exchange" binding:"required"`
		AssetType  string  `json:"asset_type" binding:"required"`
		IsActive   bool    `json:"is_active"`
		DataSource string  `json:"data_source"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if symbol already exists
	existingSymbol, err := h.repo.GetSymbolBySymbol(c.Request.Context(), req.Symbol)
	if err == nil && existingSymbol != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Symbol already exists"})
		return
	}

	if req.DataSource == "" {
		req.DataSource = "moomoo"
	}

	symbol := &database.UniverseSymbol{
		Symbol:     req.Symbol,
		Name:       req.Name,
		Exchange:   req.Exchange,
		AssetType:  req.AssetType,
		IsActive:   req.IsActive,
		DataSource: req.DataSource,
	}

	if err := h.repo.CreateSymbol(c.Request.Context(), symbol); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add symbol"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": symbol})
}

// RemoveSymbol removes a symbol from the universe
func (h *UniverseHandler) RemoveSymbol(c *gin.Context) {
	symbolName := c.Param("symbol")
	if symbolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol is required"})
		return
	}

	// Check if symbol exists
	existingSymbol, err := h.repo.GetSymbolBySymbol(c.Request.Context(), symbolName)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Symbol not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve symbol"})
		return
	}

	if err := h.repo.DeleteSymbolBySymbol(c.Request.Context(), symbolName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove symbol"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Symbol removed successfully",
		"data": existingSymbol,
	})
}

// BulkAddSymbols adds multiple symbols to the universe
func (h *UniverseHandler) BulkAddSymbols(c *gin.Context) {
	var req struct {
		Symbols []struct {
			Symbol     string  `json:"symbol" binding:"required"`
			Name       *string `json:"name"`
			Exchange   string  `json:"exchange" binding:"required"`
			AssetType  string  `json:"asset_type" binding:"required"`
			IsActive   bool    `json:"is_active"`
			DataSource string  `json:"data_source"`
		} `json:"symbols" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var symbols []*database.UniverseSymbol
	for _, s := range req.Symbols {
		if s.DataSource == "" {
			s.DataSource = "moomoo"
		}

		symbol := &database.UniverseSymbol{
			Symbol:     s.Symbol,
			Name:       s.Name,
			Exchange:   s.Exchange,
			AssetType:  s.AssetType,
			IsActive:   s.IsActive,
			DataSource: s.DataSource,
		}
		symbols = append(symbols, symbol)
	}

	if err := h.repo.BulkUpsertSymbols(c.Request.Context(), symbols); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bulk add symbols"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Symbols added successfully",
		"count": len(symbols),
	})
}