package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/moomoo-trading/core/risk"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	GMO      GMOConfig
	Trader   TraderConfig
}

// TraderConfig controls live strategy execution (paper trading in Phase 3).
type TraderConfig struct {
	Enabled             bool
	PollInterval        time.Duration
	// ReconcileInterval is how often positions are compared with the broker.
	ReconcileInterval time.Duration
	PaperInitialBalance float64
	AccountLimits       risk.Limits
	GlobalLimits        risk.Limits
	// KillSwitch (KILL_SWITCH=true) blocks all new entries regardless of the database.
	KillSwitch bool
	// SlackWebhookURL enables Slack notifications when set.
	SlackWebhookURL string
	// BrokerMode is "paper" (default) or "live". LiveConfirm must be "yes" as
	// well before any real-money broker is connected.
	BrokerMode  string
	LiveConfirm string
}

// LiveTrading reports whether the real-money broker may be connected, and if
// not, why. Two separate switches plus credentials are required so that a
// single stray variable cannot start live trading.
func (c *Config) LiveTrading() (bool, string) {
	switch {
	case c.Trader.BrokerMode != "live":
		return false, "BROKER_MODE is not \"live\""
	case c.Trader.LiveConfirm != "yes":
		return false, "BROKER_MODE=live but LIVE_CONFIRM is not \"yes\""
	case c.GMO.APIKey == "" || c.GMO.APISecret == "":
		return false, "BROKER_MODE=live but GMO_API_KEY / GMO_API_SECRET are not set"
	}
	return true, ""
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type GMOConfig struct {
	PublicURL   string
	PublicWSURL string
	PrivateURL  string
	// APIKey / APISecret are only read from the environment; never log them.
	APIKey    string
	APISecret string
	// AccountID labels the account on orders and positions.
	AccountID string
}


func Load() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "moomoo"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "moomoo_trading"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		GMO: GMOConfig{
			PublicURL:   getEnv("GMO_PUBLIC_URL", ""),
			PublicWSURL: getEnv("GMO_PUBLIC_WS_URL", ""),
			PrivateURL:  getEnv("GMO_PRIVATE_URL", ""),
			APIKey:      getEnv("GMO_API_KEY", ""),
			APISecret:   getEnv("GMO_API_SECRET", ""),
			AccountID:   getEnv("GMO_ACCOUNT_ID", "default"),
		},
		Trader: TraderConfig{
			Enabled:             getEnv("TRADER_ENABLED", "true") == "true",
			PollInterval:        getDuration("TRADER_POLL_INTERVAL", 30*time.Second),
			ReconcileInterval:   getDuration("TRADER_RECONCILE_INTERVAL", 5*time.Minute),
			PaperInitialBalance: getFloat("PAPER_INITIAL_BALANCE", 30000),
			KillSwitch:          getEnv("KILL_SWITCH", "false") == "true",
			SlackWebhookURL:     getEnv("SLACK_WEBHOOK_URL", ""),
			BrokerMode:          getEnv("BROKER_MODE", "paper"),
			LiveConfirm:         getEnv("LIVE_CONFIRM", ""),
			// Defaults are deliberately small (Phase 6 starts at 100 units).
			AccountLimits: risk.Limits{
				MaxUnitsPerPosition: getFloat("RISK_ACCOUNT_MAX_UNITS", 1000),
				MaxOpenPositions:    int(getFloat("RISK_ACCOUNT_MAX_POSITIONS", 1)),
				MaxDailyLossJPY:     getFloat("RISK_ACCOUNT_MAX_DAILY_LOSS", 500),
				MaxWeeklyLossJPY:    getFloat("RISK_ACCOUNT_MAX_WEEKLY_LOSS", 1500),
			},
			GlobalLimits: risk.Limits{
				MaxOpenPositions: int(getFloat("RISK_GLOBAL_MAX_POSITIONS", 3)),
				MaxDailyLossJPY:  getFloat("RISK_GLOBAL_MAX_DAILY_LOSS", 1000),
				MaxWeeklyLossJPY: getFloat("RISK_GLOBAL_MAX_WEEKLY_LOSS", 3000),
			},
		},
	}
}

// DSN returns the MySQL DSN. Times are read and written in UTC.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC&time_zone=%%27%%2B00%%3A00%%27&allowNativePasswords=true",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
// getFloat reads a number, falling back (with a warning) on bad input so a
// typo cannot silently disable a risk limit by parsing to zero.
func getFloat(key string, defaultValue float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		log.Printf("config: invalid %s=%q, using default %v", key, raw, defaultValue)
		return defaultValue
	}
	return v
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		log.Printf("config: invalid %s=%q, using default %s", key, raw, defaultValue)
		return defaultValue
	}
	return v
}
