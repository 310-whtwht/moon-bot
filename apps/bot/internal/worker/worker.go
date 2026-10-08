package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moomoo-trading/bot/internal/config"
	"github.com/moomoo-trading/bot/internal/imports"
	"github.com/moomoo-trading/bot/internal/jobs"
	"github.com/moomoo-trading/bot/internal/trader"
	"github.com/moomoo-trading/core/backtest"
	"github.com/moomoo-trading/core/broker"
	"github.com/moomoo-trading/core/broker/gmofx"
	"github.com/moomoo-trading/core/broker/paper"
	"github.com/moomoo-trading/core/marketdata"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Worker runs the bot's background loops: the backtest job consumer and the
// trader (strategy execution on the paper broker).
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

	// Price-history downloads requested from the UI, one at a time and at a
	// gentle pace: they share the broker's rate limit with live trading.
	importer := &imports.Runner{
		Store: &imports.MySQLStore{DB: db},
		Source: gmofx.New(gmofx.Options{
			PublicURL: w.config.GMO.PublicURL, PublicWSURL: w.config.GMO.PublicWSURL,
			MinInterval: 400 * time.Millisecond,
		}),
		Bars: marketdata.NewMySQLBarStore(db),
		Logf: log.Printf,
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		_ = importer.Run(runCtx)
	}()

	if w.config.Trader.Enabled {
		if err := w.startTrader(runCtx); err != nil {
			cancel()
			return err
		}
	} else {
		log.Println("Trader disabled (TRADER_ENABLED=false)")
	}
	return nil
}

// startTrader runs deployments on the paper broker, filled against real GMO
// quotes. Real-money brokers are added in Phase 4.
func (w *Worker) startTrader(ctx context.Context) error {
	tc := w.config.Trader
	source := gmofx.New(gmofx.Options{PublicURL: w.config.GMO.PublicURL, PublicWSURL: w.config.GMO.PublicWSURL})
	initial := decimal.NewFromFloat(tc.PaperInitialBalance)
	paperBroker := paper.New(source, paper.Options{
		InitialBalance: initial,
		FeeRate:        decimal.NewFromFloat(backtest.DefaultFeeRate),
	})

	store := &trader.MySQLStore{DB: w.db}
	if err := trader.RestorePaper(ctx, store, paperBroker, initial); err != nil {
		return err
	}

	var notifier trader.Notifier = trader.NoNotify{}
	if tc.SlackWebhookURL != "" {
		notifier = trader.NewSlackNotifier(ctx, tc.SlackWebhookURL, log.Printf)
		log.Println("Slack notifications enabled")
	}
	if tc.KillSwitch {
		log.Println("KILL_SWITCH is set: new entries are blocked")
	}

	brokers := map[string]broker.Broker{paper.BrokerName: paperBroker}
	if live, why := w.config.LiveTrading(); live {
		gmo, err := gmofx.NewPrivate(gmofx.PrivateOptions{
			Options:      gmofx.Options{PublicURL: w.config.GMO.PublicURL, PublicWSURL: w.config.GMO.PublicWSURL},
			APIKey:       w.config.GMO.APIKey,
			APISecret:    w.config.GMO.APISecret,
			PrivateURL:   w.config.GMO.PrivateURL,
			PrivateWSURL: w.config.GMO.PrivateWSURL,
			AccountID:    w.config.GMO.AccountID,
		})
		if err != nil {
			return err
		}
		// Fail at start-up, not at the first order, if the key does not work.
		assets, err := gmo.Assets(ctx)
		if err != nil {
			return fmt.Errorf("live trading: cannot read GMO account: %w", err)
		}
		brokers[gmofx.BrokerName] = gmo
		log.Printf("LIVE TRADING ENABLED: GMO account %q connected (available margin %s JPY, max %d units per order)",
			w.config.GMO.AccountID, assets.AvailableMargin.StringFixed(0), trader.HardMaxLiveUnits)
	} else {
		log.Printf("Live trading off (%s): only the paper broker is available", why)
	}

	host, _ := os.Hostname()
	manager := &trader.Manager{
		Store:        store,
		Brokers:      brokers,
		Guard:        trader.SwitchGuard{Store: store, EnvActive: tc.KillSwitch},
		Notifier:     notifier,
		Kills:        store,
		Ops:          store,
		Instance:     host,
		PollInterval: tc.PollInterval,
		Config: trader.Config{
			AccountLimits: tc.AccountLimits, GlobalLimits: tc.GlobalLimits,
			ReconcileInterval: tc.ReconcileInterval,
		},
	}
	quotes := trader.StreamQuotes(ctx, source, store, tc.PollInterval, log.Printf)

	// Real brokers push fills over a private WebSocket; use them to reconcile
	// immediately. (The paper broker's fills are always the bot's own.)
	for name, br := range brokers {
		if name == paper.BrokerName {
			continue
		}
		events, err := br.SubscribeExecutions(ctx)
		if err != nil {
			log.Printf("%s: no execution stream (%v); relying on periodic reconciliation", name, err)
			continue
		}
		w.wg.Add(1)
		go func(name string, events <-chan broker.Execution) {
			defer w.wg.Done()
			manager.WatchExecutions(ctx, name, events)
		}(name, events)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := manager.Run(ctx, tc.PollInterval, quotes); err != nil && ctx.Err() == nil {
			log.Printf("trader stopped: %v", err)
		}
	}()
	log.Printf("Trader started (paper, poll every %s, initial balance %.0f JPY)", tc.PollInterval, tc.PaperInitialBalance)
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
