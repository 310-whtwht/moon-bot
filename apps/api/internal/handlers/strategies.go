package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetStrategies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Get strategies endpoint - TODO: implement",
	})
}

func CreateStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Create strategy endpoint - TODO: implement",
	})
}

func GetStrategy(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Get strategy endpoint - TODO: implement",
		"id": id,
	})
}

func UpdateStrategy(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Update strategy endpoint - TODO: implement",
		"id": id,
	})
}

func DeleteStrategy(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Delete strategy endpoint - TODO: implement",
		"id": id,
	})
}