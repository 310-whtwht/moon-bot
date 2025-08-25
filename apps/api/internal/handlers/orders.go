package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetOrders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Get orders endpoint - TODO: implement",
	})
}

func CreateOrder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Create order endpoint - TODO: implement",
	})
}

func GetOrder(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Get order endpoint - TODO: implement",
		"id": id,
	})
}

func UpdateOrder(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Update order endpoint - TODO: implement",
		"id": id,
	})
}

func CancelOrder(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Cancel order endpoint - TODO: implement",
		"id": id,
	})
}