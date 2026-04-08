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
