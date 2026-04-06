package main

import (
	"context"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ajp-io/snips-replicated/internal/assets"
	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/ajp-io/snips-replicated/internal/config"
	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	// Database — pass a sub-FS rooted at "migrations/"
	migrFS, err := fs.Sub(assets.Migrations, "migrations")
	if err != nil {
		log.Fatalf("migrations fs: %v", err)
	}
	store, err := db.New(ctx, cfg.DatabaseURL, migrFS)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	// Cache
	redisCache, err := cache.New(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}

	// Templates — parse base + page templates + partials
	tmpl, err := template.ParseFS(assets.Templates, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	// Handlers
	recorder := handler.NewClickRecorder(store, 512)
	defer recorder.Shutdown()

	healthH := handler.NewHealthHandler(store, redisCache)
	dashboardH := handler.NewDashboardHandler(store, tmpl)
	linksH := handler.NewLinksHandler(store, redisCache, tmpl, tmpl, cfg.BaseURL)
	redirectH := handler.NewRedirectHandler(store, redisCache, recorder)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static files
	staticSub, err := fs.Sub(assets.Static, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	r.Get("/healthz", healthH.ServeHTTP)
	r.Get("/", dashboardH.ServeHTTP)
	r.Get("/links/new", linksH.Form)
	r.Post("/links", linksH.Create)
	r.Get("/links/{id}", linksH.Detail)
	r.Delete("/links/{id}", linksH.Delete)

	// Slug redirect — registered last so it doesn't shadow other routes
	r.Get("/{slug}", redirectH.ServeHTTP)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		log.Printf("snip listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	<-quit
	log.Println("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
