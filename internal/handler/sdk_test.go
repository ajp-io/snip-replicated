package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseEnabled_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/license/fields", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []map[string]any{
				{"name": "custom_slugs_enabled", "value": "true"},
			},
		})
	}))
	defer srv.Close()

	assert.True(t, handler.LicenseEnabled(context.Background(), srv.URL, "custom_slugs_enabled"))
}

func TestLicenseEnabled_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []map[string]any{
				{"name": "custom_slugs_enabled", "value": "false"},
			},
		})
	}))
	defer srv.Close()

	assert.False(t, handler.LicenseEnabled(context.Background(), srv.URL, "custom_slugs_enabled"))
}

func TestLicenseEnabled_FieldMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"fields": []any{}})
	}))
	defer srv.Close()

	assert.False(t, handler.LicenseEnabled(context.Background(), srv.URL, "custom_slugs_enabled"))
}

func TestLicenseEnabled_SDKDown(t *testing.T) {
	assert.False(t, handler.LicenseEnabled(context.Background(), "http://127.0.0.1:19999", "custom_slugs_enabled"))
}

func TestGetInstanceState_UpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app/updates":
			json.NewEncoder(w).Encode(map[string]any{
				"updates": []map[string]any{{"versionLabel": "1.0.1"}},
			})
		case "/api/v1/license/info":
			json.NewEncoder(w).Encode(map[string]any{"expirationPolicy": "non-expiring"})
		}
	}))
	defer srv.Close()

	state := handler.GetInstanceState(context.Background(), srv.URL)
	assert.True(t, state.UpdateAvailable)
	assert.False(t, state.LicenseInvalid)
}

func TestGetInstanceState_LicenseExpired(t *testing.T) {
	expired := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app/updates":
			json.NewEncoder(w).Encode(map[string]any{"updates": []any{}})
		case "/api/v1/license/info":
			json.NewEncoder(w).Encode(map[string]any{
				"expirationPolicy": "expire",
				"expiresAt":        expired,
			})
		}
	}))
	defer srv.Close()

	state := handler.GetInstanceState(context.Background(), srv.URL)
	assert.False(t, state.UpdateAvailable)
	assert.True(t, state.LicenseInvalid)
}

func TestGetInstanceState_SDKDown(t *testing.T) {
	state := handler.GetInstanceState(context.Background(), "http://127.0.0.1:19999")
	assert.False(t, state.UpdateAvailable)
	assert.False(t, state.LicenseInvalid)
}

func TestSendMetrics_PostsToSDK(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/app/custom-metrics", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &stubStore{metrics: db.Metrics{TotalLinks: 5, TotalClicks: 20, ActiveLinks: 4}}
	handler.SendMetrics(context.Background(), store, srv.URL)

	require.NotNil(t, received)
	data, ok := received["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 3)
}

func TestSendMetrics_SDKDown(t *testing.T) {
	store := &stubStore{metrics: db.Metrics{TotalLinks: 1}}
	handler.SendMetrics(context.Background(), store, "http://127.0.0.1:19999")
	// should not panic or block
}
