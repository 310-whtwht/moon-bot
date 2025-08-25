package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetBacktests(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Get backtests endpoint - TODO: implement",
	})
}

func CreateBacktest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Create backtest endpoint - TODO: implement",
	})
}

func GetBacktest(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Get backtest endpoint - TODO: implement",
		"id": id,
	})
}

func DeleteBacktest(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Delete backtest endpoint - TODO: implement",
		"id": id,
	})
}