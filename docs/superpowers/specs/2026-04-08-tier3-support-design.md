# Tier 3: Support It — Design Spec

**Date:** 2026-04-08
**Scope:** Rubric tasks 3.1–3.7 — preflight checks, support bundle collectors/analyzers, and support bundle generation from the app UI.

---

## Overview

Tier 3 adds operational observability and supportability to the Snip app. It introduces:
- **Preflight checks** that validate the environment before install
- **A support bundle spec** with log collectors, health checks, and analyzers
- **A "Generate Support Bundle" button** in the app UI that collects and uploads a bundle to the Vendor Portal via the Replicated SDK

---

## 3.1 — Preflight Checks

**File:** `chart/snip/templates/preflight.yaml`

A `Secret` with label `troubleshoot.sh/kind: preflight` containing a `kind: Preflight` spec.

**Five required checks:**

| # | Check | Collector | Condition |
|---|-------|-----------|-----------|
| 1 | External DB connectivity | `tcpConnect` to `externalDatabase.host:externalDatabase.port` | Only rendered when `postgresql.enabled=false` (Helm conditional in stringData) |
| 2 | SMTP server connectivity | `tcpConnect` to `smtp.host:smtp.port` | Only rendered when `smtp.enabled=true` |
| 3 | Cluster resources | `nodeResources` | Always |
| 4 | Kubernetes version | Built-in `clusterVersion` analyzer | Always |
| 5 | Distribution check | Built-in `distribution` analyzer | Always — explicit fail on `docker-desktop` and `microk8s` |

**New Helm values added to `values.yaml`:**
```yaml
smtp:
  enabled: false
  host: ""
  port: 587
```

Check 3 requires ≥ 500m CPU and ≥ 512Mi memory available across the cluster (realistic minimums for app + postgres + redis). Failure messages name the resource shortfall and link to docs.

Distribution check names the detected unsupported distribution in its failure message.

---

## 3.2–3.6 — Support Bundle

**File:** `chart/snip/templates/support-bundle.yaml`

A second `Secret` with label `troubleshoot.sh/kind: support-bundle` in the main chart (alongside the SDK's default spec). The `support-bundle` CLI merges multiple specs discovered via `--load-cluster-specs`.

### Collectors (3.2)

| Component | Collector | Limits |
|-----------|-----------|--------|
| Snip app | `logs` with selector `app.kubernetes.io/name=snip` | `maxLines: 10000` |
| PostgreSQL | `logs` with selector `app.kubernetes.io/name=postgresql` | `maxAge: 24h` |
| Redis | `logs` with selector `app.kubernetes.io/name=redis` | `maxLines: 5000` |
| Health endpoint | `http` GET to `http://snip.<namespace>:80/healthz` | — |

### Analyzers (3.3–3.6)

| Task | Analyzer | Behavior |
|------|----------|----------|
| 3.3 | `textAnalyze` on healthz response | Pass if collected response contains `"status":"ok"`, fail otherwise with actionable message |
| 3.4 | `deploymentStatus` for snip app | Fail with pod name and operational impact message |
| 3.4 | `statefulsetStatus` for postgresql | Fail with "Database unavailable — links cannot be created or redirected" |
| 3.4 | `statefulsetStatus` for redis | Fail with "Cache unavailable — redirect performance will degrade" |
| 3.5 | `textAnalyze` on snip logs | Regex `failed to connect to.*database` — fires on DB connection errors; message explains cause and remediation |
| 3.6 | `storageClass` | Fail if no default storage class — message says PostgreSQL PVC cannot be provisioned |
| 3.6 | `nodeResources` | Fail if any node not Ready — message names the node |

---

## 3.7 — Support Bundle from App UI

### App image change (Dockerfile)

Download the `support-bundle` binary from the troubleshoot.sh GitHub releases into the final image at `/usr/local/bin/support-bundle`. This adds ~30MB to the image.

```dockerfile
ARG TROUBLESHOOT_VERSION=0.125.0
RUN wget -q -O /tmp/sb.tar.gz \
      https://github.com/replicatedhq/troubleshoot/releases/download/v${TROUBLESHOOT_VERSION}/support-bundle_linux_amd64.tar.gz \
  && tar -xzf /tmp/sb.tar.gz -C /usr/local/bin support-bundle \
  && chmod +x /usr/local/bin/support-bundle \
  && rm /tmp/sb.tar.gz
```

### RBAC

New templates in the chart:
- **`ClusterRole` `snip-support-bundle`**: `get`/`list`/`watch` on pods, pods/log, nodes, namespaces, secrets, configmaps, storageclasses
- **`ClusterRoleBinding`**: binds the app's service account to the above ClusterRole

The ClusterRole is needed because the support-bundle CLI must read logs across the namespace and inspect cluster-level resources (nodes, storage classes).

### Go handler (`internal/handler/supportbundle.go`)

`POST /support-bundle` handler:
1. Runs `support-bundle --load-cluster-specs --output /tmp/snip-bundle.tar.gz` via `exec.CommandContext` with a 120s timeout
2. Reads the generated `.tar.gz`
3. POSTs the binary to `http://<sdk-endpoint>/api/v1/app/supportbundle` with `Content-Type: application/gzip`
4. Returns JSON `{"ok": true}` or `{"error": "..."}` to the frontend

The SDK endpoint is read from the `REPLICATED_SDK_ENDPOINT` env var (already configured in the deployment).

### UI change (`internal/assets/templates/base.html`)

Add a small "Generate Support Bundle" link/button in the nav bar (right side, next to "+ New Link"). It uses HTMX to POST to `/support-bundle` and shows an inline success/error toast. The button shows a spinner while the bundle is generating (can take 30–60s).

---

## Files Changed / Created

| File | Change |
|------|--------|
| `chart/snip/templates/preflight.yaml` | **New** — 5 preflight checks |
| `chart/snip/templates/support-bundle.yaml` | **New** — full support bundle spec |
| `chart/snip/templates/rbac.yaml` | **New** — ClusterRole + ClusterRoleBinding |
| `chart/snip/values.yaml` | Add `smtp.enabled/host/port` |
| `chart/snip/values.schema.json` | Add smtp schema |
| `Dockerfile` | Add `support-bundle` binary download |
| `internal/handler/supportbundle.go` | **New** — POST /support-bundle handler |
| `cmd/snip/main.go` | Register `/support-bundle` route |
| `internal/assets/templates/base.html` | Add "Generate Support Bundle" button |
