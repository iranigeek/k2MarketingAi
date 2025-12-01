package config

import (
	"log"
	"os"
)

// Config holds runtime configuration values.
type Config struct {
	Port        string
	DatabaseURL string
}

// FromEnv loads configuration from environment variables and applies defaults.
func FromEnv() Config {
	cfg := Config{
		Port:        getenv("APP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.Port == "" {
		log.Fatal("APP_PORT cannot be empty")
	}

	return cfg
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
