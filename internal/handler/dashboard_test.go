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
			var updates []any
			if updateAvailable {
				updates = []any{map[string]any{"versionLabel": "1.0.1"}}
			}
			json.NewEncoder(w).Encode(updates)
		case "/api/v1/license/info":
			if licenseExpired {
				json.NewEncoder(w).Encode(map[string]any{
					"entitlements": map[string]any{
						"expires_at": map[string]any{"value": time.Now().Add(-time.Hour).Format(time.RFC3339)},
					},
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"entitlements": map[string]any{}})
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
