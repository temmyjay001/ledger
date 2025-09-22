package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/subosito/gotenv"
)

type Config struct {
	Host string
	Port string
	Env  string

	DatabaseURL            string
	DatabaseMaxConnections int
	DatabaseMaxIdleTime    time.Duration

	RedisURL string

	JWTSecret    string
	APIKeySecret string

	WebhookTimeout    time.Duration
	WebhookMaxRetries int
}

func Load() (*Config, error) {
	// Load .env file if exists (ignore error in production)
	_ = gotenv.Load()

	cfg := &Config{
		Host: getEnvString("HOST", "localhost"),
		Port: getEnvString("PORT", "8080"),
		Env:  getEnvString("ENV", "development"),

		DatabaseURL:            getEnvString("DATABASE_URL", "postgres://localhost/ledger_dev?sslmode=disable"),
		DatabaseMaxConnections: getEnvInt("DATABASE_MAX_CONNECTIONS", 25),
		DatabaseMaxIdleTime:    getEnvDuration("DATABASE_MAX_IDLE_TIME", 15*time.Minute),

		RedisURL: getEnvString("REDIS_URL", "redis://localhost:6379/0"),

		JWTSecret:    getEnvString("JWT_SECRET", ""),
		APIKeySecret: getEnvString("API_KEY_SECRET", ""),

		WebhookTimeout:    getEnvDuration("WEBHOOK_TIMEOUT", 30*time.Second),
		WebhookMaxRetries: getEnvInt("WEBHOOK_MAX_RETRIES", 3),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	if cfg.APIKeySecret == "" {
		return nil, fmt.Errorf("API_KEY_SECRET is required")
	}

	return cfg, nil
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if durationValue, err := time.ParseDuration(value); err == nil {
			return durationValue
		}
	}
	return defaultValue
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
