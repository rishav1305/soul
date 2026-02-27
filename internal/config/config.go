package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	Port      int
	Host      string
	DevMode   bool
	DevUIAddr string
	APIKey    string
	Model     string
	DataDir   string
}

// Default returns a Config with sensible defaults.
func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/tmp"
	}
	return Config{
		Port:      3000,
		Host:      "127.0.0.1",
		DevUIAddr: "http://localhost:5173",
		Model:     "claude-sonnet-4-6",
		DataDir:   filepath.Join(home, ".soul"),
	}
}

// FromEnv reads configuration from environment variables, falling back to defaults.
func FromEnv() Config {
	cfg := Default()

	if v := os.Getenv("SOUL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			cfg.Port = p
		}
	}

	if v := os.Getenv("SOUL_HOST"); v != "" {
		cfg.Host = v
	}

	if v := os.Getenv("SOUL_DEV"); v != "" {
		cfg.DevMode = v == "1" || v == "true"
	}

	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.APIKey = v
	}

	if v := os.Getenv("SOUL_MODEL"); v != "" {
		cfg.Model = v
	}

	if v := os.Getenv("SOUL_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	return cfg
}
