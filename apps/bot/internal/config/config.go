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
	PaperInitialBalance float64
	AccountLimits       risk.Limits
	GlobalLimits        risk.Limits
	// KillSwitch (KILL_SWITCH=true) blocks all new entries regardless of the database.
	KillSwitch bool
	// SlackWebhookURL enables Slack notifications when set.
	SlackWebhookURL string
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
		},
		Trader: TraderConfig{
			Enabled:             getEnv("TRADER_ENABLED", "true") == "true",
			PollInterval:        getDuration("TRADER_POLL_INTERVAL", 30*time.Second),
			PaperInitialBalance: getFloat("PAPER_INITIAL_BALANCE", 30000),
			KillSwitch:          getEnv("KILL_SWITCH", "false") == "true",
			SlackWebhookURL:     getEnv("SLACK_WEBHOOK_URL", ""),
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
