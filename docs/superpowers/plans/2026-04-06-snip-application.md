# Snip Application Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Snip URL shortener application — Go backend with HTMX dashboard, PostgreSQL storage, and Redis cache.

**Architecture:** Single Go binary (Chi router) serving redirects, a server-rendered HTMX dashboard, and a `/healthz` endpoint. Redirects are cache-first (Redis → PostgreSQL fallback). Click events are recorded asynchronously via a buffered channel to avoid adding latency to the redirect path.

**Tech Stack:** Go 1.23, `go-chi/chi/v5`, `jackc/pgx/v5`, `redis/go-redis/v9`, `matoous/go-nanoid/v2`, `stretchr/testify`, html/template + HTMX (CDN), Chart.js (CDN), Tailwind CSS (CDN).

---

## File Map

```
snip/
├── cmd/snip/main.go                     # wiring: config → db → cache → handlers → server
├── internal/
│   ├── config/
│   │   ├── config.go                    # Load() reads env vars, validates required fields
│   │   └── config_test.go
│   ├── model/
│   │   └── model.go                     # Link, LinkWithCount, DailyClicks, ReferrerCount types
│   ├── slug/
│   │   ├── slug.go                      # Generate() nanoid, Validate() regex
│   │   └── slug_test.go
│   ├── db/
│   │   ├── store.go                     # Store interface
│   │   ├── db.go                        # pgx pool setup + RunMigrations()
│   │   ├── queries.go                   # Store implementation on *DB
│   │   └── queries_test.go              # integration tests (build tag: integration)
│   ├── cache/
│   │   ├── cache.go                     # Cache interface + Redis implementation
│   │   └── cache_test.go                # integration tests (build tag: integration)
│   └── handler/
│       ├── redirect.go                  # GET /:slug + ClickRecorder
│       ├── redirect_test.go
│       ├── dashboard.go                 # GET /
│       ├── dashboard_test.go
│       ├── links.go                     # POST /links, GET /links/:id, DELETE /links/:id
│       ├── links_test.go
│       ├── health.go                    # GET /healthz
│       └── health_test.go
├── internal/
│   └── assets/
│       ├── assets.go                    # package assets — embed.FS for migrations/templates/static
│       ├── migrations/
│       │   ├── 001_create_links.sql
│       │   └── 002_create_click_events.sql
│       ├── templates/
│       │   ├── base.html                # shared layout: nav, head tags, HTMX/Chart.js/Tailwind CDN
│       │   ├── home.html                # extends base: link table + new-link form area
│       │   ├── link-detail.html         # extends base: link stats, bar chart, referrers
│       │   └── partials/
│       │       ├── link-row.html        # single <tr> for HTMX out-of-band swap
│       │       └── link-form.html       # create-link form for HTMX swap
│       └── static/
│           └── app.css                  # minimal overrides (Tailwind handles most)
├── Dockerfile
├── docker-compose.yml
├── .dockerignore
└── go.mod / go.sum
```

---

## Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: all directories above (empty)

- [ ] **Step 1: Initialize the Go module**

```bash
cd /path/to/snips-replicated
go mod init github.com/ajp-io/snips-replicated
```

- [ ] **Step 2: Create the directory structure**

```bash
mkdir -p cmd/snip \
  internal/config \
  internal/model \
  internal/slug \
  internal/db \
  internal/cache \
  internal/handler \
  templates/partials \
  static \
  migrations \
  docs/superpowers/plans
```

- [ ] **Step 3: Add dependencies**

```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/redis/go-redis/v9@latest
go get github.com/matoous/go-nanoid/v2@latest
go get github.com/stretchr/testify@latest
```

- [ ] **Step 4: Create a placeholder main.go so the module compiles**

Create `cmd/snip/main.go`:
```go
package main

func main() {}
```

- [ ] **Step 5: Verify it compiles**

```bash
go build ./...
```
Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "chore: initialize Go module and add dependencies"
```

---

## Task 2: Config

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:
```go
package config_test

import (
	"os"
	"testing"

	"github.com/ajp-io/snips-replicated/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_AllRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost/snip", cfg.DatabaseURL)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Equal(t, "9090", cfg.Port)
}

func TestLoad_DefaultPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")
	os.Unsetenv("PORT")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("BASE_URL", "http://localhost:8080")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_MissingRedisURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	os.Unsetenv("REDIS_URL")
	t.Setenv("BASE_URL", "http://localhost:8080")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_URL")
}

func TestLoad_MissingBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/snip")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Unsetenv("BASE_URL")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BASE_URL")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/config/...
```
Expected: FAIL — `config` package does not exist yet.

- [ ] **Step 3: Implement config.go**

Create `internal/config/config.go`:
```go
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
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Port:        os.Getenv("PORT"),
		BaseURL:     os.Getenv("BASE_URL"),
	}
	if c.Port == "" {
		c.Port = "8080"
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

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/config/...
```
Expected: PASS, all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config package with env var loading and validation"
```

---

## Task 3: Domain model

**Files:**
- Create: `internal/model/model.go`

No tests needed — pure data types with no logic.

- [ ] **Step 1: Create model.go**

```go
package model

import "time"

// Link represents a shortened URL.
type Link struct {
	ID          int64
	Slug        string
	Destination string
	Label       string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// LinkWithCount is a Link with its total click count, used on the dashboard.
type LinkWithCount struct {
	Link
	TotalClicks int64
}

// DailyClicks is one data point in the clicks-over-time chart.
type DailyClicks struct {
	Day    time.Time
	Clicks int64
}

// ReferrerCount is one row in the top-referrers table.
type ReferrerCount struct {
	Referrer string
	Clicks   int64
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/model/...
```
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/model/
git commit -m "feat: domain model types"
```

---

## Task 4: Slug generation

**Files:**
- Create: `internal/slug/slug.go`
- Create: `internal/slug/slug_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/slug/slug_test.go`:
```go
package slug_test

import (
	"testing"

	"github.com/ajp-io/snips-replicated/internal/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_Length(t *testing.T) {
	s, err := slug.Generate()
	require.NoError(t, err)
	assert.Len(t, s, 6)
}

func TestGenerate_URLSafeChars(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, err := slug.Generate()
		require.NoError(t, err)
		assert.Regexp(t, `^[a-zA-Z0-9]+$`, s)
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		s, err := slug.Generate()
		require.NoError(t, err)
		assert.False(t, seen[s], "duplicate slug: %s", s)
		seen[s] = true
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"abc123", true},
		{"my-link", true},
		{"my_link", true},
		{"ABC", true},
		{"ab", false},        // too short
		{"a b", false},       // space
		{"abc!", false},      // invalid char
		{"", false},          // empty
		{string(make([]byte, 65)), false}, // too long
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, slug.Validate(tt.input), "input: %q", tt.input)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/slug/...
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement slug.go**

Create `internal/slug/slug.go`:
```go
package slug

import (
	"regexp"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var pattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

// Generate returns a random 6-character URL-safe slug.
func Generate() (string, error) {
	return gonanoid.Generate(alphabet, 6)
}

// Validate returns true if s is a valid custom slug.
func Validate(s string) bool {
	return pattern.MatchString(s)
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/slug/...
```
Expected: PASS, all 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/slug/
git commit -m "feat: slug generation and validation"
```

---

## Task 5: SQL migrations

**Files:**
- Create: `internal/assets/migrations/001_create_links.sql`
- Create: `internal/assets/migrations/002_create_click_events.sql`

- [ ] **Step 1: Create 001_create_links.sql**

```sql
CREATE TABLE IF NOT EXISTS links (
    id           BIGSERIAL    PRIMARY KEY,
    slug         VARCHAR(64)  NOT NULL UNIQUE,
    destination  TEXT         NOT NULL,
    label        TEXT,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_links_slug ON links (slug);
```

- [ ] **Step 2: Create 002_create_click_events.sql**

```sql
CREATE TABLE IF NOT EXISTS click_events (
    id         BIGSERIAL   PRIMARY KEY,
    link_id    BIGINT      NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    referrer   TEXT
);

CREATE INDEX IF NOT EXISTS idx_click_events_link_id  ON click_events (link_id);
CREATE INDEX IF NOT EXISTS idx_click_events_clicked_at ON click_events (clicked_at);
```

- [ ] **Step 3: Commit**

```bash
git add internal/assets/migrations/
git commit -m "feat: SQL migrations for links and click_events"
```

---

## Task 6: Store interface + DB layer

**Files:**
- Create: `internal/db/store.go`
- Create: `internal/db/db.go`
- Create: `internal/db/queries.go`
- Create: `internal/db/queries_test.go`

- [ ] **Step 1: Define the Store interface**

Create `internal/db/store.go`:
```go
package db

import (
	"context"
	"time"

	"github.com/ajp-io/snips-replicated/internal/model"
)

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
	Ping(ctx context.Context) error
}
```

- [ ] **Step 2: Implement db.go (pool setup + migration runner)**

Create `internal/db/db.go`:
```go
package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed ../../migrations/*.sql
var migrationsFS embed.FS

// DB wraps a pgxpool.Pool and implements Store.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a pgx connection pool and runs migrations.
func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	d := &DB{pool: pool}
	if err := d.RunMigrations(ctx); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return d, nil
}

// Close closes the connection pool.
func (d *DB) Close() {
	d.pool.Close()
}

// RunMigrations applies all *.sql files from the migrations directory in order.
func (d *DB) RunMigrations(ctx context.Context) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		// Try nested path used when embed path is relative
		entries, err = migrationsFS.ReadDir("../../migrations")
		if err != nil {
			return fmt.Errorf("read migrations dir: %w", err)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		var data []byte
		data, err = migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			data, err = migrationsFS.ReadFile("../../migrations/" + e.Name())
			if err != nil {
				return fmt.Errorf("read %s: %w", e.Name(), err)
			}
		}
		if _, err = d.pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Ping implements Store.
func (d *DB) Ping(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, "SELECT 1")
	return err
}
```

**Note on the embed path:** The `//go:embed` directive path is relative to the file's package directory. Since `db.go` is in `internal/db/`, use `../../migrations/*.sql`. Adjust if you move the file.

Actually, `//go:embed` does not support `..` paths. Use a different approach — embed the migrations in `cmd/snip/main.go` and pass them to `New()`, or move the embed to a package at the repo root.

**Revised approach:** Embed migrations in `cmd/snip/main.go` and pass an `fs.FS` to `db.New()`:

Update `internal/db/db.go`:
```go
package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool.Pool and implements Store.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a pgx connection pool and runs migrations from the provided fs.FS.
func New(ctx context.Context, dsn string, migrationsFS fs.FS) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	d := &DB{pool: pool}
	if err := d.runMigrations(ctx, migrationsFS); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return d, nil
}

// Close closes the connection pool.
func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) runMigrations(ctx context.Context, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err = d.pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Ping implements Store.
func (d *DB) Ping(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, "SELECT 1")
	return err
}
```

- [ ] **Step 3: Implement queries.go**

Create `internal/db/queries.go`:
```go
package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ajp-io/snips-replicated/internal/model"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("not found")

func (d *DB) CreateLink(ctx context.Context, slug, destination, label string, expiresAt *time.Time) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`INSERT INTO links (slug, destination, label, expires_at)
		 VALUES ($1, $2, NULLIF($3, ''), $4)
		 RETURNING id, slug, destination, label, expires_at, created_at`,
		slug, destination, label, expiresAt,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (d *DB) GetLinkBySlug(ctx context.Context, slug string) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, slug, destination, label, expires_at, created_at FROM links WHERE slug = $1`,
		slug,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return link, err
}

func (d *DB) GetLinkByID(ctx context.Context, id int64) (*model.Link, error) {
	link := &model.Link{}
	err := d.pool.QueryRow(ctx,
		`SELECT id, slug, destination, label, expires_at, created_at FROM links WHERE id = $1`,
		id,
	).Scan(&link.ID, &link.Slug, &link.Destination, &link.Label, &link.ExpiresAt, &link.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return link, err
}

func (d *DB) ListLinksWithClickCounts(ctx context.Context) ([]model.LinkWithCount, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT l.id, l.slug, l.destination, l.label, l.expires_at, l.created_at,
		        COUNT(c.id) AS total_clicks
		 FROM links l
		 LEFT JOIN click_events c ON c.link_id = l.id
		 GROUP BY l.id
		 ORDER BY total_clicks DESC, l.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.LinkWithCount
	for rows.Next() {
		var lwc model.LinkWithCount
		if err := rows.Scan(
			&lwc.ID, &lwc.Slug, &lwc.Destination, &lwc.Label,
			&lwc.ExpiresAt, &lwc.CreatedAt, &lwc.TotalClicks,
		); err != nil {
			return nil, err
		}
		results = append(results, lwc)
	}
	return results, rows.Err()
}

func (d *DB) DeleteLink(ctx context.Context, id int64) error {
	tag, err := d.pool.Exec(ctx, `DELETE FROM links WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) RecordClick(ctx context.Context, linkID int64, referrer string) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO click_events (link_id, referrer) VALUES ($1, NULLIF($2, ''))`,
		linkID, referrer,
	)
	return err
}

func (d *DB) GetDailyClicks(ctx context.Context, linkID int64) ([]model.DailyClicks, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT DATE(clicked_at) AS day, COUNT(*) AS clicks
		 FROM click_events
		 WHERE link_id = $1 AND clicked_at > NOW() - INTERVAL '30 days'
		 GROUP BY day ORDER BY day`,
		linkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.DailyClicks
	for rows.Next() {
		var dc model.DailyClicks
		if err := rows.Scan(&dc.Day, &dc.Clicks); err != nil {
			return nil, err
		}
		results = append(results, dc)
	}
	return results, rows.Err()
}

func (d *DB) GetTopReferrers(ctx context.Context, linkID int64) ([]model.ReferrerCount, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT COALESCE(NULLIF(referrer, ''), '(direct)') AS referrer, COUNT(*) AS clicks
		 FROM click_events
		 WHERE link_id = $1
		 GROUP BY referrer ORDER BY clicks DESC LIMIT 10`,
		linkID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.ReferrerCount
	for rows.Next() {
		var rc model.ReferrerCount
		if err := rows.Scan(&rc.Referrer, &rc.Clicks); err != nil {
			return nil, err
		}
		results = append(results, rc)
	}
	return results, rows.Err()
}
```

- [ ] **Step 4: Write integration tests**

Create `internal/db/queries_test.go`:
```go
//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	// Build an in-memory FS with the migration files
	m1, _ := os.ReadFile("../assets/migrations/001_create_links.sql")
	m2, _ := os.ReadFile("../assets/migrations/002_create_click_events.sql")
	migrationsFS := fstest.MapFS{
		"001_create_links.sql":       &fstest.MapFile{Data: m1},
		"002_create_click_events.sql": &fstest.MapFile{Data: m2},
	}
	store, err := db.New(context.Background(), dsn, migrationsFS)
	require.NoError(t, err)
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestCreateAndGetLink(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	link, err := store.CreateLink(ctx, "test01", "https://example.com", "Test", nil)
	require.NoError(t, err)
	assert.Equal(t, "test01", link.Slug)
	assert.Equal(t, "https://example.com", link.Destination)

	got, err := store.GetLinkBySlug(ctx, "test01")
	require.NoError(t, err)
	assert.Equal(t, link.ID, got.ID)

	// cleanup
	require.NoError(t, store.DeleteLink(ctx, link.ID))
}

func TestGetLinkBySlug_NotFound(t *testing.T) {
	store := newTestDB(t)
	_, err := store.GetLinkBySlug(context.Background(), "doesnotexist")
	assert.ErrorIs(t, err, db.ErrNotFound)
}

func TestRecordClickAndGetDailyClicks(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	link, err := store.CreateLink(ctx, "clk01", "https://example.com/click", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { store.DeleteLink(ctx, link.ID) })

	require.NoError(t, store.RecordClick(ctx, link.ID, "https://news.ycombinator.com"))
	require.NoError(t, store.RecordClick(ctx, link.ID, "https://news.ycombinator.com"))
	require.NoError(t, store.RecordClick(ctx, link.ID, ""))

	daily, err := store.GetDailyClicks(ctx, link.ID)
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.Equal(t, int64(3), daily[0].Clicks)

	refs, err := store.GetTopReferrers(ctx, link.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://news.ycombinator.com", refs[0].Referrer)
	assert.Equal(t, int64(2), refs[0].Clicks)
	assert.Equal(t, "(direct)", refs[1].Referrer)
}

func TestListLinksWithClickCounts(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	l1, err := store.CreateLink(ctx, "list01", "https://a.com", "", nil)
	require.NoError(t, err)
	l2, err := store.CreateLink(ctx, "list02", "https://b.com", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		store.DeleteLink(ctx, l1.ID)
		store.DeleteLink(ctx, l2.ID)
	})

	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))
	require.NoError(t, store.RecordClick(ctx, l1.ID, ""))

	links, err := store.ListLinksWithClickCounts(ctx)
	require.NoError(t, err)
	// l1 should appear first (most clicks)
	var found bool
	for _, lc := range links {
		if lc.Slug == "list01" {
			assert.Equal(t, int64(2), lc.TotalClicks)
			found = true
		}
	}
	assert.True(t, found)
}

func TestLinkExpiry(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	link, err := store.CreateLink(ctx, "exp01", "https://example.com", "", &past)
	require.NoError(t, err)
	t.Cleanup(func() { store.DeleteLink(ctx, link.ID) })

	got, err := store.GetLinkBySlug(ctx, "exp01")
	require.NoError(t, err)
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, got.ExpiresAt.Before(time.Now()))
}
```

- [ ] **Step 5: Run integration tests (requires docker-compose up)**

```bash
# Start postgres first:
docker-compose up -d postgres

# Run with tag:
TEST_DATABASE_URL="postgres://snip:snip@localhost:5432/snip?sslmode=disable" \
  go test -tags integration ./internal/db/...
```
Expected: PASS, all 5 tests.

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat: db layer — Store interface, pgx pool, SQL queries"
```

---

## Task 7: Cache layer

**Files:**
- Create: `internal/cache/cache.go`
- Create: `internal/cache/cache_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/cache/cache_test.go`:
```go
//go:build integration

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(t *testing.T) cache.Cache {
	t.Helper()
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	c, err := cache.New(redisURL)
	require.NoError(t, err)
	return c
}

func TestSetAndGet(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	err := c.Set(ctx, "test-key", "test-value", time.Minute)
	require.NoError(t, err)

	val, ok, err := c.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "test-value", val)
}

func TestGet_Miss(t *testing.T) {
	c := newTestCache(t)
	_, ok, err := c.Get(context.Background(), "key-that-does-not-exist-xyz")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDel(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "del-key", "val", time.Minute))
	require.NoError(t, c.Del(ctx, "del-key"))

	_, ok, err := c.Get(ctx, "del-key")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPing(t *testing.T) {
	c := newTestCache(t)
	assert.NoError(t, c.Ping(context.Background()))
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test -tags integration ./internal/cache/...
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement cache.go**

Create `internal/cache/cache.go`:
```go
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is the interface handlers use for caching operations.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Ping(ctx context.Context) error
}

// RedisCache implements Cache using go-redis.
type RedisCache struct {
	client *redis.Client
}

// New creates a RedisCache from a Redis URL (e.g. "redis://localhost:6379").
func New(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &RedisCache{client: redis.NewClient(opts)}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
```

- [ ] **Step 4: Run integration tests**

```bash
docker-compose up -d redis

TEST_REDIS_URL="redis://localhost:6379" \
  go test -tags integration ./internal/cache/...
```
Expected: PASS, all 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/
git commit -m "feat: cache layer — Cache interface and Redis implementation"
```

---

## Task 8: Health handler

**Files:**
- Create: `internal/handler/health.go`
- Create: `internal/handler/health_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/handler/health_test.go`:
```go
package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajp-io/snips-replicated/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStore implements db.Store for testing — only Ping is needed for health.
type stubStore struct {
	pingErr error
}

func (s *stubStore) Ping(ctx context.Context) error { return s.pingErr }

// implement remaining Store methods as no-ops
func (s *stubStore) CreateLink(ctx context.Context, slug, dest, label string, exp *time.Time) (*model.Link, error) { return nil, nil }
// ... (see full stub in Task 9 — re-use the same stub)

// stubCache implements cache.Cache for testing.
type stubCache struct {
	pingErr error
}

func (c *stubCache) Ping(ctx context.Context) error                                       { return c.pingErr }
func (c *stubCache) Get(ctx context.Context, key string) (string, bool, error)            { return "", false, nil }
func (c *stubCache) Set(ctx context.Context, k, v string, ttl time.Duration) error        { return nil }
func (c *stubCache) Del(ctx context.Context, key string) error                            { return nil }

func TestHealthHandler_AllOK(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{}, &stubCache{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	checks := body["checks"].(map[string]interface{})
	assert.Equal(t, "ok", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestHealthHandler_DBDown(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{pingErr: errors.New("conn refused")}, &stubCache{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body["status"])
	checks := body["checks"].(map[string]interface{})
	assert.Equal(t, "error", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestHealthHandler_CacheDown(t *testing.T) {
	h := handler.NewHealthHandler(&stubStore{}, &stubCache{pingErr: errors.New("conn refused")})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/handler/...
```
Expected: FAIL — handler package does not exist.

- [ ] **Step 3: Implement health.go**

Create `internal/handler/health.go`:
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ajp-io/snips-replicated/internal/cache"
	"github.com/ajp-io/snips-replicated/internal/db"
)

// HealthHandler serves GET /healthz.
type HealthHandler struct {
	store db.Store
	cache cache.Cache
}

func NewHealthHandler(store db.Store, cache cache.Cache) *HealthHandler {
	return &HealthHandler{store: store, cache: cache}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	if err := h.store.Ping(ctx); err != nil {
		dbStatus = "error"
	}
	cacheStatus := "ok"
	if err := h.cache.Ping(ctx); err != nil {
		cacheStatus = "error"
	}

	overall := "ok"
	statusCode := http.StatusOK
	if dbStatus == "error" || cacheStatus == "error" {
		overall = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": overall,
		"checks": map[string]string{
			"database": dbStatus,
			"cache":    cacheStatus,
		},
	})
}
```

- [ ] **Step 4: Fix the test file — add missing model import and complete stubStore**

Update `internal/handler/health_test.go` to import model and complete the stub. The stub needs all Store methods. Create a shared test helper file `internal/handler/stubs_test.go`:

```go
package handler_test

import (
	"context"
	"time"

	"github.com/ajp-io/snips-replicated/internal/model"
)

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

type stubCache struct {
	pingErr error
	val     string
	hit     bool
}

func (c *stubCache) Ping(_ context.Context) error                                    { return c.pingErr }
func (c *stubCache) Get(_ context.Context, key string) (string, bool, error)         { return c.val, c.hit, nil }
func (c *stubCache) Set(_ context.Context, k, v string, ttl time.Duration) error     { return nil }
func (c *stubCache) Del(_ context.Context, key string) error                         { return nil }
```

Remove the duplicate stub definitions from `health_test.go` and keep only the test functions there.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/handler/...
```
Expected: PASS, 3 health tests.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/
git commit -m "feat: health handler with DB and cache ping checks"
```

---

## Task 9: Assets package + base HTML templates

**Files:**
- Create: `internal/assets/assets.go`
- Create: `internal/assets/templates/base.html`
- Create: `internal/assets/static/app.css`

No tests — templates and assets are verified visually in Tasks 10–12.

- [ ] **Step 1: Create internal/assets/assets.go**

```go
package assets

import "embed"

//go:embed migrations
var Migrations embed.FS

//go:embed templates
var Templates embed.FS

//go:embed static
var Static embed.FS
```

- [ ] **Step 2: Create internal/assets/static/app.css**

```css
/* Minimal overrides — Tailwind CDN handles most styling */
[x-cloak] { display: none !important; }

.htmx-indicator { opacity: 0; transition: opacity 200ms ease-in; }
.htmx-request .htmx-indicator { opacity: 1; }
.htmx-request.htmx-indicator { opacity: 1; }
```

- [ ] **Step 3: Create internal/assets/templates/base.html**

```html
<!DOCTYPE html>
<html lang="en" class="bg-gray-950 text-gray-100">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{block "title" .}}Snip{{end}}</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.2/dist/chart.umd.min.js"></script>
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="min-h-screen">
  <nav class="border-b border-gray-800 px-6 py-4 flex items-center justify-between">
    <a href="/" class="text-indigo-400 font-bold text-xl tracking-tight">✂ Snip</a>
    <button
      hx-get="/links/new"
      hx-target="#link-form-container"
      hx-swap="innerHTML"
      class="bg-indigo-600 hover:bg-indigo-700 text-white text-sm px-4 py-2 rounded-md">
      + New Link
    </button>
  </nav>
  <main class="max-w-5xl mx-auto px-6 py-8">
    <div id="link-form-container"></div>
    {{block "content" .}}{{end}}
  </main>
</body>
</html>
```

- [ ] **Step 4: Commit**

```bash
git add internal/assets/
git commit -m "feat: assets package with migrations, base HTML template, and static CSS"
```

---

## Task 10: Dashboard handler + home template

**Files:**
- Create: `internal/handler/dashboard.go`
- Create: `internal/handler/dashboard_test.go`
- Create: `internal/assets/templates/home.html`
- Create: `internal/assets/templates/partials/link-row.html`
- Create: `internal/assets/templates/partials/link-form.html`

- [ ] **Step 1: Write the failing tests**

Create `internal/handler/dashboard_test.go`:
```go
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
	"github.com/stretchr/testify/require"
)

func TestDashboardHandler_RendersList(t *testing.T) {
	store := &stubStore{
		links: []model.LinkWithCount{
			{Link: model.Link{ID: 1, Slug: "abc", Destination: "https://example.com", CreatedAt: time.Now()}, TotalClicks: 42},
		},
	}
	// Use a minimal template for testing — avoids needing real template files
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
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/handler/... -run TestDashboard
```
Expected: FAIL — `NewDashboardHandler` not defined.

- [ ] **Step 3: Implement dashboard.go**

Create `internal/handler/dashboard.go`:
```go
package handler

import (
	"html/template"
	"net/http"

	"github.com/ajp-io/snips-replicated/internal/db"
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
```

Add the missing import in `dashboard.go`:
```go
import (
	"html/template"
	"net/http"

	"github.com/ajp-io/snips-replicated/internal/db"
	"github.com/ajp-io/snips-replicated/internal/model"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/handler/... -run TestDashboard
```
Expected: PASS, 2 tests.

- [ ] **Step 5: Create home.html**

Create `templates/home.html`:
```html
{{template "base" .}}
{{define "content"}}
<div class="mt-6">
  {{if .Links}}
  <table class="w-full text-sm">
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
  {{else}}
  <p class="text-gray-500 text-center py-16">No links yet. Create one above.</p>
  {{end}}
</div>
{{end}}
```

- [ ] **Step 6: Create templates/partials/link-row.html**

```html
{{define "link-row"}}
<tr class="border-b border-gray-800 hover:bg-gray-900">
  <td class="py-3 pr-4 font-mono text-indigo-400">{{.Slug}}</td>
  <td class="py-3 pr-4 text-gray-400 max-w-xs truncate">{{.Destination}}</td>
  <td class="py-3 pr-4 text-gray-300">{{if .Label}}{{.Label}}{{else}}<span class="text-gray-600">—</span>{{end}}</td>
  <td class="py-3 pr-4 text-emerald-400 font-semibold">{{.TotalClicks}}</td>
  <td class="py-3 pr-4 text-gray-400">
    {{if .ExpiresAt}}{{.ExpiresAt.Format "Jan 2"}}{{else}}<span class="text-gray-600">—</span>{{end}}
  </td>
  <td class="py-3">
    <a href="/links/{{.ID}}" class="text-indigo-400 hover:text-indigo-300 text-xs">View</a>
  </td>
</tr>
{{end}}
```

- [ ] **Step 7: Create templates/partials/link-form.html**

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
    <div>
      <label class="block text-xs text-gray-400 mb-1">Custom slug (optional)</label>
      <input type="text" name="slug" placeholder="my-link"
        class="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm focus:outline-none focus:border-indigo-500">
    </div>
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

- [ ] **Step 8: Commit**

```bash
git add internal/handler/dashboard.go internal/handler/dashboard_test.go internal/assets/templates/
git commit -m "feat: dashboard handler and home/partial templates"
```

---

## Task 11: Redirect handler + async click recorder

**Files:**
- Create: `internal/handler/redirect.go`
- Create: `internal/handler/redirect_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/handler/redirect_test.go`:
```go
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
	cache := &stubCache{val: "https://example.com", hit: true}
	store := &stubStore{}
	recorder := handler.NewClickRecorder(store, 10)
	defer recorder.Shutdown()

	h := handler.NewRedirectHandler(store, cache, recorder)

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

	h := handler.NewRedirectHandler(store, cache, recorder)
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

	h := handler.NewRedirectHandler(store, cache, recorder)
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

	h := handler.NewRedirectHandler(store, cache, recorder)
	r := chi.NewRouter()
	r.Get("/{slug}", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/exp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/handler/... -run TestRedirect
```
Expected: FAIL — `NewRedirectHandler` and `NewClickRecorder` not defined.

- [ ] **Step 3: Implement redirect.go**

Create `internal/handler/redirect.go`:
```go
package handler

import (
	"context"
	"errors"
	"net/http"
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

// RedirectHandler serves GET /:slug.
type RedirectHandler struct {
	store    db.Store
	cache    cache.Cache
	recorder *ClickRecorder
}

func NewRedirectHandler(store db.Store, cache cache.Cache, recorder *ClickRecorder) *RedirectHandler {
	return &RedirectHandler{store: store, cache: cache, recorder: recorder}
}

func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	// 1. Try cache first.
	if destination, ok, _ := h.cache.Get(ctx, cacheKeyPrefix+slug); ok {
		h.recorder.Record(0, r.Referer()) // link ID unknown from cache-only path; use 0 to skip DB write
		http.Redirect(w, r, destination, http.StatusFound)
		return
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

	// 4. Populate cache. TTL = 1 hour, or time until expiry if shorter.
	ttl := time.Hour
	if link.ExpiresAt != nil {
		if remaining := time.Until(*link.ExpiresAt); remaining < ttl {
			ttl = remaining
		}
	}
	_ = h.cache.Set(ctx, cacheKeyPrefix+slug, link.Destination, ttl)

	// 5. Record click asynchronously.
	h.recorder.Record(link.ID, r.Referer())

	http.Redirect(w, r, link.Destination, http.StatusFound)
}
```

**Note on cache-only path:** When link ID is not known (cache hit), pass `0` as linkID — `RecordClick` will fail silently since linkID 0 violates the FK constraint. To record clicks from cache hits accurately, store the link ID in the cache value as `"<id>|<url>"` and parse it. This is a simple optimization:

Update the cache value format — store `"<id>:<destination>"` and parse on cache hit:

```go
import (
	"fmt"
	"strconv"
	"strings"
)

// encodeCache encodes link ID + destination for Redis storage.
func encodeCache(id int64, destination string) string {
	return fmt.Sprintf("%d|%s", id, destination)
}

// decodeCache parses the cached value. Returns 0, "" on parse error.
func decodeCache(val string) (int64, string) {
	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return 0, val // legacy: plain URL
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, val
	}
	return id, parts[1]
}
```

Update the cache hit section of `ServeHTTP`:
```go
if raw, ok, _ := h.cache.Get(ctx, cacheKeyPrefix+slug); ok {
	id, destination := decodeCache(raw)
	if destination != "" {
		h.recorder.Record(id, r.Referer())
		http.Redirect(w, r, destination, http.StatusFound)
		return
	}
}
```

Update the cache set section:
```go
_ = h.cache.Set(ctx, cacheKeyPrefix+slug, encodeCache(link.ID, link.Destination), ttl)
```

Also update the cache stub in `stubs_test.go` to return an encoded value in the cache-hit test:
```go
// In TestRedirectHandler_CacheHit, set cache val to encoded format:
cache := &stubCache{val: "1|https://example.com", hit: true}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/handler/... -run TestRedirect
```
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/redirect.go internal/handler/redirect_test.go
git commit -m "feat: redirect handler with cache-first lookup and async click recording"
```

---

## Task 12: Links handler (create, detail, delete)

**Files:**
- Create: `internal/handler/links.go`
- Create: `internal/handler/links_test.go`
- Create: `internal/assets/templates/link-detail.html`

- [ ] **Step 1: Write the failing tests**

Create `internal/handler/links_test.go`:
```go
package handler_test

import (
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

func TestCreateLink_AutoSlug(t *testing.T) {
	store := &stubStore{link: &model.Link{ID: 1, Slug: "gen01", Destination: "https://example.com", CreatedAt: time.Now()}}
	rowTmpl := template.Must(template.New("link-row").Parse(`{{.Slug}}`))
	h := handler.NewLinksHandler(store, &stubCache{}, rowTmpl, nil, "http://localhost")

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=&label=")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "gen01")
}

func TestCreateLink_CustomSlug(t *testing.T) {
	store := &stubStore{link: &model.Link{ID: 2, Slug: "my-link", Destination: "https://example.com", CreatedAt: time.Now()}}
	rowTmpl := template.Must(template.New("link-row").Parse(`{{.Slug}}`))
	h := handler.NewLinksHandler(store, &stubCache{}, rowTmpl, nil, "http://localhost")

	form := strings.NewReader("destination=https%3A%2F%2Fexample.com&slug=my-link&label=")
	req := httptest.NewRequest(http.MethodPost, "/links", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Create(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateLink_InvalidSlug(t *testing.T) {
	store := &stubStore{}
	formTmpl := template.Must(template.New("link-form").Parse(`error:{{.Error}}`))
	h := handler.NewLinksHandler(store, &stubCache{}, nil, formTmpl, "http://localhost")

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
	h := handler.NewLinksHandler(store, &stubCache{}, nil, nil, "http://localhost")
	// Override the detail template
	hWithDetail := handler.NewLinksHandler(store, &stubCache{}, nil, detailTmpl, "http://localhost")

	r := chi.NewRouter()
	r.Get("/links/{id}", hWithDetail.Detail)

	req := httptest.NewRequest(http.MethodGet, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "abc")
	_ = h
}

func TestDeleteLink(t *testing.T) {
	store := &stubStore{}
	h := handler.NewLinksHandler(store, &stubCache{}, nil, nil, "http://localhost")

	r := chi.NewRouter()
	r.Delete("/links/{id}", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestDeleteLink_NotFound(t *testing.T) {
	store := &stubStore{deleteErr: db.ErrNotFound}
	h := handler.NewLinksHandler(store, &stubCache{}, nil, nil, "http://localhost")

	r := chi.NewRouter()
	r.Delete("/links/{id}", h.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/links/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/handler/... -run TestCreateLink -run TestDetailLink -run TestDeleteLink
```
Expected: FAIL — `NewLinksHandler` not defined.

- [ ] **Step 3: Implement links.go**

Create `internal/handler/links.go`:
```go
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
	rowTmpl    *template.Template // "link-row" partial — returned on successful create
	detailTmpl *template.Template // "link-detail" full page
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

	// Resolve slug.
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

	// Return the new table row for HTMX to prepend.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := model.LinkWithCount{Link: *link}
	h.rowTmpl.ExecuteTemplate(w, "link-row", data)
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

	// Evict from cache.
	if link != nil {
		_ = h.cache.Del(r.Context(), cacheKeyPrefix+link.Slug)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/handler/...
```
Expected: PASS, all handler tests.

- [ ] **Step 5: Create templates/link-detail.html**

```html
{{template "base" .}}
{{define "title"}}{{.Link.Slug}} — Snip{{end}}
{{define "content"}}
<div class="mt-6">
  <a href="/" class="text-indigo-400 text-sm hover:text-indigo-300">← All links</a>

  <!-- Link info card -->
  <div class="bg-gray-900 border border-gray-800 rounded-lg p-6 mt-4">
    <div class="flex justify-between items-start">
      <div>
        <div class="flex items-center gap-3 mb-2">
          <span class="font-mono text-2xl font-bold text-indigo-400">{{.Link.Slug}}</span>
          {{if .Link.Label}}<span class="bg-indigo-950 text-indigo-300 text-xs px-2 py-1 rounded">{{.Link.Label}}</span>{{end}}
        </div>
        <p class="text-gray-400 text-sm break-all">→ {{.Link.Destination}}</p>
      </div>
      <div class="text-right">
        <div class="text-4xl font-black text-emerald-400">{{len .Daily}}</div>
        <div class="text-xs text-gray-500">days active</div>
      </div>
    </div>
    <div class="flex gap-6 mt-4 pt-4 border-t border-gray-800 text-xs text-gray-500">
      <span>Created <span class="text-gray-300">{{.Link.CreatedAt.Format "Jan 2, 2006"}}</span></span>
      <span>Expires <span class="text-gray-300">{{if .Link.ExpiresAt}}{{.Link.ExpiresAt.Format "Jan 2, 2006"}}{{else}}—{{end}}</span></span>
      <span>Short URL <span class="text-indigo-400 font-mono">{{.ShortURL}}</span></span>
      <span class="ml-auto">
        <form method="POST" action="/links/{{.Link.ID}}" onsubmit="return confirm('Delete this link?')">
          <input type="hidden" name="_method" value="DELETE">
          <button
            hx-delete="/links/{{.Link.ID}}"
            hx-confirm="Delete this link?"
            hx-push-url="/"
            class="text-red-500 hover:text-red-400">Delete</button>
        </form>
      </span>
    </div>
  </div>

  <!-- Clicks over time chart -->
  <div class="bg-gray-900 border border-gray-800 rounded-lg p-6 mt-4">
    <h3 class="text-sm font-semibold mb-4">Clicks over time (last 30 days)</h3>
    <canvas id="clicks-chart" height="80"></canvas>
  </div>

  <!-- Top referrers -->
  <div class="bg-gray-900 border border-gray-800 rounded-lg p-6 mt-4">
    <h3 class="text-sm font-semibold mb-4">Top referrers</h3>
    {{if .Referrers}}
    <div class="space-y-3">
      {{range .Referrers}}
      <div class="flex items-center gap-3">
        <span class="font-mono text-xs text-gray-400 w-48 truncate">{{.Referrer}}</span>
        <div class="flex-1 bg-gray-800 rounded h-1.5">
          <div class="bg-indigo-500 h-1.5 rounded" style="width: {{.Clicks}}%"></div>
        </div>
        <span class="text-sm font-semibold text-gray-200 w-10 text-right">{{.Clicks}}</span>
      </div>
      {{end}}
    </div>
    {{else}}
    <p class="text-gray-600 text-sm">No clicks recorded yet.</p>
    {{end}}
  </div>
</div>

<script>
(function() {
  const labels = [{{range .Daily}}"{{.Day.Format "Jan 2"}}",{{end}}];
  const data   = [{{range .Daily}}{{.Clicks}},{{end}}];
  new Chart(document.getElementById('clicks-chart'), {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        data,
        backgroundColor: '#4f46e5',
        borderRadius: 3,
      }]
    },
    options: {
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: '#6b7280' }, grid: { color: '#1f2937' } },
        y: { ticks: { color: '#6b7280' }, grid: { color: '#1f2937' } }
      }
    }
  });
})();
</script>
{{end}}
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/links.go internal/handler/links_test.go internal/assets/templates/link-detail.html
git commit -m "feat: links handler (create, detail, delete) and link-detail template"
```

---

## Task 13: Main entry point

**Files:**
- Create: `cmd/snip/main.go`

- [ ] **Step 1: Implement main.go**

Replace the placeholder `cmd/snip/main.go`:

```go
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

	// Templates
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
	staticSub, _ := fs.Sub(assets.Static, "static")
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
		Addr:    ":" + cfg.Port,
		Handler: r,
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

- [ ] **Step 2: Verify the project builds**

```bash
go build ./...
```
Expected: no errors, exit 0.

- [ ] **Step 3: Run unit tests**

```bash
go test ./...
```
Expected: PASS for all non-integration tests.

- [ ] **Step 4: Commit**

```bash
git add cmd/snip/main.go
git commit -m "feat: main entry point — wires config, db, cache, handlers, and router"
```

---

## Task 14: Dockerfile and docker-compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `.dockerignore`

- [ ] **Step 1: Create .dockerignore**

```
.git
.gitignore
docs/
*.md
.superpowers/
```

- [ ] **Step 2: Create Dockerfile**

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /snip ./cmd/snip

# Runtime stage
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /snip /app/snip
EXPOSE 8080
ENTRYPOINT ["/app/snip"]
```

- [ ] **Step 3: Create docker-compose.yml**

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://snip:snip@postgres:5432/snip?sslmode=disable
      REDIS_URL: redis://redis:6379
      BASE_URL: http://localhost:8080
      PORT: "8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: snip
      POSTGRES_PASSWORD: snip
      POSTGRES_DB: snip
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U snip"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres-data:
```

- [ ] **Step 4: Verify Docker build**

```bash
docker build -t snip:local .
```
Expected: successful build, image tagged `snip:local`.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile docker-compose.yml .dockerignore
git commit -m "chore: Dockerfile and docker-compose for local development"
```

---

## Task 15: End-to-end smoke test

- [ ] **Step 1: Start all services**

```bash
docker-compose up --build -d
```
Expected: all 3 containers start. Watch logs: `docker-compose logs -f app`

- [ ] **Step 2: Health check**

```bash
curl -s http://localhost:8080/healthz | jq .
```
Expected:
```json
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "cache": "ok"
  }
}
```

- [ ] **Step 3: Create a link via the API**

```bash
curl -s -X POST http://localhost:8080/links \
  -d "destination=https://github.com&label=GitHub" \
  -H "Content-Type: application/x-www-form-urlencoded"
```
Expected: HTML fragment containing the new link's slug.

- [ ] **Step 4: Open the dashboard**

Open `http://localhost:8080` in a browser.
Expected: the link table shows the new link ranked by clicks.

- [ ] **Step 5: Test redirect**

```bash
# Note the slug from Step 3, e.g. "abc123"
curl -s -o /dev/null -w "%{http_code} %{redirect_url}" http://localhost:8080/<slug>
```
Expected: `302 https://github.com`

- [ ] **Step 6: Verify click was recorded**

Open `http://localhost:8080/links/<id>` in a browser.
Expected: total clicks shows 1, chart shows today's bar, referrer shows `(direct)`.

- [ ] **Step 7: Test expiration**

```bash
# Create a link with a past expiration date (can't do via form — use psql)
docker-compose exec postgres psql -U snip -c \
  "INSERT INTO links (slug, destination, expires_at) VALUES ('expired', 'https://example.com', NOW() - INTERVAL '1 hour');"

curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/expired
```
Expected: `410`

- [ ] **Step 8: Test the health endpoint with Redis down**

```bash
docker-compose stop redis
curl -s http://localhost:8080/healthz | jq .
```
Expected: `503` with `"cache": "error"`.

```bash
docker-compose start redis
```

- [ ] **Step 9: Run all unit tests one final time**

```bash
go test ./...
```
Expected: PASS, all tests.

- [ ] **Step 10: Run integration tests**

```bash
TEST_DATABASE_URL="postgres://snip:snip@localhost:5432/snip?sslmode=disable" \
TEST_REDIS_URL="redis://localhost:6379" \
  go test -tags integration ./...
```
Expected: PASS, all integration tests.

- [ ] **Step 11: Final commit**

```bash
git add -A
git commit -m "feat: Snip application — URL shortener with analytics dashboard"
```

---

## Notes for the implementer

### Embed paths — important

Go's `//go:embed` does **not** support `..` path elements. Since `cmd/snip/main.go` sits two levels below the repo root, it cannot directly embed `migrations/`, `templates/`, or `static/`.

**Fix:** Create `internal/assets/assets.go` and move the asset directories there:

```
internal/assets/
├── assets.go          # package assets — embed directives live here
├── migrations/        # move from repo root
├── templates/         # move from repo root
└── static/            # move from repo root
```

`internal/assets/assets.go`:
```go
package assets

import "embed"

//go:embed migrations
var Migrations embed.FS

//go:embed templates
var Templates embed.FS

//go:embed static
var Static embed.FS
```

Update `main.go` to import `github.com/ajp-io/snips-replicated/internal/assets` and use `assets.Migrations`, `assets.Templates`, `assets.Static`. Replace the three `//go:embed` directives and their `embed.FS` vars in `main.go` with calls to the `assets` package.

Update `db.New()` call: `db.New(ctx, cfg.DatabaseURL, assets.Migrations)` — but since `assets.Migrations` is an `embed.FS` rooted at `migrations/`, pass a sub-FS: `migrFS, _ := fs.Sub(assets.Migrations, "migrations")`.

Similarly for templates: `template.ParseFS(assets.Templates, "templates/*.html", "templates/partials/*.html")`.

Update `internal/db/queries_test.go` to use `internal/assets` instead of reading migration files directly.

### Template execution

The `DashboardHandler` and `LinksHandler` both accept `*template.Template` — in `main.go`, pass the same parsed template set to both. Use `tmpl.ExecuteTemplate(w, "home", data)` (not `tmpl.Execute`) since templates use `{{define "content"}}` blocks.

### Integration tests

Integration tests use `//go:build integration` and are skipped when `TEST_DATABASE_URL` / `TEST_REDIS_URL` are not set — safe to run `go test ./...` without Docker running.
