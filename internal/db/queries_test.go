//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	m1, _ := os.ReadFile("../assets/migrations/001_create_links.sql")
	m2, _ := os.ReadFile("../assets/migrations/002_create_click_events.sql")
	migrationsFS := fstest.MapFS{
		"001_create_links.sql":        &fstest.MapFile{Data: m1},
		"002_create_click_events.sql": &fstest.MapFile{Data: m2},
	}
	store, err := db.New(context.Background(), dsn, migrationsFS)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCreateAndGetLink(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	link, err := store.CreateLink(ctx, "test01", "https://example.com", "Test", nil)
	require.NoError(t, err)
	assert.Equal(t, "test01", link.Slug)
	assert.Equal(t, "https://example.com", link.Destination)

	got, err := store.GetLinkBySlug(ctx, "test01")
	require.NoError(t, err)
	assert.Equal(t, link.ID, got.ID)

	require.NoError(t, store.DeleteLink(ctx, link.ID))
}

func TestGetLinkBySlug_NotFound(t *testing.T) {
	store := newTestDB(t)
	_, err := store.GetLinkBySlug(context.Background(), "doesnotexist")
	assert.ErrorIs(t, err, db.ErrNotFound)
}

func TestRecordClickAndGetDailyClicks(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	link, err := store.CreateLink(ctx, "clk01", "https://example.com/click", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { store.DeleteLink(ctx, link.ID) })

	require.NoError(t, store.RecordClick(ctx, link.ID, "https://news.ycombinator.com"))
	require.NoError(t, store.RecordClick(ctx, link.ID, "https://news.ycombinator.com"))
	require.NoError(t, store.RecordClick(ctx, link.ID, ""))

	daily, err := store.GetDailyClicks(ctx, link.ID)
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.Equal(t, int64(3), daily[0].Clicks)

	refs, err := store.GetTopReferrers(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://news.ycombinator.com", refs[0].Referrer)
	assert.Equal(t, int64(2), refs[0].Clicks)
	assert.Equal(t, "(direct)", refs[1].Referrer)
}

func TestListLinksWithClickCounts(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	l1, err := store.CreateLink(ctx, "list01", "https://a.com", "", nil)
	require.NoError(t, err)
	l2, err := store.CreateLink(ctx, "list02", "https://b.com", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		store.DeleteLink(ctx, l1.ID)
		store.DeleteLink(ctx, l2.ID)
	})

	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))
	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))

	links, err := store.ListLinksWithClickCounts(ctx)
	require.NoError(t, err)
	var found bool
	for _, lc := range links {
		if lc.Slug == "list01" {
			assert.Equal(t, int64(2), lc.TotalClicks)
			found = true
		}
	}
	assert.True(t, found)
}

func TestLinkExpiry(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	link, err := store.CreateLink(ctx, "exp01", "https://example.com", "", &past)
	require.NoError(t, err)
	t.Cleanup(func() { store.DeleteLink(ctx, link.ID) })

	got, err := store.GetLinkBySlug(ctx, "exp01")
	require.NoError(t, err)
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, got.ExpiresAt.Before(time.Now()))
}

func TestGetMetrics(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Create two links: one active, one expired
	past := time.Now().Add(-time.Hour)
	l1, err := store.CreateLink(ctx, "met01", "https://a.com", "", nil)
	require.NoError(t, err)
	l2, err := store.CreateLink(ctx, "met02", "https://b.com", "", &past)
	require.NoError(t, err)
	t.Cleanup(func() {
		store.DeleteLink(ctx, l1.ID)
		store.DeleteLink(ctx, l2.ID)
	})

	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))
	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))

	m, err := store.GetMetrics(ctx)
	require.NoError(t, err)
	// At minimum our test rows should appear (other tests may have added rows)
	assert.GreaterOrEqual(t, m.TotalLinks, int64(2))
	assert.GreaterOrEqual(t, m.TotalClicks, int64(2))
	// l1 is active (no expiry), l2 is expired
	assert.GreaterOrEqual(t, m.ActiveLinks, int64(1))
	assert.Less(t, m.ActiveLinks, m.TotalLinks+1)
}
