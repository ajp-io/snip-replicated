package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/ajp-io/snips-replicated/internal/db"
)

const cacheKeyPrefix = "slug:"

// ClickRecorder asynchronously records click events via a buffered channel.
type ClickRecorder struct {
	store  db.Store
	buffer chan clickEvent
	done   chan struct{}
}

type clickEvent struct {
	linkID   int64
	referrer string
}

// NewClickRecorder starts the background goroutine.
func NewClickRecorder(store db.Store, bufferSize int) *ClickRecorder {
	cr := &ClickRecorder{
		store:  store,
		buffer: make(chan clickEvent, bufferSize),
		done:   make(chan struct{}),
	}
	go cr.run()
	return cr
}

// Record enqueues a click event. Drops silently if the buffer is full.
func (cr *ClickRecorder) Record(linkID int64, referrer string) {
	select {
	case cr.buffer <- clickEvent{linkID, referrer}:
	default:
	}
}

// Shutdown drains the buffer and waits for the goroutine to exit.
func (cr *ClickRecorder) Shutdown() {
	close(cr.buffer)
	<-cr.done
}

func (cr *ClickRecorder) run() {
	for e := range cr.buffer {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = cr.store.RecordClick(ctx, e.linkID, e.referrer)
		cancel()
	}
	close(cr.done)
}

// encodeCache encodes link ID + destination for Redis storage.
func encodeCache(id int64, destination string) string {
	return fmt.Sprintf("%d|%s", id, destination)
}

// decodeCache parses the cached value. Returns 0, original value on parse error.
func decodeCache(val string) (int64, string) {
	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return 0, val
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, val
	}
	return id, parts[1]
}

// RedirectHandler serves GET /:slug.
type RedirectHandler struct {
	store    db.Store
	cache    cache.Cache
	recorder *ClickRecorder
}

func NewRedirectHandler(store db.Store, cache cache.Cache, recorder *ClickRecorder) *RedirectHandler {
	return &RedirectHandler{store: store, cache: cache, recorder: recorder}
}

func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	// 1. Try cache first.
	if raw, ok, _ := h.cache.Get(ctx, cacheKeyPrefix+slug); ok {
		id, destination := decodeCache(raw)
		if destination != "" {
			h.recorder.Record(id, r.Referer())
			http.Redirect(w, r, destination, http.StatusFound)
			return
		}
	}

	// 2. Cache miss — query DB.
	link, err := h.store.GetLinkBySlug(ctx, slug)
	if errors.Is(err, db.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 3. Check expiry.
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		http.Error(w, "this link has expired", http.StatusGone)
		return
	}

	// 4. Populate cache with TTL = 1 hour, or time until expiry if shorter.
	ttl := time.Hour
	if link.ExpiresAt != nil {
		if remaining := time.Until(*link.ExpiresAt); remaining < ttl {
			ttl = remaining
		}
	}
	if ttl > 0 {
		_ = h.cache.Set(ctx, cacheKeyPrefix+slug, encodeCache(link.ID, link.Destination), ttl)
	}

	// 5. Record click asynchronously.
	h.recorder.Record(link.ID, r.Referer())

	http.Redirect(w, r, link.Destination, http.StatusFound)
}
