package config

import (
	"fmt"
	"os"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	GMO      GMOConfig
	Worker   WorkerConfig
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

type WorkerConfig struct {
	Concurrency int
	PollInterval int // seconds
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
		Worker: WorkerConfig{
			Concurrency:  5,
			PollInterval: 30,
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