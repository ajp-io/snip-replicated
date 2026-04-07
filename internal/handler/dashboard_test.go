package handler_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/ajp-io/snips-replicated/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestDashboardHandler_RendersList(t *testing.T) {
	store := &stubStore{
		links: []model.LinkWithCount{
			{Link: model.Link{ID: 1, Slug: "abc", Destination: "https://example.com", CreatedAt: time.Now()}, TotalClicks: 42},
		},
	}
	tmpl := template.Must(template.New("home").Parse(`{{range .Links}}{{.Slug}}:{{.TotalClicks}}{{end}}`))
	h := handler.NewDashboardHandler(store, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "abc:42")
}

func TestDashboardHandler_EmptyState(t *testing.T) {
	store := &stubStore{links: nil}
	tmpl := template.Must(template.New("home").Parse(`{{if .Links}}links{{else}}empty{{end}}`))
	h := handler.NewDashboardHandler(store, tmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "empty")
}
