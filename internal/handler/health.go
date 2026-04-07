package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/ajp-io/snips-replicated/internal/db"
)

// HealthHandler serves GET /healthz.
type HealthHandler struct {
	store db.Store
	cache cache.Cache
}

func NewHealthHandler(store db.Store, cache cache.Cache) *HealthHandler {
	return &HealthHandler{store: store, cache: cache}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	if err := h.store.Ping(ctx); err != nil {
		dbStatus = "error"
	}
	cacheStatus := "ok"
	if err := h.cache.Ping(ctx); err != nil {
		cacheStatus = "error"
	}

	overall := "ok"
	statusCode := http.StatusOK
	if dbStatus == "error" || cacheStatus == "error" {
		overall = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": overall,
		"checks": map[string]string{
			"database": dbStatus,
			"cache":    cacheStatus,
		},
	})
}
