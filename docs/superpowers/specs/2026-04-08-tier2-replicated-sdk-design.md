# Tier 2: Replicated SDK Integration Design

**Date:** 2026-04-08
**Scope:** Rubric tasks 2.1, 2.2, 2.4, 2.5, 2.6, 2.9, 2.10

---

## Overview

Integrate the Replicated SDK into the snip Helm chart and application to unlock Vendor Portal capabilities: license entitlements, custom metrics, update notifications, license validity enforcement, and instance health reporting. All images are routed through a custom proxy domain for registry access control.

---

## 2.1 — SDK Subchart

Add the Replicated SDK as a fourth subchart in `chart/snip/Chart.yaml`:

```yaml
- name: replicated
  repository: "oci://registry.replicated.com/library"
  # version: pin to latest stable from `helm search repo replicated/replicated` at implementation time
```

In `values.yaml`, set `nameOverride` so the deployment is named `snip-sdk`:

```yaml
replicated:
  nameOverride: snip-sdk
```

Do **not** use the Helm `alias` field. The SDK is accessible in-cluster at `http://snip-sdk`.

---

## 2.2 — Image Proxy

All container images are proxied through `proxy.alexparker.info` using the Replicated proxy path format:

```
proxy.alexparker.info/proxy/snip-enterprise/<original-registry>/<org>/<image>:<tag>
```

**App image** (`deployment.yaml` / `values.yaml`):
```
proxy.alexparker.info/proxy/snip-enterprise/index.docker.io/ajpio/snip:<tag>
```

**Subchart image overrides** (in `values.yaml`):
```yaml
postgresql:
  image:
    registry: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io

redis:
  image:
    registry: proxy.alexparker.info/proxy/snip-enterprise/index.docker.io

replicated:
  image:
    registry: proxy.alexparker.info
```

> Note: Bitnami subchart image override key paths (`image.registry`) should be verified against the specific chart version during implementation — they may differ between PostgreSQL and Redis chart versions.

The vendor portal custom domain must alias `proxy.replicated.com` for this to work.

---

## 2.4 — Custom Metrics

A shared `sendMetrics(ctx, store)` helper is added to the `handler` package. It queries current DB counts and POSTs to `http://snip-sdk/api/v1/app/custom-metrics`:

```json
{
  "data": [
    { "name": "total_links",   "value": 42 },
    { "name": "total_clicks",  "value": 1337 },
    { "name": "active_links",  "value": 38 }
  ]
}
```

`sendMetrics` is called (fire-and-forget, errors logged but not surfaced to the user) after:

- `LinksHandler.Create` — on successful link creation
- `LinksHandler.Delete` — on successful link deletion
- `RedirectHandler.ServeHTTP` — after recording the click (after `h.recorder.Record`)

The SDK URL is injected via an environment variable `REPLICATED_SDK_ENDPOINT` (default: `http://snip-sdk`) to allow override in tests or non-Replicated deployments.

---

## 2.5 — Custom Slugs Entitlement

A custom license field `custom_slugs_enabled` (type: boolean, default: false) is defined in the Vendor Portal.

A `licenseEnabled(ctx, field string) bool` helper in the `handler` package calls `GET http://snip-sdk/api/v1/license/fields` and returns the boolean value for the given field name. On SDK error (network failure, non-200), it defaults to `false` (deny by default).

**Form rendering (`GET /links/new`):**

`LinksHandler.Form` checks `licenseEnabled(ctx, "custom_slugs_enabled")` and passes the result into the template context. The `link-form` template conditionally renders the custom slug input field only when `CustomSlugsEnabled: true`.

**Create handler (`POST /links`):**

`LinksHandler.Create` checks the same field before processing a submitted custom slug. If `customSlug != ""` and the entitlement is disabled, it returns a 422 with a form error: "Custom slugs require a higher license tier."

This means the check happens on every form load and every submit — no caching, no state.

---

## 2.6 — Update Banner + License Validity

Two SDK calls are made in `DashboardHandler.ServeHTTP` before rendering:

1. `GET http://snip-sdk/api/v1/app/updates` — parse response for available updates
2. `GET http://snip-sdk/api/v1/license/info` — parse response for `expiresAt` and `isExpired`

Results are passed into the dashboard template context:

```go
type DashboardData struct {
    Links          []model.LinkWithCount
    UpdateAvailable bool
    LicenseInvalid  bool
}
```

The dashboard template renders banners at the top of the page:

- **Update banner** (yellow): "A new version of Snip is available." — shown when `UpdateAvailable: true`
- **License banner** (red): "Your license has expired. Please contact support." — shown when `LicenseInvalid: true`

SDK errors on these calls are logged and treated as "no banner" — they do not fail the page load.

---

## 2.9 / 2.10 — Instance + Health Reporting

These are operational steps (no code changes):

1. Deploy the updated chart to a CMX Kubernetes cluster
2. In Vendor Portal → Instances, name the instance (e.g., `snip-dev`) and add tags
3. Verify the instance reports back as healthy and shows custom metrics on the Instance Details page
4. Confirm all workload services appear as Running in instance reporting

---

## SDK Client Pattern

All SDK HTTP calls use `net/http` with a 3-second timeout. No shared client or retry logic. The SDK endpoint is read from `os.Getenv("REPLICATED_SDK_ENDPOINT")` at call time, defaulting to `http://snip-sdk`.

When the SDK is unreachable (e.g., local development without Replicated), all entitlement checks return `false`, metrics calls are silently dropped, and banners are suppressed. This ensures the app runs cleanly outside a Replicated cluster.

---

## values.schema.json

When a chart is pulled through the Replicated registry, Helm injects `global.replicated.*` values before install. Without a `global` entry in the schema these get flagged as unknown properties. Two additions are required:

1. **`global.replicated`** — object with `channelName`, `customerEmail`, `customerName`, `dockerconfigjson`, `licenseID`, `licenseType`, `licenseFields` (all optional strings/object)
2. **`replicated`** — top-level object entry for the SDK subchart values

These are already applied to `values.schema.json` as part of the design.

---

## Files Changed

| File | Change |
|------|--------|
| `chart/snip/Chart.yaml` | Add `replicated` subchart dependency |
| `chart/snip/values.yaml` | Add `replicated.nameOverride`, proxy image registries |
| `chart/snip/values.schema.json` | Add `global.replicated` and `replicated` top-level entries (already done) |
| `chart/snip/templates/deployment.yaml` | Update app image reference to proxy domain |
| `internal/handler/sdk.go` | New file: `sendMetrics`, `licenseEnabled` helpers |
| `internal/handler/links.go` | Custom slug entitlement check in `Form` and `Create` |
| `internal/handler/redirect.go` | Call `sendMetrics` after click recording |
| `internal/handler/dashboard.go` | Update/license checks, updated `DashboardData` |
| `internal/assets/templates/` | Update banner HTML in dashboard template, conditional slug field in form template |
