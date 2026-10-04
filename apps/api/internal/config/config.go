package config

import (
	"os"
)

type Config struct {
	Environment string
	AutoMigrate bool
	// APIToken is the bearer token every /api/v1 request must carry.
	APIToken string
	Database    DatabaseConfig
	Redis       RedisConfig
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


func Load() *Config {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		AutoMigrate: getEnv("AUTO_MIGRATE", "false") == "true",
		APIToken:    getEnv("API_TOKEN", ""),
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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}