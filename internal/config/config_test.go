package config_test

import (
	"os"
	"testing"

	"github.com/ajp-io/snips-replicated/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_AllRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost/snip", cfg.DatabaseURL)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Equal(t, "9090", cfg.Port)
}

func TestLoad_DefaultPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	os.Unsetenv("PORT")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_MissingRedisURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	os.Unsetenv("REDIS_URL")
	t.Setenv("BASE_URL", "http://localhost:8080")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_URL")
}

func TestLoad_MissingBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Unsetenv("BASE_URL")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BASE_URL")
}
