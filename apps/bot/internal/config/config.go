package config

import (
	"os"
)

type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Moomoo   MoomooConfig
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

type MoomooConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	AppID    string
	AppKey   string
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
			Password: getEnv("DB_PASSWORD", "moomoo123"),
			Database: getEnv("DB_NAME", "moomoo_trading"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		Moomoo: MoomooConfig{
			Host:     getEnv("MOOMOO_HOST", "localhost"),
			Port:     11111,
			Username: getEnv("MOOMOO_USERNAME", ""),
			Password: getEnv("MOOMOO_PASSWORD", ""),
			AppID:    getEnv("MOOMOO_APP_ID", ""),
			AppKey:   getEnv("MOOMOO_APP_KEY", ""),
		},
		Worker: WorkerConfig{
			Concurrency:  5,
			PollInterval: 30,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}