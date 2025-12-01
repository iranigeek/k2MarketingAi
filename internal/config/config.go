package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration values.
type Config struct {
	Port        string
	DatabaseURL string
	Media       MediaConfig
	Geodata     GeodataConfig
}

// MediaConfig describes S3/media related configuration.
type MediaConfig struct {
	Bucket         string
	Region         string
	Endpoint       string
	PublicURL      string
	KeyPrefix      string
	ForcePathStyle bool
}

// GeodataConfig bundles relevant API keys.
type GeodataConfig struct {
	GooglePlacesAPIKey string
	TrafficAPIKey      string
}

// FromEnv loads configuration from environment variables and applies defaults.
func FromEnv() Config {
	cfg := Config{
		Port:        getenv("APP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Media: MediaConfig{
			Bucket:         os.Getenv("S3_BUCKET"),
			Region:         os.Getenv("S3_REGION"),
			Endpoint:       os.Getenv("S3_ENDPOINT"),
			PublicURL:      os.Getenv("S3_PUBLIC_URL"),
			KeyPrefix:      strings.Trim(os.Getenv("S3_KEY_PREFIX"), "/"),
			ForcePathStyle: getenvBool("S3_FORCE_PATH_STYLE", false),
		},
		Geodata: GeodataConfig{
			GooglePlacesAPIKey: os.Getenv("GOOGLE_PLACES_API_KEY"),
			TrafficAPIKey:      os.Getenv("TRAFIKLAB_API_KEY"),
		},
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

func getenvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}

	return parsed
}
