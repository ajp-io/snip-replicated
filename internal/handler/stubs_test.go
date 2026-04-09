package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/model"
)

// stubStore implements db.Store for testing without a real database.
type stubStore struct {
	pingErr    error
	link       *model.Link
	links      []model.LinkWithCount
	daily      []model.DailyClicks
	referrers  []model.ReferrerCount
	createErr  error
	getSlugErr error
	getIDErr   error
	deleteErr  error
	metrics    db.Metrics
	metricsErr error
}

func (s *stubStore) Ping(_ context.Context) error { return s.pingErr }
func (s *stubStore) CreateLink(_ context.Context, slug, dest, label string, exp *time.Time) (*model.Link, error) {
	return s.link, s.createErr
}
func (s *stubStore) GetLinkBySlug(_ context.Context, slug string) (*model.Link, error) {
	return s.link, s.getSlugErr
}
func (s *stubStore) GetLinkByID(_ context.Context, id int64) (*model.Link, error) {
	return s.link, s.getIDErr
}
func (s *stubStore) ListLinksWithClickCounts(_ context.Context) ([]model.LinkWithCount, error) {
	return s.links, nil
}
func (s *stubStore) DeleteLink(_ context.Context, id int64) error { return s.deleteErr }
func (s *stubStore) RecordClick(_ context.Context, linkID int64, referrer string) error {
	return nil
}
func (s *stubStore) GetDailyClicks(_ context.Context, linkID int64) ([]model.DailyClicks, error) {
	return s.daily, nil
}
func (s *stubStore) GetTopReferrers(_ context.Context, linkID int64) ([]model.ReferrerCount, error) {
	return s.referrers, nil
}
func (s *stubStore) GetMetrics(_ context.Context) (db.Metrics, error) {
	return s.metrics, s.metricsErr
}

// stubCache implements cache.Cache for testing without a real Redis.
type stubCache struct {
	pingErr error
	val     string
	hit     bool
}

func (c *stubCache) Ping(_ context.Context) error                                { return c.pingErr }
func (c *stubCache) Get(_ context.Context, key string) (string, bool, error)     { return c.val, c.hit, nil }
func (c *stubCache) Set(_ context.Context, k, v string, ttl time.Duration) error { return nil }
func (c *stubCache) Del(_ context.Context, key string) error                     { return nil }

// noopSDKServer starts an httptest server that accepts any request with 200 OK.
// Pass its URL to handlers so SDK fire-and-forget goroutines complete quickly in tests.
func noopSDKServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}
