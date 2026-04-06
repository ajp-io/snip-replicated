//go:build integration

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T) cache.Cache {
	t.Helper()
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	c, err := cache.New(redisURL)
	require.NoError(t, err)
	return c
}

func TestSetAndGet(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "test-key", "test-value", time.Minute)
	require.NoError(t, err)

	val, ok, err := c.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "test-value", val)
}

func TestGet_Miss(t *testing.T) {
	c := newTestCache(t)
	_, ok, err := c.Get(context.Background(), "key-that-does-not-exist-xyz")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDel(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "del-key", "val", time.Minute))
	require.NoError(t, c.Del(ctx, "del-key"))

	_, ok, err := c.Get(ctx, "del-key")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPing(t *testing.T) {
	c := newTestCache(t)
	assert.NoError(t, c.Ping(context.Background()))
}
