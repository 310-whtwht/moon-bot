package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
)

// StrategyHandler handles strategy-related HTTP requests
type StrategyHandler struct {
	repo *database.StrategyRepository
}

// NewStrategyHandler creates a new strategy handler
func NewStrategyHandler(repo *database.StrategyRepository) *StrategyHandler {
	return &StrategyHandler{repo: repo}
}

// GetStrategies retrieves all strategy packages
func (h *StrategyHandler) GetStrategies(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

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

	packages, err := h.repo.ListPackages(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategies"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": packages,
		"limit": limit,
		"offset": offset,
	})
}

// CreateStrategy creates a new strategy package
func (h *StrategyHandler) CreateStrategy(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
		Author      string  `json:"author" binding:"required"`
		IsPublic    bool    `json:"is_public"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	pkg := &database.StrategyPackage{
		Name:        req.Name,
		Description: req.Description,
		Author:      req.Author,
		IsPublic:    req.IsPublic,
	}

	if err := h.repo.CreatePackage(c.Request.Context(), pkg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create strategy"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": pkg})
}

// GetStrategy retrieves a strategy package by ID
func (h *StrategyHandler) GetStrategy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy ID is required"})
		return
	}

	pkg, err := h.repo.GetPackageByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": pkg})
}

// UpdateStrategy updates a strategy package
func (h *StrategyHandler) UpdateStrategy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy ID is required"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Author      string  `json:"author"`
		IsPublic    *bool   `json:"is_public"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing package
	pkg, err := h.repo.GetPackageByID(c.Request.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategy"})
		return
	}

	// Update fields if provided
	if req.Name != "" {
		pkg.Name = req.Name
	}
	if req.Description != nil {
		pkg.Description = req.Description
	}
	if req.Author != "" {
		pkg.Author = req.Author
	}
	if req.IsPublic != nil {
		pkg.IsPublic = *req.IsPublic
	}

	if err := h.repo.UpdatePackage(c.Request.Context(), pkg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update strategy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": pkg})
}

// DeleteStrategy deletes a strategy package
func (h *StrategyHandler) DeleteStrategy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy ID is required"})
		return
	}

	if err := h.repo.DeletePackage(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete strategy"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Strategy deleted successfully"})
}

// GetStrategyVersions retrieves all versions of a strategy
func (h *StrategyHandler) GetStrategyVersions(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy ID is required"})
		return
	}

	versions, err := h.repo.ListVersionsByPackageID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve strategy versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": versions})
}

// CreateStrategyVersion creates a new strategy version
func (h *StrategyHandler) CreateStrategyVersion(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy ID is required"})
		return
	}

	var req struct {
		Version     string  `json:"version" binding:"required"`
		Code        string  `json:"code" binding:"required"`
		Description *string `json:"description"`
		IsActive    bool    `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	version := &database.StrategyVersion{
		PackageID:   id,
		Version:     req.Version,
		Code:        req.Code,
		Description: req.Description,
		IsActive:    req.IsActive,
	}

	if err := h.repo.CreateVersion(c.Request.Context(), version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create strategy version"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": version})
}