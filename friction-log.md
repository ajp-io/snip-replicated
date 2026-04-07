# Friction Log

## 2026-04-07 — NULL label scanning panics on link creation without label

**What was attempted:** BYO database/cache test — deployed standalone PostgreSQL and Redis in `snip-external` namespace, then deployed snip in `snip-byo` with `postgresql.enabled=false` and `redis.enabled=false` pointing to them via cross-namespace DNS.

**What went wrong:** Creating a link without a label field returned HTTP 422 "Could not create link. The slug may already be taken." The real error was pgx failing to scan a SQL `NULL` value into a Go `string` (non-pointer). The query uses `NULLIF($3, '')` to store NULL for empty labels, but the RETURNING clause returned NULL which pgx cannot scan into `string`.

**How it was resolved:** Changed all four queries that scan the `label` column to use `COALESCE(label, '')` so NULL is mapped to empty string at the DB layer rather than requiring a nullable Go type. Fixed in `internal/db/queries.go`.

**Note:** This bug affects any link creation without a label, regardless of embedded vs BYO database. It was masked during initial development because testing always included a label value.

---

## 2026-04-07 — Template set sharing causes {{define "title"}} to bleed across pages

**What was attempted:** Loading all HTML templates into a single `*template.Template` set via `template.ParseFS(..., "templates/*.html", "templates/partials/*.html")` and sharing it across all handlers.

**What went wrong:** `link-detail.html` defines `{{define "title"}}{{.Link.Slug}} — Snip{{end}}`. Since all templates share one set, this `{{define}}` overrides the `{{block "title"}}` in `base.html` for every page — including the dashboard. When the dashboard executed the template with `DashboardData` (no `.Link` field), the render failed mid-`<title>` tag and the response was truncated to 181 bytes, causing a blank page in the browser.

**How it was resolved:** Parse separate `*template.Template` sets per page in `main.go`: `homeTmpl` (base + partials + home.html), `detailTmpl` (base + partials + link-detail.html), and `rowTmpl` (partials only). Each handler receives only the template set it needs.

---

## 2026-04-07 — Helm v4 requires extracted chart directories, not just tarballs

**What was attempted:** Running `helm dependency build` then installing the chart.

**What went wrong:** `helm dependency build` downloaded `.tgz` files into `charts/` but Helm v4.1.3 still reported "found in Chart.yaml, but missing in charts/ directory". Helm v3 accepted `.tgz` files; Helm v4 requires them to be extracted as directories.

**How it was resolved:** Manually extracted the tarballs: `tar xzf postgresql-16.6.4.tgz && tar xzf redis-20.11.3.tgz` inside `charts/`. Will need to investigate whether `helm dependency update` behavior changed in v4 or if a config option controls this.

---

## 2026-04-07 — `replicatedhq/replicated-actions/compatibility-matrix` does not exist

**What was attempted:** Added `replicatedhq/replicated-actions/compatibility-matrix@v1` step to PR and release workflows based on Replicated documentation references.

**What went wrong:** GitHub Actions failed immediately at setup with "Can't find 'action.yml', 'action.yaml' or 'Dockerfile' for action". The action does not exist in the `replicatedhq/replicated-actions` repo.

**How it was resolved:** Checked the actual repo contents (`gh api repos/replicatedhq/replicated-actions/contents`). The correct compatibility matrix pattern uses separate actions: `create-customer`, `create-cluster`, `helm-install`, `remove-cluster`, `archive-customer`. This requires the image to be proxied through Replicated (Tier 2 task 2.2). For Tier 1, replaced with `helm lint chart/snip` as a basic chart validation step. Full CMX testing can be added after image proxy is configured in Tier 2.
# scoped rbac token test Tue Apr  7 16:35:41 EDT 2026
Tue Apr  7 16:42:04 EDT 2026
Tue Apr  7 16:47:02 EDT 2026
Tue Apr  7 16:52:47 EDT 2026
Tue Apr  7 16:55:01 EDT 2026
Tue Apr  7 16:58:02 EDT 2026
