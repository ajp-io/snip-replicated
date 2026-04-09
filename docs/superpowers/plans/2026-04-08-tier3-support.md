# Tier 3: Preflight Checks, Support Bundle & Bundle Generation UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 5 preflight checks, a full support bundle spec (collectors + analyzers), RBAC for in-pod bundle generation, a `support-bundle` binary in the app image, and a "Generate Bundle" button in the nav that collects and uploads a bundle to the Vendor Portal via the Replicated SDK.

**Architecture:** Preflight and support bundle specs are Helm-templated Secrets in `chart/snip/templates/` — labeled so troubleshoot.sh auto-discovers them. The `support-bundle` binary is downloaded in the Dockerfile builder stage and copied to the runtime image. A new Go handler shells out to `support-bundle --load-cluster-specs`, then POSTs the `.tar.gz` to the SDK's upload endpoint. The nav button triggers the handler via HTMX and displays success/error inline.

**Tech Stack:** Go, chi router, HTMX, Helm, troubleshoot.sh v1beta2 API, Replicated SDK API

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `chart/snip/values.yaml` | Modify | Add `smtp` block |
| `chart/snip/values.schema.json` | Modify | Add smtp schema |
| `chart/snip/templates/preflight.yaml` | Create | 5 preflight checks as a labeled Secret |
| `chart/snip/templates/support-bundle.yaml` | Create | Full support bundle spec: log collectors, http collector, all analyzers |
| `chart/snip/templates/rbac.yaml` | Create | ClusterRole + ClusterRoleBinding for bundle generation permissions |
| `Dockerfile` | Modify | Download `support-bundle` in builder stage, copy to runtime |
| `internal/handler/supportbundle.go` | Create | `POST /support-bundle` handler |
| `cmd/snip/main.go` | Modify | Construct `SupportBundleHandler`, register `/support-bundle` route |
| `internal/assets/templates/base.html` | Modify | Add Generate Bundle button + status span to nav |
| `chart/snip/Chart.yaml` | Modify | Bump chart version |

---

## Task 1: Add smtp values to chart

**Files:**
- Modify: `chart/snip/values.yaml`
- Modify: `chart/snip/values.schema.json`

- [ ] **Step 1: Add smtp block to values.yaml**

In `chart/snip/values.yaml`, add after the `externalRedis` block (after line 93):

```yaml
## Optional SMTP server — when enabled, preflight checks TCP connectivity
smtp:
  enabled: false
  host: ""
  port: 587
```

- [ ] **Step 2: Add smtp to values.schema.json**

In `chart/snip/values.schema.json`, inside the top-level `"properties"` object, add after the `"externalRedis"` entry:

```json
    "smtp": {
      "type": "object",
      "properties": {
        "enabled": { "type": "boolean" },
        "host": { "type": "string" },
        "port": { "type": "integer" }
      }
    },
```

- [ ] **Step 3: Lint the chart**

```bash
helm lint chart/snip
```

Expected: `0 chart(s) failed`

- [ ] **Step 4: Commit**

```bash
git add chart/snip/values.yaml chart/snip/values.schema.json
git commit -m "feat(chart): add optional smtp config for preflight check"
```

---

## Task 2: Create preflight checks

**Files:**
- Create: `chart/snip/templates/preflight.yaml`

- [ ] **Step 1: Create the preflight template**

Create `chart/snip/templates/preflight.yaml`:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "snip.fullname" . }}-preflight
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "snip.labels" . | nindent 4 }}
    troubleshoot.sh/kind: preflight
stringData:
  preflight.yaml: |-
    apiVersion: troubleshoot.sh/v1beta2
    kind: Preflight
    metadata:
      name: snip-preflight
    spec:
      collectors:
        {{- if not .Values.postgresql.enabled }}
        - tcpConnect:
            collectorName: external-database
            address: "{{ .Values.externalDatabase.host }}:{{ .Values.externalDatabase.port }}"
        {{- end }}
        {{- if .Values.smtp.enabled }}
        - tcpConnect:
            collectorName: smtp-server
            address: "{{ .Values.smtp.host }}:{{ .Values.smtp.port }}"
        {{- end }}
      analyzers:
        {{- if not .Values.postgresql.enabled }}
        - tcpConnect:
            checkName: External Database Connectivity
            collectorName: external-database
            outcomes:
              - fail:
                  when: connection-refused
                  message: "Cannot connect to the external PostgreSQL database at {{ .Values.externalDatabase.host }}:{{ .Values.externalDatabase.port }}. Verify the host, port, and firewall rules."
              - fail:
                  when: connection-timeout
                  message: "Timed out connecting to the external PostgreSQL database at {{ .Values.externalDatabase.host }}:{{ .Values.externalDatabase.port }}. Check network connectivity and firewall rules."
              - pass:
                  when: connected
                  message: "External database is reachable."
        {{- end }}
        {{- if .Values.smtp.enabled }}
        - tcpConnect:
            checkName: SMTP Server Connectivity
            collectorName: smtp-server
            outcomes:
              - fail:
                  when: connection-refused
                  message: "Cannot connect to the SMTP server at {{ .Values.smtp.host }}:{{ .Values.smtp.port }}. Verify the host and port are correct."
              - fail:
                  when: connection-timeout
                  message: "Timed out connecting to the SMTP server at {{ .Values.smtp.host }}:{{ .Values.smtp.port }}. Check firewall rules and network connectivity."
              - pass:
                  when: connected
                  message: "SMTP server is reachable."
        {{- end }}
        - nodeResources:
            checkName: Cluster Resource Requirements
            outcomes:
              - fail:
                  when: "sum(cpuCapacity) < 500m"
                  message: "Insufficient cluster CPU. Snip requires at least 500m total CPU capacity across all nodes."
              - fail:
                  when: "sum(memoryCapacity) < 512Mi"
                  message: "Insufficient cluster memory. Snip requires at least 512Mi total memory capacity across all nodes."
              - pass:
                  when: "sum(cpuCapacity) >= 500m"
                  message: "Cluster has sufficient CPU and memory."
        - clusterVersion:
            checkName: Kubernetes Version
            outcomes:
              - fail:
                  when: "< 1.25.0"
                  message: "Snip requires Kubernetes 1.25 or later. Upgrade your cluster before installing."
              - pass:
                  when: ">= 1.25.0"
                  message: "Kubernetes version is supported."
        - distribution:
            checkName: Kubernetes Distribution
            outcomes:
              - fail:
                  when: "== docker-desktop"
                  message: "Docker Desktop is not a supported distribution. See https://docs.replicated.com/vendor/testing-supported-clusters for supported options."
              - fail:
                  when: "== microk8s"
                  message: "MicroK8s is not a supported distribution. See https://docs.replicated.com/vendor/testing-supported-clusters for supported options."
              - pass:
                  when: "== eks"
                  message: "EKS is a supported distribution."
              - pass:
                  when: "== gke"
                  message: "GKE is a supported distribution."
              - pass:
                  when: "== aks"
                  message: "AKS is a supported distribution."
              - pass:
                  when: "== k3s"
                  message: "K3s is a supported distribution."
              - pass:
                  when: "== rke2"
                  message: "RKE2 is a supported distribution."
              - pass:
                  when: "== unknown"
                  message: "Distribution is unknown but not explicitly unsupported."
```

- [ ] **Step 2: Verify preflight Secret renders**

```bash
helm template snip chart/snip | grep -A5 "troubleshoot.sh/kind: preflight"
```

Expected: Shows the Secret with preflight label and the fixed analyzers (no conditional collectors since defaults have postgresql.enabled=true).

- [ ] **Step 3: Verify conditional rendering with external DB and SMTP**

```bash
helm template snip chart/snip \
  --set postgresql.enabled=false \
  --set externalDatabase.host=db.example.com \
  --set externalDatabase.user=snip \
  --set externalDatabase.name=snip \
  --set smtp.enabled=true \
  --set smtp.host=smtp.example.com | grep -A3 "tcpConnect"
```

Expected: Shows both `collectorName: external-database` and `collectorName: smtp-server` entries.

- [ ] **Step 4: Lint**

```bash
helm lint chart/snip
```

Expected: `0 chart(s) failed`

- [ ] **Step 5: Commit**

```bash
git add chart/snip/templates/preflight.yaml
git commit -m "feat(chart): add 5 preflight checks (db, smtp, resources, k8s version, distribution)"
```

---

## Task 3: Create support bundle spec

**Files:**
- Create: `chart/snip/templates/support-bundle.yaml`

- [ ] **Step 1: Create the support bundle template**

Create `chart/snip/templates/support-bundle.yaml`:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "snip.fullname" . }}-support-bundle
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "snip.labels" . | nindent 4 }}
    troubleshoot.sh/kind: support-bundle
stringData:
  support-bundle-spec: |-
    apiVersion: troubleshoot.sh/v1beta2
    kind: SupportBundle
    metadata:
      name: snip-support-bundle
    spec:
      collectors:
        - logs:
            collectorName: snip-app-logs
            selector:
              - app.kubernetes.io/name=snip
              - app.kubernetes.io/instance={{ .Release.Name }}
            namespace: {{ .Release.Namespace | quote }}
            name: snip/logs
            maxLines: 10000
        {{- if .Values.postgresql.enabled }}
        - logs:
            collectorName: postgresql-logs
            selector:
              - app.kubernetes.io/name=postgresql
              - app.kubernetes.io/instance={{ .Release.Name }}
            namespace: {{ .Release.Namespace | quote }}
            name: postgresql/logs
            maxAge: 24h
        {{- end }}
        {{- if .Values.redis.enabled }}
        - logs:
            collectorName: redis-logs
            selector:
              - app.kubernetes.io/name=redis
              - app.kubernetes.io/instance={{ .Release.Name }}
            namespace: {{ .Release.Namespace | quote }}
            name: redis/logs
            maxLines: 5000
        {{- end }}
        - http:
            collectorName: healthz-check
            get:
              url: "http://{{ include "snip.fullname" . }}.{{ .Release.Namespace }}/healthz"
              timeout: 5s
      analyzers:
        - textAnalyze:
            checkName: Application Health Endpoint
            fileName: "healthz-check.json"
            regex: '"status":"ok"'
            outcomes:
              - fail:
                  when: "false"
                  message: "Application health endpoint does not report ok status. Check database and cache connectivity — run kubectl logs on the snip pod for details."
              - pass:
                  when: "true"
                  message: "Application is healthy."
        - deploymentStatus:
            checkName: Snip Deployment
            name: {{ include "snip.fullname" . }}
            namespace: {{ .Release.Namespace | quote }}
            outcomes:
              - fail:
                  when: "< 1"
                  message: "Snip deployment has no available replicas — the URL shortener is down and all requests will fail. Check pod events with kubectl describe pod."
              - pass:
                  when: ">= 1"
                  message: "Snip deployment is running."
        {{- if .Values.postgresql.enabled }}
        - statefulsetStatus:
            checkName: PostgreSQL StatefulSet
            name: {{ include "snip.fullname" . }}-postgresql
            namespace: {{ .Release.Namespace | quote }}
            outcomes:
              - fail:
                  when: "< 1"
                  message: "PostgreSQL is not running. The database is unavailable — links cannot be created or followed. Check pod logs with kubectl logs."
              - pass:
                  when: ">= 1"
                  message: "PostgreSQL is running."
        {{- end }}
        {{- if .Values.redis.enabled }}
        - statefulsetStatus:
            checkName: Redis StatefulSet
            name: {{ include "snip.fullname" . }}-redis-master
            namespace: {{ .Release.Namespace | quote }}
            outcomes:
              - fail:
                  when: "< 1"
                  message: "Redis is not running. The cache is unavailable — redirect latency will increase significantly."
              - pass:
                  when: ">= 1"
                  message: "Redis is running."
        {{- end }}
        - textAnalyze:
            checkName: Database Connection Errors in App Logs
            fileName: "snip/logs/{{ .Release.Namespace }}/*/*.log"
            regex: "failed to connect to"
            outcomes:
              - fail:
                  when: "true"
                  message: "Database connection errors detected in Snip app logs. Verify PostgreSQL is running and DATABASE_URL is correct. Check pod events with kubectl describe pod."
              - pass:
                  when: "false"
                  message: "No database connection errors found in Snip app logs."
        - storageClass:
            checkName: Default Storage Class
            outcomes:
              - fail:
                  when: "not-found"
                  message: "No default storage class found. PostgreSQL requires a default storage class to provision its persistent volume. Contact your cluster administrator to set a default storage class."
              - pass:
                  when: "available"
                  message: "Default storage class is available."
        - nodeResources:
            checkName: Node Readiness
            filters:
              nodeCondition: Ready
            outcomes:
              - fail:
                  when: "count() == 0"
                  message: "No Ready nodes found. Pod scheduling will fail — check node status with kubectl get nodes."
              - pass:
                  when: "count() > 0"
                  message: "Nodes are Ready."
```

- [ ] **Step 2: Verify two support-bundle Secrets render**

```bash
helm template snip chart/snip | grep "troubleshoot.sh/kind: support-bundle" | wc -l
```

Expected: `2` (one from the replicated subchart, one from this template).

- [ ] **Step 3: Verify conditional statefulset analyzers are omitted when subcharts disabled**

```bash
helm template snip chart/snip \
  --set postgresql.enabled=false \
  --set redis.enabled=false \
  --set externalDatabase.host=db.example.com \
  --set externalDatabase.user=snip \
  --set externalDatabase.name=snip | grep "statefulsetStatus" | wc -l
```

Expected: `0`

- [ ] **Step 4: Lint**

```bash
helm lint chart/snip
```

Expected: `0 chart(s) failed`

- [ ] **Step 5: Commit**

```bash
git add chart/snip/templates/support-bundle.yaml
git commit -m "feat(chart): add support bundle spec with collectors, health check, and analyzers"
```

---

## Task 4: Create RBAC for support bundle generation

**Files:**
- Create: `chart/snip/templates/rbac.yaml`

The `support-bundle` binary runs in the app pod using the pod's service account token. It needs cluster-level read access to collect pod logs, node info, and storage class details.

- [ ] **Step 1: Create rbac.yaml**

Create `chart/snip/templates/rbac.yaml`:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "snip.fullname" . }}-support-bundle
  labels:
    {{- include "snip.labels" . | nindent 4 }}
rules:
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - configmaps
      - events
      - namespaces
      - nodes
      - persistentvolumes
      - persistentvolumeclaims
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources:
      - secrets
    verbs: ["get", "list"]
  - apiGroups: ["apps"]
    resources:
      - deployments
      - statefulsets
      - replicasets
      - daemonsets
    verbs: ["get", "list", "watch"]
  - apiGroups: ["storage.k8s.io"]
    resources:
      - storageclasses
    verbs: ["get", "list"]
  - apiGroups: ["batch"]
    resources:
      - jobs
      - cronjobs
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "snip.fullname" . }}-support-bundle
  labels:
    {{- include "snip.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "snip.fullname" . }}-support-bundle
subjects:
  - kind: ServiceAccount
    name: {{ include "snip.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
```

- [ ] **Step 2: Verify ClusterRole and ClusterRoleBinding render**

```bash
helm template snip chart/snip | grep "kind: ClusterRole"
```

Expected: Two lines — one `ClusterRole`, one `ClusterRoleBinding`.

- [ ] **Step 3: Lint**

```bash
helm lint chart/snip
```

Expected: `0 chart(s) failed`

- [ ] **Step 4: Commit**

```bash
git add chart/snip/templates/rbac.yaml
git commit -m "feat(chart): add ClusterRole + ClusterRoleBinding for support bundle generation"
```

---

## Task 5: Add support-bundle binary to Dockerfile

**Files:**
- Modify: `Dockerfile`

Download in the builder stage (which already has wget via Alpine) and copy the static binary to the runtime stage. No extra tools are added to the production image.

- [ ] **Step 1: Check the latest troubleshoot release tag**

```bash
curl -s https://api.github.com/repos/replicatedhq/troubleshoot/releases/latest | grep '"tag_name"'
```

Note the version (e.g. `v0.120.1`). Strip the `v` prefix for use in the URL. Use this version in the next step.

- [ ] **Step 2: Update the Dockerfile**

Replace the current `Dockerfile` content with (substitute the actual version from Step 1 for `TROUBLESHOOT_VERSION`):

```dockerfile
# Build stage
FROM --platform=linux/amd64 golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /snip ./cmd/snip

# Download support-bundle binary for in-app bundle generation (task 3.7)
ARG TROUBLESHOOT_VERSION=0.120.1
RUN wget -q -O /tmp/sb.tar.gz \
      "https://github.com/replicatedhq/troubleshoot/releases/download/v${TROUBLESHOOT_VERSION}/support-bundle_linux_amd64.tar.gz" \
  && tar -xzf /tmp/sb.tar.gz -C /usr/local/bin support-bundle \
  && chmod +x /usr/local/bin/support-bundle \
  && rm /tmp/sb.tar.gz

# Runtime stage
FROM --platform=linux/amd64 alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /snip /app/snip
COPY --from=builder /usr/local/bin/support-bundle /usr/local/bin/support-bundle
EXPOSE 8080
ENTRYPOINT ["/app/snip"]
```

- [ ] **Step 3: Build locally to verify**

```bash
docker build -t snip:tier3-test .
```

Expected: Build succeeds. If the download fails (wrong version), adjust `TROUBLESHOOT_VERSION` to the tag from Step 1.

- [ ] **Step 4: Confirm the binary runs**

```bash
docker run --rm snip:tier3-test support-bundle version
```

Expected: Prints `support-bundle version v<x.y.z>` (or similar version output).

- [ ] **Step 5: Commit**

```bash
git add Dockerfile
git commit -m "feat(docker): add support-bundle binary for in-app bundle generation"
```

---

## Task 6: Create the support bundle Go handler

**Files:**
- Create: `internal/handler/supportbundle.go`

The handler always returns 200 with an HTML fragment — HTMX requires a 2xx to swap the response into the target element.

- [ ] **Step 1: Create the handler**

Create `internal/handler/supportbundle.go`:

```go
package handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SupportBundleHandler handles POST /support-bundle.
type SupportBundleHandler struct {
	sdkEndpoint string
}

func NewSupportBundleHandler(sdkEndpoint string) *SupportBundleHandler {
	return &SupportBundleHandler{sdkEndpoint: sdkEndpoint}
}

func (h *SupportBundleHandler) Generate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	bundlePath := filepath.Join(os.TempDir(), fmt.Sprintf("snip-bundle-%d.tar.gz", time.Now().UnixNano()))
	defer os.Remove(bundlePath)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "support-bundle",
		"--load-cluster-specs",
		"--output", bundlePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("support-bundle generation failed: %v\n%s", err, out)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Bundle generation failed: %v</span>`, err)
		return
	}

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		log.Printf("reading bundle file: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to read bundle: %v</span>`, err)
		return
	}

	uploadURL := h.sdkEndpoint + "/api/v1/app/supportbundle"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		log.Printf("building upload request: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to build upload request: %v</span>`, err)
		return
	}
	req.Header.Set("Content-Type", "application/gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("uploading bundle to SDK: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Upload failed: %v</span>`, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("SDK returned HTTP %d on bundle upload", resp.StatusCode)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">SDK rejected bundle (HTTP %d)</span>`, resp.StatusCode)
		return
	}

	fmt.Fprint(w, `<span class="text-green-400 text-sm">Bundle uploaded — check Vendor Portal</span>`)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add internal/handler/supportbundle.go
git commit -m "feat(handler): add POST /support-bundle handler"
```

---

## Task 7: Wire route and add UI button

**Files:**
- Modify: `cmd/snip/main.go`
- Modify: `internal/assets/templates/base.html`

- [ ] **Step 1: Add handler construction in main.go**

In `cmd/snip/main.go`, after line 73 (after `redirectH := ...`), add:

```go
	supportBundleH := handler.NewSupportBundleHandler(cfg.SDKEndpoint)
```

- [ ] **Step 2: Register the route in main.go**

After `r.Delete("/links/{id}", linksH.Delete)` (before the slug redirect comment), add:

```go
	r.Post("/support-bundle", supportBundleH.Generate)
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 4: Update the nav in base.html**

In `internal/assets/templates/base.html`, replace the `<nav>...</nav>` block with:

```html
  <nav class="border-b border-gray-800 px-6 py-4 flex items-center justify-between">
    <a href="/" class="text-indigo-400 font-bold text-xl tracking-tight">✂ Snip</a>
    <div class="flex items-center gap-3">
      <span id="bundle-msg" class="text-sm"></span>
      <button
        hx-post="/support-bundle"
        hx-target="#bundle-msg"
        hx-swap="innerHTML"
        hx-indicator="#bundle-spinner"
        class="text-gray-400 hover:text-gray-200 text-sm px-3 py-2 rounded-md border border-gray-700 hover:border-gray-500 transition-colors">
        <span id="bundle-spinner" class="htmx-indicator">⏳ </span>Generate Bundle
      </button>
      <button
        hx-get="/links/new"
        hx-target="#link-form-container"
        hx-swap="innerHTML"
        class="bg-indigo-600 hover:bg-indigo-700 text-white text-sm px-4 py-2 rounded-md">
        + New Link
      </button>
    </div>
  </nav>
```

- [ ] **Step 5: Add htmx-indicator CSS to base.html**

In `internal/assets/templates/base.html`, inside `<head>`, add after the existing `<link rel="stylesheet" ...>` line:

```html
  <style>.htmx-indicator{display:none}.htmx-request .htmx-indicator{display:inline}</style>
```

- [ ] **Step 6: Verify it compiles**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add cmd/snip/main.go internal/assets/templates/base.html
git commit -m "feat: wire /support-bundle route and add Generate Bundle button to nav"
```

---

## Task 8: Bump chart version and final lint

**Files:**
- Modify: `chart/snip/Chart.yaml`

- [ ] **Step 1: Bump chart version**

In `chart/snip/Chart.yaml`, change `version: 0.1.11` to `version: 0.1.12`.

- [ ] **Step 2: Final lint and template check**

```bash
helm lint chart/snip
helm template snip chart/snip > /dev/null && echo "template OK"
```

Expected: Both succeed.

- [ ] **Step 3: Commit**

```bash
git add chart/snip/Chart.yaml
git commit -m "chore(chart): bump to 0.1.12 for tier 3 support features"
```
