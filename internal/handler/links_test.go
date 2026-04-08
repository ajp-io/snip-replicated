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
