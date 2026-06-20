package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/infrastructure/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("BEACON_DB_DSN", "postgres://localhost/testdb")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 2*time.Second, cfg.ProcessInterval)
	assert.Equal(t, 100, cfg.ProcessBatch)
	assert.Equal(t, time.Hour, cfg.DigestInterval)
	assert.InDelta(t, 5.0, cfg.SpikeFactor, 0.001)
	assert.Equal(t, int64(10), cfg.SpikeMin)
	assert.Equal(t, 15*time.Minute, cfg.AlertCooldown)
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("BEACON_DB_DSN", "postgres://localhost/mydb")
	t.Setenv("BEACON_HTTP_ADDR", ":9090")
	t.Setenv("BEACON_LOG_LEVEL", "debug")
	t.Setenv("BEACON_PROCESS_BATCH", "50")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 50, cfg.ProcessBatch)
}

func TestLoad_MissingRequiredDSN(t *testing.T) {
	// BEACON_DB_DSN is required; empty string should also fail.
	t.Setenv("BEACON_DB_DSN", "")

	_, err := config.Load()
	assert.Error(t, err)
}
