package handler

import (
	"html/template"
	"net/http"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/model"
)

// DashboardData is the template context for the home page.
type DashboardData struct {
	Links []model.LinkWithCount
}

// DashboardHandler serves GET /.
type DashboardHandler struct {
	store db.Store
	tmpl  *template.Template
}

func NewDashboardHandler(store db.Store, tmpl *template.Template) *DashboardHandler {
	return &DashboardHandler{store: store, tmpl: tmpl}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	links, err := h.store.ListLinksWithClickCounts(r.Context())
	if err != nil {
		http.Error(w, "failed to load links", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.Execute(w, DashboardData{Links: links})
}
