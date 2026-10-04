package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/moomoo-trading/bot/internal/backfill"
	"github.com/moomoo-trading/bot/internal/backtestcmd"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/killcmd"
	"github.com/moomoo-trading/bot/internal/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Subcommands run once and exit:
	//   bot backfill [flags]  download historical bars
	//   bot backtest [flags]  run a strategy over stored bars
	//   bot kill status|on|off  show or change the kill switches
	if len(os.Args) > 1 {
		commands := map[string]func(context.Context, *config.Config, []string, io.Writer) error{
			"backfill": backfill.Run,
			"backtest": backtestcmd.Run,
			"kill":     killcmd.Run,
		}
		if run, ok := commands[os.Args[1]]; ok {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			if err := run(ctx, cfg, os.Args[2:], os.Stdout); err != nil {
				log.Fatalf("%s failed: %v", os.Args[1], err)
			}
			return
		}
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