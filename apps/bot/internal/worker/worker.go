package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/jobs"
	"github.com/moomoo-trading/core/marketdata"
	"github.com/redis/go-redis/v9"
)

// Worker runs the bot's background loops. Today: the backtest job consumer.
// Live strategy execution is added in Phase 3.
type Worker struct {
	config *config.Config
	db     *sql.DB
	redis  *redis.Client
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg *config.Config) *Worker {
	return &Worker{config: cfg}
}

func (w *Worker) Start(ctx context.Context) error {
	log.Println("Starting worker...")

	db, err := sql.Open("mysql", w.config.Database.DSN())
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	w.db = db

	w.redis = redis.NewClient(&redis.Options{
		Addr:     w.config.Redis.Host + ":" + w.config.Redis.Port,
		Password: w.config.Redis.Password,
		DB:       w.config.Redis.DB,
	})
	if err := w.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}

	runner := &jobs.BacktestRunner{
		Store: &jobs.MySQLBacktestStore{DB: db},
		Bars:  marketdata.NewMySQLBarStore(db),
	}
	host, _ := os.Hostname()
	consumer := &jobs.Consumer{
		Client: w.redis,
		Name:   fmt.Sprintf("%s-%d", host, os.Getpid()),
		Handle: runner.Handle,
	}

	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := consumer.Run(runCtx); err != nil && runCtx.Err() == nil {
			log.Printf("backtest consumer stopped: %v", err)
		}
	}()

	log.Println("Worker started (backtest jobs)")
	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	log.Println("Stopping worker...")
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	if w.redis != nil {
		_ = w.redis.Close()
	}
	if w.db != nil {
		_ = w.db.Close()
	}
	return nil
}
