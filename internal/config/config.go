package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds runtime configuration values loaded from config.json.
type Config struct {
	Port        string        `json:"port"`
	DatabaseURL string        `json:"database_url"`
	Media       MediaConfig   `json:"media"`
	Geodata     GeodataConfig `json:"geodata"`
	AI          AIConfig      `json:"ai"`
}

// MediaConfig describes S3/media related configuration.
type MediaConfig struct {
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	Endpoint       string `json:"endpoint"`
	PublicURL      string `json:"public_url"`
	KeyPrefix      string `json:"key_prefix"`
	ForcePathStyle bool   `json:"force_path_style"`
}

// GeodataConfig bundles relevant API keys.
type GeodataConfig struct {
	GooglePlacesAPIKey string `json:"google_places_api_key"`
	TrafficAPIKey      string `json:"trafiklab_api_key"`
	CacheTTLMinutes    int    `json:"cache_ttl_minutes"`
}

// AIConfig selects which LLM provider to use.
type AIConfig struct {
	Provider string       `json:"provider"`
	Gemini   GeminiConfig `json:"gemini"`
}

// GeminiConfig holds Google Generative Language credentials.
type GeminiConfig struct {
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	VisionModel    string `json:"vision_model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Load reads configuration from the provided JSON file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.Geodata.CacheTTLMinutes == 0 {
		cfg.Geodata.CacheTTLMinutes = 30
	}
	if cfg.AI.Gemini.Model == "" {
		cfg.AI.Gemini.Model = "gemini-1.5-pro-latest"
	}
	if cfg.AI.Gemini.VisionModel == "" {
		cfg.AI.Gemini.VisionModel = "gemini-1.5-flash-latest"
	}
	if cfg.AI.Gemini.TimeoutSeconds <= 0 {
		cfg.AI.Gemini.TimeoutSeconds = 60
	}
}
