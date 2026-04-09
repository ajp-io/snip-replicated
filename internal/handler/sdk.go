package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ajp-io/snips-replicated/internal/db"
)

const defaultSDKEndpoint = "http://snip-sdk:3000"

func resolveEndpoint(endpoint string) string {
	if endpoint == "" {
		return defaultSDKEndpoint
	}
	return endpoint
}

// InstanceState holds SDK-derived state for display in the UI.
type InstanceState struct {
	UpdateAvailable bool
	LicenseInvalid  bool
}

// SendMetrics queries current DB counts and POSTs them to the SDK.
// Errors are logged and never returned — metrics are best-effort.
func SendMetrics(ctx context.Context, store db.Store, endpoint string) {
	endpoint = resolveEndpoint(endpoint)

	m, err := store.GetMetrics(ctx)
	if err != nil {
		log.Printf("sdk: metrics query error: %v", err)
		return
	}

	payload := map[string]any{
		"data": map[string]any{
			"total_links":  m.TotalLinks,
			"total_clicks": m.TotalClicks,
			"active_links": m.ActiveLinks,
		},
	}
	body, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint+"/api/v1/app/custom-metrics", bytes.NewReader(body))
	if err != nil {
		log.Printf("sdk: sendMetrics request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("sdk: sendMetrics error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("sdk: sendMetrics unexpected status: %d", resp.StatusCode)
	}
}

// LicenseEnabled checks whether a boolean license field is enabled.
// Returns false on any error (deny by default).
func LicenseEnabled(ctx context.Context, endpoint, field string) bool {
	endpoint = resolveEndpoint(endpoint)

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"/api/v1/license/fields", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("sdk: licenseEnabled error: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result map[string]struct {
		Value any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	if f, ok := result[field]; ok {
		switch v := f.Value.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(v, "true")
		}
	}
	return false
}

// GetInstanceState fetches update availability and license validity from the SDK.
// Never fails — returns zero-value InstanceState on any error.
func GetInstanceState(ctx context.Context, endpoint string) InstanceState {
	endpoint = resolveEndpoint(endpoint)
	var state InstanceState

	// Check for available updates
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"/api/v1/app/updates", nil)
	resp, err := http.DefaultClient.Do(req)
	cancel()
	if err == nil && resp.StatusCode == http.StatusOK {
		var updates []json.RawMessage
		if json.NewDecoder(resp.Body).Decode(&updates) == nil {
			state.UpdateAvailable = len(updates) > 0
		}
		resp.Body.Close()
	}

	// Check license validity via entitlements.expires_at
	reqCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
	req, _ = http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"/api/v1/license/info", nil)
	resp, err = http.DefaultClient.Do(req)
	cancel()
	if err == nil && resp.StatusCode == http.StatusOK {
		var result struct {
			Entitlements map[string]struct {
				Value any `json:"value"`
			} `json:"entitlements"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			if e, ok := result.Entitlements["expires_at"]; ok {
				if s, ok := e.Value.(string); ok && s != "" {
					t, err := time.Parse(time.RFC3339, s)
					if err == nil && t.Before(time.Now()) {
						state.LicenseInvalid = true
					}
				}
			}
		}
		resp.Body.Close()
	}

	return state
}
