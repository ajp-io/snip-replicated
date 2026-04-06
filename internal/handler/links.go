package handler

import (
	"errors"
	"html/template"
	"net/http"
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
	rowTmpl    *template.Template // rendered on successful create (HTMX swap)
	detailTmpl *template.Template // full link detail page + form partial
	baseURL    string
}

func NewLinksHandler(store db.Store, cache cache.Cache, rowTmpl, detailTmpl *template.Template, baseURL string) *LinksHandler {
	return &LinksHandler{store: store, cache: cache, rowTmpl: rowTmpl, detailTmpl: detailTmpl, baseURL: baseURL}
}

// Form serves GET /links/new — the inline create-link form.
func (h *LinksHandler) Form(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.detailTmpl.ExecuteTemplate(w, "link-form", nil)
}

// Create handles POST /links.
func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
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
		h.renderFormError(w, "Destination URL is required.")
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
		h.renderFormError(w, "Slug must be 3–64 characters: letters, numbers, hyphens, underscores only.")
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
		h.renderFormError(w, "Could not create link. The slug may already be taken.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.rowTmpl.ExecuteTemplate(w, "link-row", model.LinkWithCount{Link: *link})
}

func (h *LinksHandler) renderFormError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if h.detailTmpl != nil {
		h.detailTmpl.ExecuteTemplate(w, "link-form", map[string]string{"Error": msg})
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
	h.detailTmpl.Execute(w, DetailData{
		Link:      link,
		Daily:     daily,
		Referrers: referrers,
		ShortURL:  h.baseURL + "/" + link.Slug,
	})
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

	if err := h.store.DeleteLink(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		http.NotFound(w, r)
		return
	}

	if link != nil {
		_ = h.cache.Del(r.Context(), cacheKeyPrefix+link.Slug)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
