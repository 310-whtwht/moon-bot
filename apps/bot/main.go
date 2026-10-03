package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/moomoo-trading/bot/internal/backfill"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Subcommands: `bot backfill [flags]` downloads historical bars and exits.
	if len(os.Args) > 1 && os.Args[1] == "backfill" {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := backfill.Run(ctx, cfg, os.Args[2:], os.Stdout); err != nil {
			log.Fatalf("backfill failed: %v", err)
		}
		return
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create worker
	w := worker.New(cfg)

	// Start worker
	if err := w.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down worker...")

	// Graceful shutdown
	if err := w.Stop(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Worker stopped")
}