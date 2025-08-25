package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetUniverse(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Get universe endpoint - TODO: implement",
	})
}

func AddSymbol(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Add symbol endpoint - TODO: implement",
	})
}

func RemoveSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	c.JSON(http.StatusOK, gin.H{
		"message": "Remove symbol endpoint - TODO: implement",
		"symbol": symbol,
	})
}