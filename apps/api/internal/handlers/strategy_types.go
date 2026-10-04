package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/core/strategy"
)

// GetStrategyTypes lists the registered strategy types and their parameter
// specs, used by the UI to build parameter forms.
func GetStrategyTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": strategy.Definitions()})
}

// strategyTypeOf returns the registered strategy type stored in a version's
// code, or "" for legacy versions.
func strategyTypeOf(code string) string {
	if _, err := strategy.Lookup(code); err != nil {
		return ""
	}
	return code
}
