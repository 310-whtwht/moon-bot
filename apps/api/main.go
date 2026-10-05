package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/moomoo-trading/api/internal/audit"
	"github.com/moomoo-trading/api/internal/config"
	"github.com/moomoo-trading/api/internal/database"
	"github.com/moomoo-trading/api/internal/handlers"
	"github.com/moomoo-trading/api/internal/middleware"
	"github.com/moomoo-trading/api/internal/redis"
	"github.com/moomoo-trading/core/broker/gmofx"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Apply pending migrations: `go run . migrate` runs them and exits,
	// AUTO_MIGRATE=true runs them before the server starts.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := database.RunMigrations(context.Background(), cfg.Database); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations applied")
		return
	}
	if cfg.AutoMigrate {
		if err := database.RunMigrations(context.Background(), cfg.Database); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}

	// Initialize database connection
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize GORM connection for audit
	gormDB, err := database.NewGormConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database with GORM: %v", err)
	}

	// Initialize Redis connection
	redisClient, err := database.NewRedisConnection(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize repositories
	strategyRepo := database.NewStrategyRepository(db)
	orderRepo := database.NewOrderRepository(db)
	universeRepo := database.NewUniverseRepository(db)
	backtestRepo := database.NewBacktestRepository(db)

	// Initialize handlers
	strategyHandler := handlers.NewStrategyHandler(strategyRepo)
	orderHandler := handlers.NewOrderHandler(orderRepo)
	universeHandler := handlers.NewUniverseHandler(universeRepo)
	streamManager := redis.NewStreamManager(redisClient)
	backtestHandler := handlers.NewBacktestHandler(backtestRepo, strategyRepo, streamManager)
	botRepo := database.NewBotRepository(db)
	botHandler := handlers.NewBotHandler(botRepo)
	// The chart reads bars from GMO's public API (no key needed).
	chartHandler := handlers.NewChartHandler(botRepo, gmofx.New(gmofx.Options{}))
	auditHandler := handlers.NewAuditHandler(audit.NewTraceManager(gormDB, redisClient))

	// Initialize Redis Streams
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

	// API routes. Every route needs the bearer token; without one configured
	// the API only runs in development (it controls orders and the kill switch).
	api := r.Group("/api/v1")
	switch {
	case cfg.APIToken != "":
		api.Use(middleware.RequireToken(cfg.APIToken))
	case cfg.Environment == "production":
		log.Fatal("API_TOKEN is required when ENVIRONMENT=production")
	default:
		log.Println("WARNING: API_TOKEN is not set; /api/v1 is open to anyone who can reach this port (development only)")
	}
	{
		// Orders
		orders := api.Group("/orders")
		{
			orders.GET("", orderHandler.GetOrders)
			orders.POST("", orderHandler.CreateOrder)
			orders.GET("/:id", orderHandler.GetOrder)
			orders.PUT("/:id", orderHandler.UpdateOrder)
			orders.DELETE("/:id", orderHandler.CancelOrder)
		}

		// Trades
		trades := api.Group("/trades")
		{
			trades.GET("", orderHandler.GetTrades)
		}

		// Bot state and controls
		api.GET("/bot/status", botHandler.GetStatus)
		api.PUT("/kill-switch/:scope", botHandler.SetKillSwitch)
		api.PUT("/deployments/:id", botHandler.SetDeploymentEnabled)
		api.GET("/chart", chartHandler.GetChart)

		// Strategy types (registered in packages/core/strategy)
		api.GET("/strategy-types", handlers.GetStrategyTypes)

		// Strategies
		strategies := api.Group("/strategies")
		{
			strategies.GET("", strategyHandler.GetStrategies)
			strategies.POST("", strategyHandler.CreateStrategy)
			strategies.GET("/:id", strategyHandler.GetStrategy)
			strategies.PUT("/:id", strategyHandler.UpdateStrategy)
			strategies.DELETE("/:id", strategyHandler.DeleteStrategy)
			strategies.GET("/:id/versions", strategyHandler.GetStrategyVersions)
			strategies.POST("/:id/versions", strategyHandler.CreateStrategyVersion)
		}

		// Backtests
		backtests := api.Group("/backtests")
		{
			backtests.GET("", backtestHandler.GetBacktests)
			backtests.POST("", backtestHandler.CreateBacktest)
			backtests.GET("/:id", backtestHandler.GetBacktest)
			backtests.DELETE("/:id", backtestHandler.DeleteBacktest)
			backtests.POST("/:id/cancel", backtestHandler.CancelBacktest)
		}

		// Universe
		universe := api.Group("/universe")
		{
			universe.GET("", universeHandler.GetUniverse)
			universe.POST("", universeHandler.AddSymbol)
			universe.DELETE("/:symbol", universeHandler.RemoveSymbol)
			universe.POST("/bulk", universeHandler.BulkAddSymbols)
		}

		// Audit traces
		audit := api.Group("/audit/traces")
		{
			audit.POST("", auditHandler.CreateTrace)
			audit.GET("/:id", auditHandler.GetTraceByID)
			audit.GET("/:id/chain", auditHandler.GetTraceChain)
			audit.GET("/strategy/:strategy_id", auditHandler.GetTracesByStrategy)
			audit.GET("/symbol/:symbol", auditHandler.GetTracesBySymbol)
			audit.GET("/strategy/:strategy_id/statistics", auditHandler.GetTraceStatistics)
			audit.POST("/search", auditHandler.SearchTraces)
			audit.POST("/export", auditHandler.ExportTraces)
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