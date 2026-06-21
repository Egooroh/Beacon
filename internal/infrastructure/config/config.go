package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration sourced from environment variables.
// Missing required fields cause Load to return an error (fail-fast on startup).
type Config struct {
	HTTPAddr        string        `env:"BEACON_HTTP_ADDR"        envDefault:":8080"`
	DBDSN           string        `env:"BEACON_DB_DSN,required"`
	TelegramToken   string        `env:"BEACON_TELEGRAM_TOKEN"`
	LogLevel        string        `env:"BEACON_LOG_LEVEL"        envDefault:"info"`
	ProcessInterval time.Duration `env:"BEACON_PROCESS_INTERVAL" envDefault:"2s"`
	ProcessBatch    int           `env:"BEACON_PROCESS_BATCH"    envDefault:"100"`
	DigestInterval  time.Duration `env:"BEACON_DIGEST_INTERVAL"  envDefault:"1h"`
	SpikeFactor     float64       `env:"BEACON_SPIKE_FACTOR"     envDefault:"5"`
	SpikeMin        int64         `env:"BEACON_SPIKE_MIN"        envDefault:"10"`
	AlertCooldown   time.Duration `env:"BEACON_ALERT_COOLDOWN"   envDefault:"15m"`
	SentrySecret    string        `env:"BEACON_SENTRY_SECRET"`
	SlackToken      string        `env:"BEACON_SLACK_TOKEN"`
	IngestRateLimit int           `env:"BEACON_INGEST_RATE_LIMIT" envDefault:"60"`
	PublicURL       string        `env:"BEACON_PUBLIC_URL"`
}

// Load parses and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DBDSN == "" {
		return fmt.Errorf("BEACON_DB_DSN is required and must not be empty")
	}
	return nil
}
