package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/core/risk"
)

// BotHandler serves the bot's state and its controls (kill switch, deployments).
type BotHandler struct {
	repo *database.BotRepository
	now  func() time.Time
}

func NewBotHandler(repo *database.BotRepository) *BotHandler {
	return &BotHandler{repo: repo, now: time.Now}
}

// killScopes are the scopes a kill switch may have: everything, or one broker.
var killScopes = map[string]bool{"global": true, "paper": true, "gmo": true}

type pnlSummary struct {
	Since  time.Time `json:"since"`
	PnL    float64   `json:"pnl"`
	Closed int       `json:"closed"`
}

// GetStatus returns everything the dashboard shows in one call.
func (h *BotHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	now := h.now().UTC()
	fail := func(what string) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load " + what})
	}

	heartbeats, err := h.repo.Heartbeats(ctx, now)
	if err != nil {
		fail("bot heartbeats")
		return
	}
	switches, err := h.repo.KillSwitches(ctx)
	if err != nil {
		fail("kill switches")
		return
	}
	deployments, err := h.repo.Deployments(ctx)
	if err != nil {
		fail("deployments")
		return
	}
	open, err := h.repo.Positions(ctx, "open", 50)
	if err != nil {
		fail("open positions")
		return
	}
	closed, err := h.repo.Positions(ctx, "closed", 10)
	if err != nil {
		fail("closed positions")
		return
	}

	daily := pnlSummary{Since: risk.TradingDayStart(now)}
	if daily.PnL, daily.Closed, err = h.repo.RealizedPnL(ctx, daily.Since); err != nil {
		fail("daily P&L")
		return
	}
	weekly := pnlSummary{Since: risk.TradingWeekStart(now)}
	if weekly.PnL, weekly.Closed, err = h.repo.RealizedPnL(ctx, weekly.Since); err != nil {
		fail("weekly P&L")
		return
	}

	alive := false
	for _, hb := range heartbeats {
		alive = alive || hb.Alive
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"now":              now,
		"bot_alive":        alive,
		"heartbeats":       heartbeats,
		"kill_switches":    switches,
		"deployments":      deployments,
		"open_positions":   open,
		"closed_positions": closed,
		"daily":            daily,
		"weekly":           weekly,
	}})
}

// SetKillSwitch turns a kill switch on or off.
func (h *BotHandler) SetKillSwitch(c *gin.Context) {
	scope := c.Param("scope")
	if !killScopes[scope] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown scope (use global, paper or gmo)"})
		return
	}

	var req struct {
		Active         *bool   `json:"active" binding:"required"`
		ClosePositions bool    `json:"close_positions"`
		Reason         *string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: active is required"})
		return
	}

	by := "web"
	k := database.KillSwitch{Scope: scope, Active: *req.Active, ClosePositions: req.ClosePositions, Reason: req.Reason, UpdatedBy: &by}
	if err := h.repo.SetKillSwitch(c.Request.Context(), k); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update kill switch"})
		return
	}
	switches, err := h.repo.KillSwitches(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load kill switches"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": switches})
}

// SetDeploymentEnabled enables or disables a deployment. Disabling does not
// close an open position: the bot keeps managing its exit and stop-loss.
func (h *BotHandler) SetDeploymentEnabled(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: enabled is required"})
		return
	}

	found, err := h.repo.SetDeploymentEnabled(c.Request.Context(), c.Param("id"), *req.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deployment"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": c.Param("id"), "enabled": *req.Enabled}})
}
