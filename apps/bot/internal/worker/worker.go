package worker

import (
	"context"
	"log"
	"time"

	"github.com/moomoo-trading/bot/internal/config"
)

type Worker struct {
	config *config.Config
	stop   chan struct{}
}

func New(cfg *config.Config) *Worker {
	return &Worker{
		config: cfg,
		stop:   make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	log.Println("Starting worker...")

	// Start main worker loop
	go w.run(ctx)

	log.Println("Worker started")
	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	log.Println("Stopping worker...")
	close(w.stop)
	return nil
}

func (w *Worker) run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(w.config.Worker.PollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker context cancelled")
			return
		case <-w.stop:
			log.Println("Worker stop signal received")
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	log.Println("Processing tasks...")
	// TODO: Implement actual task processing
	// - Poll Redis streams for new events
	// - Execute strategies
	// - Handle order management
}