package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_AllOK(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{}, &stubCache{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	checks := body["checks"].(map[string]interface{})
	assert.Equal(t, "ok", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestHealthHandler_DBDown(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{pingErr: errors.New("conn refused")}, &stubCache{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body["status"])
	checks := body["checks"].(map[string]interface{})
	assert.Equal(t, "error", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestHealthHandler_CacheDown(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{}, &stubCache{pingErr: errors.New("conn refused")})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
