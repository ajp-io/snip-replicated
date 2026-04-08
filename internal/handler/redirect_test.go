package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/ajp-io/snips-replicated/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestRedirectHandler_CacheHit(t *testing.T) {
	cache := &stubCache{val: "1|https://example.com", hit: true}
	store := &stubStore{}
	recorder := handler.NewClickRecorder(store, 10)
	defer recorder.Shutdown()
	sdk := noopSDKServer(t)

	h := handler.NewRedirectHandler(store, cache, recorder, sdk.URL)
	r := chi.NewRouter()
	r.Get("/{slug}", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Location"))
}

func TestRedirectHandler_CacheMiss_DBHit(t *testing.T) {
	cache := &stubCache{hit: false}
	dest := "https://example.com/from-db"
	store := &stubStore{link: &model.Link{ID: 1, Slug: "xyz", Destination: dest}}
	recorder := handler.NewClickRecorder(store, 10)
	defer recorder.Shutdown()
	sdk := noopSDKServer(t)

	h := handler.NewRedirectHandler(store, cache, recorder, sdk.URL)
	r := chi.NewRouter()
	r.Get("/{slug}", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, dest, w.Header().Get("Location"))
}

func TestRedirectHandler_NotFound(t *testing.T) {
	cache := &stubCache{hit: false}
	store := &stubStore{getSlugErr: db.ErrNotFound}
	recorder := handler.NewClickRecorder(store, 10)
	defer recorder.Shutdown()
	sdk := noopSDKServer(t)

	h := handler.NewRedirectHandler(store, cache, recorder, sdk.URL)
	r := chi.NewRouter()
	r.Get("/{slug}", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRedirectHandler_Expired(t *testing.T) {
	cache := &stubCache{hit: false}
	past := time.Now().Add(-time.Hour)
	store := &stubStore{link: &model.Link{ID: 1, Slug: "exp", Destination: "https://x.com", ExpiresAt: &past}}
	recorder := handler.NewClickRecorder(store, 10)
	defer recorder.Shutdown()
	sdk := noopSDKServer(t)

	h := handler.NewRedirectHandler(store, cache, recorder, sdk.URL)
	r := chi.NewRouter()
	r.Get("/{slug}", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/exp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
}

var _ = errors.New // suppress unused import if needed
