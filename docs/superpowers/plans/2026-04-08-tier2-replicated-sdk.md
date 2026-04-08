# Tier 2: Replicated SDK Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate the Replicated SDK into the snip Helm chart and Go application to enable license entitlements, custom metrics, update notifications, and license validity enforcement; proxy all images through `proxy.alexparker.info`.

**Architecture:** The Replicated SDK runs as a separate pod (`snip-sdk`) reachable in-cluster at `http://snip-sdk:3000`. Three handler types (redirect, links, dashboard) gain an `sdkEndpoint string` field and call SDK helper functions in `internal/handler/sdk.go`. All SDK calls are best-effort: errors are logged and never surface to the user.

**Tech Stack:** Go 1.25, net/http, html/template, Helm 3, Replicated SDK subchart

---

## File Map

| File | Change |
|------|--------|
| `chart/snip/Chart.yaml` | Add `replicated` subchart dependency |
| `chart/snip/values.yaml` | Add SDK `nameOverride`, proxy image registries for all images |
| `chart/snip/values.schema.json` | Already updated with `global.replicated` and `replicated` entries |
| `chart/snip/templates/deployment.yaml` | Proxy app + busybox images; add `REPLICATED_SDK_ENDPOINT` env var |
| `internal/config/config.go` | Add `SDKEndpoint` field (default `http://snip-sdk:3000`) |
| `internal/db/store.go` | Add `GetMetrics(ctx) (Metrics, error)` to `Store` interface |
| `internal/db/queries.go` | Implement `GetMetrics` |
| `internal/db/queries_test.go` | Integration test for `GetMetrics` |
| `internal/handler/sdk.go` | New: `sendMetrics`, `licenseEnabled`, `getInstanceState` |
| `internal/handler/sdk_test.go` | New: unit tests for all sdk.go functions using httptest servers |
| `internal/handler/stubs_test.go` | Add `GetMetrics` stub + `noopSDKServer` helper |
| `internal/handler/redirect.go` | Add `sdkEndpoint` field; call `go sendMetrics` after click |
| `internal/handler/redirect_test.go` | Update constructor calls; add SDK server to tests |
| `internal/handler/links.go` | Add `sdkEndpoint` + `LinkFormData`; entitlement check in `Form`/`Create`; `sendMetrics` in `Create`/`Delete` |
| `internal/handler/links_test.go` | Update constructor calls; add entitlement tests |
| `internal/handler/dashboard.go` | Add `sdkEndpoint`; update `DashboardData`; call `getInstanceState` |
| `internal/handler/dashboard_test.go` | Update constructor calls; add banner tests |
| `internal/assets/templates/partials/link-form.html` | Conditional custom slug field on `CustomSlugsEnabled` |
| `internal/assets/templates/home.html` | Add update and license expiry banners |
| `cmd/snip/main.go` | Pass `cfg.SDKEndpoint` to all three handlers |

---

## Task 1: Helm chart — SDK subchart + image proxy

**Files:**
- Modify: `chart/snip/Chart.yaml`
- Modify: `chart/snip/values.yaml`
- Modify: `chart/snip/templates/deployment.yaml`

No unit tests for Helm/YAML. Validation is `helm dependency update` + `helm lint`.

- [ ] **Step 1: Find the latest Replicated SDK chart version**

```bash
helm pull oci://registry.replicated.com/library/replicated --untar --untardir /tmp/replicated-chart 2>&1 | head -5
# Or check: https://github.com/replicatedhq/helm-charts/releases
# Note the version string for use in Chart.yaml
```

- [ ] **Step 2: Add replicated subchart to Chart.yaml**

```yaml
# chart/snip/Chart.yaml
apiVersion: v2
name: snip
description: A self-hosted URL shortener with click analytics
type: application
version: 0.1.0
appVersion: "0.1.0"
dependencies:
  - name: postgresql
    version: "16.6.4"
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
  - name: redis
    version: "20.11.3"
    repository: "https://charts.bitnami.com/bitnami"
    condition: redis.enabled
  - name: replicated
    version: "1.5.2"   # replace with version found in step 1
    repository: "oci://registry.replicated.com/library"
```

- [ ] **Step 3: Add SDK nameOverride and image proxy overrides to values.yaml**

Add these sections to the bottom of `chart/snip/values.yaml` (before the final blank line):

```yaml
## Replicated SDK subchart — renamed for branding
replicated:
  nameOverride: snip-sdk
  image:
    registry: proxy.alexparker.info/proxy/snip-enterprise/registry.replicated.com

## Image proxy — all images routed through custom proxy domain
## Overrides bitnami subchart image registries
postgresql:
  enabled: true
  image:
    registry: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io
    tag: latest
  auth:
    username: snip
    password: snip
    database: snip
  primary:
    persistence:
      enabled: true
      size: 1Gi
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        cpu: 500m
        memory: 512Mi

redis:
  enabled: true
  image:
    registry: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io
    tag: latest
  architecture: standalone
  auth:
    enabled: false
  master:
    persistence:
      enabled: false
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 200m
        memory: 128Mi
```

> Note: The existing `postgresql` and `redis` sections in values.yaml must be replaced (not duplicated) with the above. The `image.registry` key is what adds the proxy prefix; the chart appends `bitnami/postgresql` or `bitnami/redis` as the repository.

- [ ] **Step 4: Update app image and busybox init container in deployment.yaml**

Replace the `image:` line in the app container and the init container image:

```yaml
# In the initContainers section, replace:
#   image: busybox:1.36
# With:
          image: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io/library/busybox:1.36

# In the containers section, replace:
#   image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
# With (no change to template, just values.yaml):
```

Update `values.yaml` app image repository:
```yaml
image:
  repository: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io/ajpio/snip
  tag: "0.1.0"
  pullPolicy: IfNotPresent
```

- [ ] **Step 5: Run helm dependency update and lint**

```bash
helm dependency update chart/snip
helm lint chart/snip
```

Expected: `==> Linting chart/snip` with no errors. Warnings about missing values are OK.

- [ ] **Step 6: Commit**

```bash
git add chart/snip/Chart.yaml chart/snip/values.yaml chart/snip/templates/deployment.yaml chart/snip/charts/
git commit -m "$(cat <<'EOF'
feat(chart): add Replicated SDK subchart and proxy all images through custom domain

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: SDKEndpoint in config + deployment env var

**Files:**
- Modify: `internal/config/config.go`
- Modify: `chart/snip/templates/deployment.yaml`

- [ ] **Step 1: Add SDKEndpoint to Config**

```go
// internal/config/config.go
package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
	BaseURL     string
	SDKEndpoint string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Port:        os.Getenv("PORT"),
		BaseURL:     os.Getenv("BASE_URL"),
		SDKEndpoint: os.Getenv("REPLICATED_SDK_ENDPOINT"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.SDKEndpoint == "" {
		c.SDKEndpoint = "http://snip-sdk:3000"
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}
	if c.BaseURL == "" {
		return Config{}, errors.New("BASE_URL is required")
	}
	return c, nil
}
```

- [ ] **Step 2: Add REPLICATED_SDK_ENDPOINT env var to deployment.yaml**

In `chart/snip/templates/deployment.yaml`, add to the `env:` section of the app container after `REDIS_URL`:

```yaml
            - name: REPLICATED_SDK_ENDPOINT
              value: "http://snip-sdk:3000"
```

The service name is `snip-sdk` because `nameOverride: snip-sdk` is set in values.yaml for the replicated subchart.

- [ ] **Step 3: Verify the app builds**

```bash
go build ./...
```

Expected: no output (build succeeds).

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go chart/snip/templates/deployment.yaml
git commit -m "$(cat <<'EOF'
feat(config): add SDKEndpoint config field and deployment env var

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add GetMetrics to db.Store

**Files:**
- Modify: `internal/db/store.go`
- Modify: `internal/db/queries.go`
- Modify: `internal/db/queries_test.go`

- [ ] **Step 1: Add Metrics type and GetMetrics to Store interface**

```go
// internal/db/store.go
package db

import (
	"context"
	"time"

	"github.com/ajp-io/snips-replicated/internal/model"
)

// Metrics holds aggregated counts for SDK reporting.
type Metrics struct {
	TotalLinks  int64
	TotalClicks int64
	ActiveLinks int64
}

// Store is the interface handlers use for all database operations.
type Store interface {
	CreateLink(ctx context.Context, slug, destination, label string, expiresAt *time.Time) (*model.Link, error)
	GetLinkBySlug(ctx context.Context, slug string) (*model.Link, error)
	GetLinkByID(ctx context.Context, id int64) (*model.Link, error)
	ListLinksWithClickCounts(ctx context.Context) ([]model.LinkWithCount, error)
	DeleteLink(ctx context.Context, id int64) error
	RecordClick(ctx context.Context, linkID int64, referrer string) error
	GetDailyClicks(ctx context.Context, linkID int64) ([]model.DailyClicks, error)
	GetTopReferrers(ctx context.Context, linkID int64) ([]model.ReferrerCount, error)
	GetMetrics(ctx context.Context) (Metrics, error)
	Ping(ctx context.Context) error
}
```

- [ ] **Step 2: Write the failing integration test**

Add to `internal/db/queries_test.go` (inside the existing file, after the last test):

```go
func TestGetMetrics(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Create two links, one expired
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
	// Only l1 is active (l2 is expired)
	assert.GreaterOrEqual(t, m.ActiveLinks, int64(1))
	assert.Less(t, m.ActiveLinks, m.TotalLinks+1)
}
```

- [ ] **Step 3: Run the test to confirm it fails (build error is expected)**

```bash
go test -tags integration -run TestGetMetrics ./internal/db/ -v 2>&1 | head -20
```

Expected: compile error — `store.GetMetrics undefined`.

- [ ] **Step 4: Implement GetMetrics**

Add to `internal/db/queries.go`:

```go
func (d *DB) GetMetrics(ctx context.Context) (Metrics, error) {
	var m Metrics
	err := d.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM links)                                                    AS total_links,
			(SELECT COUNT(*) FROM click_events)                                            AS total_clicks,
			(SELECT COUNT(*) FROM links WHERE expires_at IS NULL OR expires_at > NOW())    AS active_links
	`).Scan(&m.TotalLinks, &m.TotalClicks, &m.ActiveLinks)
	return m, err
}
```

- [ ] **Step 5: Run the integration test**

```bash
go test -tags integration -run TestGetMetrics ./internal/db/ -v
```

Expected: `PASS`. (Requires `TEST_DATABASE_URL` to be set; skip if not available locally.)

- [ ] **Step 6: Commit**

```bash
git add internal/db/store.go internal/db/queries.go internal/db/queries_test.go
git commit -m "$(cat <<'EOF'
feat(db): add GetMetrics to Store interface and implement

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Create internal/handler/sdk.go with helpers

**Files:**
- Create: `internal/handler/sdk.go`
- Create: `internal/handler/sdk_test.go`

- [ ] **Step 1: Write the failing tests in sdk_test.go**

```go
// internal/handler/sdk_test.go
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
	// Port 0 is never bound — connection refused immediately
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
	// Should return zero-value state, never panic or block
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
	// Should not panic
	store := &stubStore{metrics: db.Metrics{TotalLinks: 1}}
	handler.SendMetrics(context.Background(), store, "http://127.0.0.1:19999")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test -run 'TestLicenseEnabled|TestGetInstanceState|TestSendMetrics' ./internal/handler/ -v 2>&1 | head -20
```

Expected: compile error — `handler.LicenseEnabled`, `handler.GetInstanceState`, `handler.SendMetrics` undefined.

- [ ] **Step 3: Implement sdk.go**

```go
// internal/handler/sdk.go
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
		"data": []map[string]any{
			{"name": "total_links", "value": m.TotalLinks},
			{"name": "total_clicks", "value": m.TotalClicks},
			{"name": "active_links", "value": m.ActiveLinks},
		},
	}
	body, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint+"/api/v1/app/custom-metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("sdk: sendMetrics error: %v", err)
		return
	}
	resp.Body.Close()
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

	var result struct {
		Fields []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	for _, f := range result.Fields {
		if f.Name == field {
			return strings.EqualFold(f.Value, "true")
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
		var result struct {
			Updates []json.RawMessage `json:"updates"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			state.UpdateAvailable = len(result.Updates) > 0
		}
		resp.Body.Close()
	}

	// Check license validity
	reqCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
	req, _ = http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint+"/api/v1/license/info", nil)
	resp, err = http.DefaultClient.Do(req)
	cancel()
	if err == nil && resp.StatusCode == http.StatusOK {
		var result struct {
			ExpirationPolicy string `json:"expirationPolicy"`
			ExpiresAt        string `json:"expiresAt"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			if result.ExpirationPolicy == "expire" && result.ExpiresAt != "" {
				t, err := time.Parse(time.RFC3339, result.ExpiresAt)
				if err == nil && t.Before(time.Now()) {
					state.LicenseInvalid = true
				}
			}
		}
		resp.Body.Close()
	}

	return state
}
```

- [ ] **Step 4: Run the sdk tests**

```bash
go test -run 'TestLicenseEnabled|TestGetInstanceState|TestSendMetrics' ./internal/handler/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/sdk.go internal/handler/sdk_test.go
git commit -m "$(cat <<'EOF'
feat(handler): add SDK helper functions (metrics, license, instance state)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Update stubs_test.go — add GetMetrics stub and noopSDKServer helper

**Files:**
- Modify: `internal/handler/stubs_test.go`

- [ ] **Step 1: Add GetMetrics to stubStore and noopSDKServer helper**

```go
// internal/handler/stubs_test.go
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
```

- [ ] **Step 2: Verify the package still compiles**

```bash
go build ./internal/handler/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/handler/stubs_test.go
git commit -m "$(cat <<'EOF'
test(handler): add GetMetrics stub and noopSDKServer test helper

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Update redirect handler — add sdkEndpoint + sendMetrics

**Files:**
- Modify: `internal/handler/redirect.go`
- Modify: `internal/handler/redirect_test.go`
- Modify: `cmd/snip/main.go` (partial — just redirect handler wiring)

- [ ] **Step 1: Update redirect_test.go — fix constructor calls**

Replace all `handler.NewRedirectHandler(store, cache, recorder)` calls (4 occurrences) to pass the noop SDK server URL:

```go
// internal/handler/redirect_test.go
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
```

- [ ] **Step 2: Run tests to confirm compile error**

```bash
go test ./internal/handler/ -run TestRedirect -v 2>&1 | head -10
```

Expected: compile error — `NewRedirectHandler` called with 4 args, wants 3.

- [ ] **Step 3: Update redirect.go**

```go
// internal/handler/redirect.go
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
	store       db.Store
	cache       cache.Cache
	recorder    *ClickRecorder
	sdkEndpoint string
}

func NewRedirectHandler(store db.Store, cache cache.Cache, recorder *ClickRecorder, sdkEndpoint string) *RedirectHandler {
	return &RedirectHandler{store: store, cache: cache, recorder: recorder, sdkEndpoint: sdkEndpoint}
}

func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	// 1. Try cache first.
	if raw, ok, _ := h.cache.Get(ctx, cacheKeyPrefix+slug); ok {
		id, destination := decodeCache(raw)
		if destination != "" {
			h.recorder.Record(id, r.Referer())
			go SendMetrics(context.Background(), h.store, h.sdkEndpoint)
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

	// 5. Record click asynchronously and update metrics.
	h.recorder.Record(link.ID, r.Referer())
	go SendMetrics(context.Background(), h.store, h.sdkEndpoint)

	http.Redirect(w, r, link.Destination, http.StatusFound)
}
```

- [ ] **Step 4: Update main.go redirect handler wiring**

In `cmd/snip/main.go`, find the line:
```go
redirectH := handler.NewRedirectHandler(store, redisCache, recorder)
```
Replace with:
```go
redirectH := handler.NewRedirectHandler(store, redisCache, recorder, cfg.SDKEndpoint)
```

- [ ] **Step 5: Run redirect tests**

```bash
go test ./internal/handler/ -run TestRedirect -v
```

Expected: all 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/redirect.go internal/handler/redirect_test.go cmd/snip/main.go
git commit -m "$(cat <<'EOF'
feat(handler): send custom metrics after each redirect

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Update links handler — custom slug entitlement + metrics

**Files:**
- Modify: `internal/handler/links.go`
- Modify: `internal/handler/links_test.go`
- Modify: `internal/assets/templates/partials/link-form.html`
- Modify: `cmd/snip/main.go` (links handler wiring)

- [ ] **Step 1: Write failing tests for entitlement in links_test.go**

Replace the full `internal/handler/links_test.go` with:

```go
package handler_test

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/ajp-io/snips-replicated/internal/model"
	"github.com/stretchr/testify/assert"
)

// sdkWithLicenseField starts a test SDK server that returns the given boolean value
// for "custom_slugs_enabled".
func sdkWithLicenseField(t *testing.T, enabled bool) *httptest.Server {
	t.Helper()
	val := "false"
	if enabled {
		val = "true"
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"fields": []map[string]any{
				{"name": "custom_slugs_enabled", "value": val},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCreateLink_AutoSlug(t *testing.T) {
	store := &stubStore{link: &model.Link{ID: 1, Slug: "gen01", Destination: "https://example.com", CreatedAt: time.Now()}}
	rowTmpl := template.Must(template.New("link-row").Parse(`{{define "link-row"}}{{.Slug}}{{end}}`))
	sdk := noopSDKServer(t)
	h := handler.NewLinksHandler(store, &stubCache{}, rowTmpl, rowTmpl, "http://localhost", sdk.URL)

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=&label=")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "gen01")
}

func TestCreateLink_CustomSlug_Enabled(t *testing.T) {
	store := &stubStore{link: &model.Link{ID: 2, Slug: "my-link", Destination: "https://example.com", CreatedAt: time.Now()}}
	rowTmpl := template.Must(template.New("link-row").Parse(`{{define "link-row"}}{{.Slug}}{{end}}`))
	sdk := sdkWithLicenseField(t, true)
	h := handler.NewLinksHandler(store, &stubCache{}, rowTmpl, rowTmpl, "http://localhost", sdk.URL)

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=my-link&label=")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateLink_CustomSlug_Disabled(t *testing.T) {
	store := &stubStore{}
	formTmpl := template.Must(template.New("link-form").Parse(`{{define "link-form"}}error:{{.Error}}{{end}}`))
	sdk := sdkWithLicenseField(t, false)
	h := handler.NewLinksHandler(store, &stubCache{}, formTmpl, formTmpl, "http://localhost", sdk.URL)

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=my-link&label=")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "error:")
	assert.Contains(t, w.Body.String(), "Custom slugs")
}

func TestCreateLink_InvalidSlug(t *testing.T) {
	store := &stubStore{}
	formTmpl := template.Must(template.New("link-form").Parse(`{{define "link-form"}}error:{{.Error}}{{end}}`))
	sdk := noopSDKServer(t)
	h := handler.NewLinksHandler(store, &stubCache{}, formTmpl, formTmpl, "http://localhost", sdk.URL)

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=a!")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "error:")
}

func TestDetailLink(t *testing.T) {
	link := &model.Link{ID: 1, Slug: "abc", Destination: "https://example.com", CreatedAt: time.Now()}
	store := &stubStore{link: link, daily: []model.DailyClicks{}, referrers: []model.ReferrerCount{}}
	detailTmpl := template.Must(template.New("link-detail").Parse(`{{.Link.Slug}}`))
	sdk := noopSDKServer(t)
	h := handler.NewLinksHandler(store, &stubCache{}, detailTmpl, detailTmpl, "http://localhost", sdk.URL)

	r := chi.NewRouter()
	r.Get("/links/{id}", h.Detail)

	req := httptest.NewRequest(http.MethodGet, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "abc")
}

func TestDeleteLink(t *testing.T) {
	store := &stubStore{link: &model.Link{ID: 1, Slug: "abc"}}
	sdk := noopSDKServer(t)
	h := handler.NewLinksHandler(store, &stubCache{}, nil, nil, "http://localhost", sdk.URL)

	r := chi.NewRouter()
	r.Delete("/links/{id}", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestDeleteLink_NotFound(t *testing.T) {
	store := &stubStore{deleteErr: db.ErrNotFound}
	sdk := noopSDKServer(t)
	h := handler.NewLinksHandler(store, &stubCache{}, nil, nil, "http://localhost", sdk.URL)

	r := chi.NewRouter()
	r.Delete("/links/{id}", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 2: Run tests to confirm compile error**

```bash
go test ./internal/handler/ -run TestCreateLink -v 2>&1 | head -10
```

Expected: compile error — `NewLinksHandler` called with 6 args, wants 5.

- [ ] **Step 3: Update links.go**

```go
// internal/handler/links.go
package handler

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/model"
	"github.com/ajp-io/snips-replicated/internal/slug"
)

// LinksHandler handles link CRUD and the link-form partial.
type LinksHandler struct {
	store      db.Store
	cache      cache.Cache
	rowTmpl    *template.Template
	detailTmpl *template.Template
	baseURL    string
	sdkEndpoint string
}

// LinkFormData is the template context for the link creation form.
type LinkFormData struct {
	Error              string
	CustomSlugsEnabled bool
}

func NewLinksHandler(store db.Store, cache cache.Cache, rowTmpl, detailTmpl *template.Template, baseURL, sdkEndpoint string) *LinksHandler {
	return &LinksHandler{
		store:       store,
		cache:       cache,
		rowTmpl:     rowTmpl,
		detailTmpl:  detailTmpl,
		baseURL:     baseURL,
		sdkEndpoint: sdkEndpoint,
	}
}

// Form serves GET /links/new — the inline create-link form.
func (h *LinksHandler) Form(w http.ResponseWriter, r *http.Request) {
	data := LinkFormData{
		CustomSlugsEnabled: LicenseEnabled(r.Context(), h.sdkEndpoint, "custom_slugs_enabled"),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.detailTmpl.ExecuteTemplate(w, "link-form", data); err != nil {
		log.Printf("link-form template error: %v", err)
	}
}

// Create handles POST /links.
func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
	customSlugsEnabled := LicenseEnabled(r.Context(), h.sdkEndpoint, "custom_slugs_enabled")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	destination := r.FormValue("destination")
	customSlug := r.FormValue("slug")
	label := r.FormValue("label")
	expiresStr := r.FormValue("expires_at")

	if destination == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderFormError(w, "Destination URL is required.", customSlugsEnabled)
		return
	}

	u, err := url.Parse(destination)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderFormError(w, "Destination must be a valid http:// or https:// URL.", customSlugsEnabled)
		return
	}

	// Reject custom slugs when entitlement is disabled.
	if customSlug != "" && !customSlugsEnabled {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderFormError(w, "Custom slugs require a higher license tier.", customSlugsEnabled)
		return
	}

	finalSlug := customSlug
	if finalSlug == "" {
		var err error
		for i := 0; i < 5; i++ {
			finalSlug, err = slug.Generate()
			if err == nil {
				break
			}
		}
		if err != nil {
			http.Error(w, "failed to generate slug", http.StatusInternalServerError)
			return
		}
	} else if !slug.Validate(finalSlug) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderFormError(w, "Slug must be 3–64 characters: letters, numbers, hyphens, underscores only.", customSlugsEnabled)
		return
	}

	var expiresAt *time.Time
	if expiresStr != "" {
		t, err := time.Parse("2006-01-02", expiresStr)
		if err == nil {
			expiresAt = &t
		}
	}

	link, err := h.store.CreateLink(r.Context(), finalSlug, destination, label, expiresAt)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.renderFormError(w, "Could not create link. The slug may already be taken.", customSlugsEnabled)
		return
	}

	go SendMetrics(context.Background(), h.store, h.sdkEndpoint)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.rowTmpl.ExecuteTemplate(w, "link-row", model.LinkWithCount{Link: *link}); err != nil {
		log.Printf("link-row template error: %v", err)
	}
}

func (h *LinksHandler) renderFormError(w http.ResponseWriter, msg string, customSlugsEnabled bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if h.detailTmpl != nil {
		data := LinkFormData{Error: msg, CustomSlugsEnabled: customSlugsEnabled}
		if err := h.detailTmpl.ExecuteTemplate(w, "link-form", data); err != nil {
			log.Printf("link-form error template error: %v", err)
		}
	}
}

// DetailData is the template context for the link detail page.
type DetailData struct {
	Link      *model.Link
	Daily     []model.DailyClicks
	Referrers []model.ReferrerCount
	ShortURL  string
}

// Detail serves GET /links/:id.
func (h *LinksHandler) Detail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	link, err := h.store.GetLinkByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	daily, _ := h.store.GetDailyClicks(r.Context(), id)
	referrers, _ := h.store.GetTopReferrers(r.Context(), id)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.detailTmpl.Execute(w, DetailData{
		Link:      link,
		Daily:     daily,
		Referrers: referrers,
		ShortURL:  h.baseURL + "/" + link.Slug,
	}); err != nil {
		log.Printf("detail template error: %v", err)
	}
}

// Delete handles DELETE /links/:id.
func (h *LinksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	link, _ := h.store.GetLinkByID(r.Context(), id)

	err = h.store.DeleteLink(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if link != nil {
		_ = h.cache.Del(r.Context(), cacheKeyPrefix+link.Slug)
	}

	go SendMetrics(context.Background(), h.store, h.sdkEndpoint)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

- [ ] **Step 4: Update link-form.html template**

```html
{{define "link-form"}}
<form
  hx-post="/links"
  hx-target="#links-table-body"
  hx-swap="afterbegin"
class="bg-gray-900 border border-gray-700 rounded-lg p-6 mb-8">
  <h2 class="text-lg font-semibold mb-4">New Link</h2>
  <div class="grid grid-cols-2 gap-4">
    <div class="col-span-2">
      <label class="block text-xs text-gray-400 mb-1">Destination URL <span class="text-red-400">*</span></label>
      <input type="url" name="destination" required placeholder="https://example.com/long-path"
        class="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm focus:outline-none focus:border-indigo-500">
    </div>
    {{if .CustomSlugsEnabled}}
    <div>
      <label class="block text-xs text-gray-400 mb-1">Custom slug (optional)</label>
      <input type="text" name="slug" placeholder="my-link"
        class="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm focus:outline-none focus:border-indigo-500">
    </div>
    {{end}}
    <div>
      <label class="block text-xs text-gray-400 mb-1">Label (optional)</label>
      <input type="text" name="label" placeholder="Product Hunt"
        class="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm focus:outline-none focus:border-indigo-500">
    </div>
    <div>
      <label class="block text-xs text-gray-400 mb-1">Expires (optional)</label>
      <input type="date" name="expires_at"
        class="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm focus:outline-none focus:border-indigo-500">
    </div>
    <div class="flex items-end">
      <button type="submit"
        class="bg-indigo-600 hover:bg-indigo-700 text-white text-sm px-6 py-2 rounded-md w-full">
        Create Link
      </button>
    </div>
  </div>
  {{if .Error}}<p class="text-red-400 text-sm mt-3">{{.Error}}</p>{{end}}
</form>
{{end}}
```

- [ ] **Step 5: Update main.go links handler wiring**

Find:
```go
linksH := handler.NewLinksHandler(store, redisCache, rowTmpl, detailTmpl, cfg.BaseURL)
```
Replace with:
```go
linksH := handler.NewLinksHandler(store, redisCache, rowTmpl, detailTmpl, cfg.BaseURL, cfg.SDKEndpoint)
```

- [ ] **Step 6: Run all handler tests**

```bash
go test ./internal/handler/ -v
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/handler/links.go internal/handler/links_test.go \
        internal/assets/templates/partials/link-form.html cmd/snip/main.go
git commit -m "$(cat <<'EOF'
feat(handler): gate custom slugs behind license entitlement; send metrics on create/delete

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Update dashboard handler — update + license banners

**Files:**
- Modify: `internal/handler/dashboard.go`
- Modify: `internal/handler/dashboard_test.go`
- Modify: `internal/assets/templates/home.html`
- Modify: `cmd/snip/main.go` (dashboard handler wiring)

- [ ] **Step 1: Write failing tests in dashboard_test.go**

```go
// internal/handler/dashboard_test.go
package handler_test

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/ajp-io/snips-replicated/internal/model"
	"github.com/stretchr/testify/assert"
)

// sdkWithInstanceState starts a test SDK server returning the given update/license state.
func sdkWithInstanceState(t *testing.T, updateAvailable bool, licenseExpired bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/app/updates":
			updates := []any{}
			if updateAvailable {
				updates = []map[string]any{{"versionLabel": "1.0.1"}}
			}
			json.NewEncoder(w).Encode(map[string]any{"updates": updates})
		case "/api/v1/license/info":
			if licenseExpired {
				json.NewEncoder(w).Encode(map[string]any{
					"expirationPolicy": "expire",
					"expiresAt":        time.Now().Add(-time.Hour).Format(time.RFC3339),
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"expirationPolicy": "non-expiring"})
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDashboardHandler_RendersList(t *testing.T) {
	store := &stubStore{
		links: []model.LinkWithCount{
			{Link: model.Link{ID: 1, Slug: "abc", Destination: "https://example.com", CreatedAt: time.Now()}, TotalClicks: 42},
		},
	}
	tmpl := template.Must(template.New("home").Parse(`{{range .Links}}{{.Slug}}:{{.TotalClicks}}{{end}}`))
	sdk := noopSDKServer(t)
	h := handler.NewDashboardHandler(store, tmpl, sdk.URL)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "abc:42")
}

func TestDashboardHandler_EmptyState(t *testing.T) {
	store := &stubStore{links: nil}
	tmpl := template.Must(template.New("home").Parse(`{{if .Links}}links{{else}}empty{{end}}`))
	sdk := noopSDKServer(t)
	h := handler.NewDashboardHandler(store, tmpl, sdk.URL)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "empty")
}

func TestDashboardHandler_UpdateBanner(t *testing.T) {
	store := &stubStore{}
	tmpl := template.Must(template.New("home").Parse(
		`{{if .UpdateAvailable}}update-available{{end}}{{if .LicenseInvalid}}license-invalid{{end}}`,
	))
	sdk := sdkWithInstanceState(t, true, false)
	h := handler.NewDashboardHandler(store, tmpl, sdk.URL)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "update-available")
	assert.NotContains(t, w.Body.String(), "license-invalid")
}

func TestDashboardHandler_LicenseBanner(t *testing.T) {
	store := &stubStore{}
	tmpl := template.Must(template.New("home").Parse(
		`{{if .UpdateAvailable}}update-available{{end}}{{if .LicenseInvalid}}license-invalid{{end}}`,
	))
	sdk := sdkWithInstanceState(t, false, true)
	h := handler.NewDashboardHandler(store, tmpl, sdk.URL)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "update-available")
	assert.Contains(t, w.Body.String(), "license-invalid")
}
```

- [ ] **Step 2: Run tests to confirm compile error**

```bash
go test ./internal/handler/ -run TestDashboard -v 2>&1 | head -10
```

Expected: compile error — `NewDashboardHandler` called with 3 args, wants 2.

- [ ] **Step 3: Update dashboard.go**

```go
// internal/handler/dashboard.go
package handler

import (
	"html/template"
	"log"
	"net/http"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/model"
)

// DashboardData is the template context for the home page.
type DashboardData struct {
	Links           []model.LinkWithCount
	UpdateAvailable bool
	LicenseInvalid  bool
}

// DashboardHandler serves GET /.
type DashboardHandler struct {
	store       db.Store
	tmpl        *template.Template
	sdkEndpoint string
}

func NewDashboardHandler(store db.Store, tmpl *template.Template, sdkEndpoint string) *DashboardHandler {
	return &DashboardHandler{store: store, tmpl: tmpl, sdkEndpoint: sdkEndpoint}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	links, err := h.store.ListLinksWithClickCounts(r.Context())
	if err != nil {
		http.Error(w, "failed to load links", http.StatusInternalServerError)
		return
	}

	state := GetInstanceState(r.Context(), h.sdkEndpoint)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, DashboardData{
		Links:           links,
		UpdateAvailable: state.UpdateAvailable,
		LicenseInvalid:  state.LicenseInvalid,
	}); err != nil {
		log.Printf("dashboard template error: %v", err)
	}
}
```

- [ ] **Step 4: Update home.html with banners**

```html
{{template "base" .}}
{{define "content"}}
{{if .LicenseInvalid}}
<div class="mb-4 bg-red-950 border border-red-700 text-red-300 rounded-lg px-4 py-3 text-sm">
  Your license has expired. Please contact support.
</div>
{{end}}
{{if .UpdateAvailable}}
<div class="mb-4 bg-yellow-950 border border-yellow-700 text-yellow-300 rounded-lg px-4 py-3 text-sm">
  A new version of Snip is available.
</div>
{{end}}
<div class="mt-6">
  {{if not .Links}}
  <p id="empty-state" class="text-gray-500 text-center py-16">No links yet. Create one above.</p>
  {{end}}
  <table id="links-table" class="w-full text-sm{{if not .Links}} hidden{{end}}">
    <thead>
      <tr class="text-left text-gray-500 text-xs uppercase border-b border-gray-800">
        <th class="pb-3 pr-4">Slug</th>
        <th class="pb-3 pr-4">Destination</th>
        <th class="pb-3 pr-4">Label</th>
        <th class="pb-3 pr-4">Clicks</th>
        <th class="pb-3 pr-4">Expires</th>
        <th class="pb-3"></th>
      </tr>
    </thead>
    <tbody id="links-table-body">
      {{range .Links}}
      {{template "link-row" .}}
      {{end}}
    </tbody>
  </table>
</div>
<script>
document.body.addEventListener('htmx:afterSwap', function(e) {
  if (e.detail.target.id === 'links-table-body') {
    document.getElementById('empty-state')?.remove();
    document.getElementById('links-table')?.classList.remove('hidden');
    document.getElementById('link-form-container').innerHTML = '';
  }
});
</script>
{{end}}
```

- [ ] **Step 5: Update main.go dashboard handler wiring**

Find:
```go
dashboardH := handler.NewDashboardHandler(store, homeTmpl)
```
Replace with:
```go
dashboardH := handler.NewDashboardHandler(store, homeTmpl, cfg.SDKEndpoint)
```

- [ ] **Step 6: Run all tests**

```bash
go test ./internal/handler/ -v
go build ./...
```

Expected: all tests PASS, build succeeds.

- [ ] **Step 7: Commit**

```bash
git add internal/handler/dashboard.go internal/handler/dashboard_test.go \
        internal/assets/templates/home.html cmd/snip/main.go
git commit -m "$(cat <<'EOF'
feat(handler): show update available and license expiry banners on dashboard

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Final verification

- [ ] **Run all non-integration tests**

```bash
go test ./...
```

Expected: all PASS (integration tests skipped without `TEST_DATABASE_URL`).

- [ ] **Helm lint**

```bash
helm lint chart/snip
```

Expected: no errors.

- [ ] **Verify rubric items met**

| Item | How to verify |
|------|--------------|
| 2.1 | `kubectl get deployment snip-sdk -n <ns>` after install |
| 2.2 | `kubectl get pods -A -o custom-columns='STATUS:.status.phase,IMAGE:.spec.containers[*].image'` — all images start with `proxy.alexparker.info` |
| 2.4 | Visit Instance Details in Vendor Portal — see `total_links`, `total_clicks`, `active_links` metrics |
| 2.5 | Disable `custom_slugs_enabled` — custom slug field hidden. Enable — field appears. No redeploy needed. |
| 2.6 | Expire license in Vendor Portal — red banner appears on dashboard. Promote new release — yellow banner appears. |
