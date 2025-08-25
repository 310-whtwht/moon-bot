package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/config"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/api/internal/handlers"
	"github.com/moomoo-trading/api/internal/middleware"
	"github.com/moomoo-trading/api/internal/redis"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database connection
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis connection
	redis, err := database.NewRedisConnection(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Initialize Redis Streams
	streamManager := redis.NewStreamManager(redis)
	if err := streamManager.InitializeStreams(context.Background()); err != nil {
		log.Fatalf("Failed to initialize Redis streams: %v", err)
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	r := gin.Default()

	// Add middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	// Health check endpoint
	r.GET("/healthz", handlers.HealthCheck)

	// API routes
	api := r.Group("/api/v1")
	{
		// Orders
		orders := api.Group("/orders")
		{
			orders.GET("/", handlers.GetOrders)
			orders.POST("/", handlers.CreateOrder)
			orders.GET("/:id", handlers.GetOrder)
			orders.PUT("/:id", handlers.UpdateOrder)
			orders.DELETE("/:id", handlers.CancelOrder)
		}

		// Strategies
		strategies := api.Group("/strategies")
		{
			strategies.GET("/", handlers.GetStrategies)
			strategies.POST("/", handlers.CreateStrategy)
			strategies.GET("/:id", handlers.GetStrategy)
			strategies.PUT("/:id", handlers.UpdateStrategy)
			strategies.DELETE("/:id", handlers.DeleteStrategy)
		}

		// Backtests
		backtests := api.Group("/backtests")
		{
			backtests.GET("/", handlers.GetBacktests)
			backtests.POST("/", handlers.CreateBacktest)
			backtests.GET("/:id", handlers.GetBacktest)
			backtests.DELETE("/:id", handlers.DeleteBacktest)
		}

		// Universe
		universe := api.Group("/universe")
		{
			universe.GET("/", handlers.GetUniverse)
			universe.POST("/", handlers.AddSymbol)
			universe.DELETE("/:symbol", handlers.RemoveSymbol)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}