# Snip — Application Design Spec

**Date:** 2026-04-06  
**Status:** Approved

---

## Overview

Snip is a self-hosted URL shortening service with built-in click analytics. It is the application component of a Replicated Bootcamp project — the app itself is Tier 0 work; subsequent tiers wrap it in Helm, Replicated SDK, KOTS, and Embedded Cluster.

---

## Technology Stack

| Concern | Choice |
|---------|--------|
| Language | Go 1.23 |
| HTTP router | `go-chi/chi/v5` |
| UI rendering | `html/template` + HTMX (via CDN) |
| Charts | Chart.js (via CDN) |
| Styling | Tailwind CSS (via CDN) |
| Database driver | `jackc/pgx/v5` |
| Cache client | `redis/go-redis/v9` |
| Slug generation | `matoous/go-nanoid/v2` |
| Config | Environment variables |
| Migrations | Plain SQL files, applied at startup via `pgx` |
| Container | Multi-stage: `golang:1.23-alpine` → `alpine:3.20` |
| Asset embedding | `//go:embed` for templates and static files |

---

## Architecture

A single Go binary serves all traffic: short-link redirects, the dashboard UI, and the health endpoint. No separate frontend service.

```
Browser
  │
  ▼
snip (Go binary)
  ├── Chi router
  ├── html/template + HTMX  ← dashboard UI
  ├── Chart.js              ← analytics charts (CDN)
  │
  ├── Redis                 ← read-through cache (slug → URL)
  └── PostgreSQL            ← persistent storage
```

### Redirect path (performance-critical)

1. Incoming request: `GET /:slug`
2. Check Redis for `slug:abc123` → destination URL
3. Cache miss → query PostgreSQL → write to Redis with TTL (1 hour, or `expires_at - now` if shorter)
4. Check expiration — if expired, return 404
5. Record click event **asynchronously** (buffered channel + goroutine) to avoid adding latency to the redirect
6. Return `HTTP 302` to destination URL

### Click recording

Click events are written to a buffered channel. A background goroutine drains the channel and batch-inserts into `click_events`. This decouples the redirect hot path from the database write. On shutdown, the buffer is flushed before exit.

---

## HTTP Routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/:slug` | Redirect to destination; record click |
| `GET` | `/` | Dashboard home: all links ranked by total clicks |
| `GET` | `/links/new` | Create link form (rendered inline via HTMX) |
| `POST` | `/links` | Create a new link; returns redirect to `/` |
| `GET` | `/links/:id` | Link detail: info + click chart + top referrers |
| `DELETE` | `/links/:id` | Delete a link and its click events |
| `GET` | `/healthz` | Structured JSON health check |

The `/:slug` route is registered last so it doesn't shadow `/links`, `/healthz`, etc.

---

## Data Model

### `links`

```sql
CREATE TABLE links (
    id             BIGSERIAL PRIMARY KEY,
    slug           VARCHAR(64)  NOT NULL UNIQUE,
    destination    TEXT         NOT NULL,
    label          TEXT,
    expires_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_links_slug ON links (slug);
```

### `click_events`

```sql
CREATE TABLE click_events (
    id         BIGSERIAL PRIMARY KEY,
    link_id    BIGINT      NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    referrer   TEXT
);
CREATE INDEX idx_click_events_link_id ON click_events (link_id);
CREATE INDEX idx_click_events_clicked_at ON click_events (clicked_at);
```

`ON DELETE CASCADE` ensures click events are cleaned up when a link is deleted.

---

## Analytics Queries

All analytics are plain SQL aggregates over `click_events`.

**Total clicks per link** (used on the dashboard home):
```sql
SELECT link_id, COUNT(*) AS total FROM click_events GROUP BY link_id
```

**Clicks per day for a link** (chart data, last 30 days):
```sql
SELECT DATE(clicked_at) AS day, COUNT(*) AS clicks
FROM click_events
WHERE link_id = $1 AND clicked_at > NOW() - INTERVAL '30 days'
GROUP BY day ORDER BY day
```

**Top referrers for a link** (top 10):
```sql
SELECT COALESCE(NULLIF(referrer, ''), '(direct)') AS referrer, COUNT(*) AS clicks
FROM click_events
WHERE link_id = $1
GROUP BY referrer ORDER BY clicks DESC LIMIT 10
```

---

## Slug Generation

Auto-generated slugs use nanoid with a URL-safe alphabet and length 6 (e.g., `abc123`, `x9Kq2m`). Custom slugs are accepted if they match `^[a-zA-Z0-9_-]{3,64}$` and are not already taken. On collision for auto-generated slugs, retry up to 5 times before returning a 500.

---

## Dashboard UI

### Home screen (`GET /`)

- Top nav: `✂ Snip` wordmark + `+ New Link` button
- Table of all links, ordered by total click count descending
- Columns: Slug, Destination (truncated), Label, Clicks, Expires, View link
- "New Link" opens an inline form below the nav via HTMX (no full page reload)
- Empty state when no links exist

### Create link form

Inline HTMX form (swapped into the page, no modal):
- **Destination URL** (required, validated as URL)
- **Custom slug** (optional; auto-generated if blank)
- **Label** (optional)
- **Expiration date** (optional, date picker)
- Submit button; on success the table row is prepended via HTMX swap

### Link detail page (`GET /links/:id`)

- Back link → home
- Header card: slug, label badge, destination URL, total click count, created date, expiry, delete button
- Bar chart (Chart.js): clicks per day for last 30 days
- Top referrers table: referrer hostname + inline bar + click count

---

## Health Endpoint

`GET /healthz` returns `200 OK` with JSON when healthy, `503` when any check fails:

```json
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "cache": "ok"
  }
}
```

Checks:
- **database**: `SELECT 1` via pgx
- **cache**: `PING` via go-redis

On partial failure, the failed check returns `"error"` and `status` becomes `"degraded"`.

---

## Project Layout

```
snip/
├── cmd/snip/
│   └── main.go              # entry point: config, DB, Redis, server
├── internal/
│   ├── config/
│   │   └── config.go        # env var parsing (DB_URL, REDIS_URL, PORT, BASE_URL)
│   ├── db/
│   │   ├── db.go            # pgx pool setup, migration runner
│   │   └── queries.go       # typed query functions
│   ├── cache/
│   │   └── cache.go         # Redis client + get/set/del helpers
│   ├── handler/
│   │   ├── redirect.go      # GET /:slug
│   │   ├── dashboard.go     # GET /
│   │   ├── links.go         # POST /links, GET /links/:id, DELETE /links/:id
│   │   └── health.go        # GET /healthz
│   ├── model/
│   │   └── model.go         # Link, ClickEvent types
│   └── slug/
│       └── slug.go          # nanoid generation + validation
├── templates/
│   ├── base.html            # shared layout (nav, head, scripts)
│   ├── home.html            # dashboard home + link table
│   ├── link-detail.html     # analytics page
│   └── partials/
│       ├── link-row.html    # single table row (for HTMX swap)
│       └── link-form.html   # create link form (for HTMX swap)
├── static/
│   └── app.css              # minimal overrides (Tailwind handles most)
├── migrations/
│   ├── 001_create_links.sql
│   └── 002_create_click_events.sql
├── Dockerfile
├── docker-compose.yml       # local dev: app + postgres + redis
├── go.mod
└── go.sum
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | PostgreSQL DSN |
| `REDIS_URL` | (required) | Redis URL (`redis://...`) |
| `PORT` | `8080` | HTTP listen port |
| `BASE_URL` | (required) | Public base URL for generating short links (e.g. `https://snip.example.com`) |

---

## Error Handling

- **Slug not found**: 404 page (HTML response for browser, also used by support bundle health check)
- **Expired link**: 410 Gone with a brief message
- **Database down**: Redirect path returns 503; dashboard returns error page
- **Redis down**: Redirect path falls through to PostgreSQL (graceful degradation); health check reports `"cache": "error"`
- **Duplicate slug**: Create returns a form-level error message via HTMX

---

## Local Development

`docker-compose.yml` runs three services: `app`, `postgres`, `redis`. The app container is built from the repo Dockerfile. A `make dev` target (or `go run ./cmd/snip`) supports hot reload via `air` (optional).

---

## Out of Scope

- Authentication / multi-user support
- QR code generation
- Custom domain per-link
- Rate limiting (deferred to Replicated / ingress layer)
- Link editing (delete + recreate is sufficient)
