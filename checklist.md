# Rubric Progress Checklist

## Tier 0: Build It

- [x] **0.1** — Build custom web app with stateful component (Snip: URL shortener, Go backend, PostgreSQL, Redis)
- [x] **0.2** — Helm chart packages and deploys the application (`helm lint` passes, `values.schema.json` included)
- [x] **0.3** — 2 open-source Helm subcharts (PostgreSQL + Redis); embedded default, BYO opt-in with conditional
- [x] **0.4** — K8s best practices: liveness/readiness probes, resource requests/limits, `/healthz` endpoint, data persists across pod restart
- [ ] **0.5** — HTTPS: auto-provisioned cert, manually uploaded cert, self-signed cert options
- [x] **0.6** — App waits for database before starting (init container waits for PostgreSQL on port 5432)
- [x] **0.7** — At least 2 user-facing demoable features (link creation + analytics dashboard — built in 0.1)

## Tier 1: Automate It

- [ ] **1.1** — Images built and pushed to private registry in CI (no manual builds)
- [ ] **1.2** — Scoped Replicated RBAC policy for CI service account token
- [ ] **1.2b** — PR workflow: `.replicated` file, Replicated GitHub Actions, passing run on PR
- [ ] **1.3** — Release workflow: creates release, tests, promotes to Unstable on merge to main
- [ ] **1.4** — Email notifications on promotion to Stable channel

## Tier 2: Ship It with Helm

- [ ] **2.1** — Replicated SDK subchart, renamed to `snip-sdk` (`kubectl get deployment snip-sdk` succeeds)
- [ ] **2.2** — All images proxied through custom domain (including subchart images)
- [ ] **2.4** — App sends custom metrics to Vendor Portal (real activity, not synthetic)
- [ ] **2.5** — License entitlement gates a feature via SDK at runtime (not via Helm values/env vars)
- [ ] **2.6a** — Update available banner shown in app when update exists
- [ ] **2.6b** — License validity enforced via SDK (warning/block on expired/invalid)
- [x] **2.7** — Optional ingress, off by default, routes to app when enabled
- [x] **2.8** — Service type configurable
- [ ] **2.9/2.10** — Instance live, named, tagged, healthy, custom metrics showing

## Tier 3: Support It

- [ ] **3.1** — Preflight checks (5 required): external DB connectivity, required endpoint, CPU/mem, K8s version, distro block
- [ ] **3.2** — Support bundle: separate log collector per component with `maxLines`/`maxAge`
- [ ] **3.3** — `http` collector hits `/healthz` in-cluster; `textAnalyze` produces pass/fail
- [ ] **3.4** — Status analyzers for all workload types (`deploymentStatus`, `statefulsetStatus`, etc.)
- [ ] **3.5** — `textAnalyze` catches a known app-specific failure pattern in logs
- [ ] **3.6** — `storageClass` analyzer (fails without default SC) + `nodeResources` analyzer (fails if node not Ready)
- [ ] **3.7** — "Generate Support Bundle" in app UI, uploads to Vendor Portal via SDK

## Tier 4: Ship It on a VM

- [ ] **4.1** — App installs on bare VM via Embedded Cluster v3, all pods Running, app in browser
- [ ] **4.2** — In-place upgrade without data loss
- [ ] **4.3** — Air-gapped install from bundle, no internet, all pods Running
- [ ] **4.6** — App icon and name set in installer
- [ ] **4.7** — License entitlement gates config screen item (KOTS `LicenseFieldValue` template in `helmchart.yaml`)

## Config Screen (Tier 4/5)

- [ ] **5.0** — External/embedded DB toggle with conditional fields (host, port, credentials)
- [ ] **5.1** — At least 2 configurable features wired through config screen
- [ ] **5.2** — Generated default DB password survives upgrade
- [ ] **5.3** — Input validation with regex on at least one config item
- [ ] **5.4** — `help_text` on every config item (describes field + valid values)

## Tier 5: Deliver It (Enterprise Portal v2)

- [ ] **6.1** — Branding: custom logo, favicon, title, colors
- [ ] **6.2** — Custom email sender from own domain
- [ ] **6.3** — Security center: view CVEs, use securebuild image if needed
- [ ] **6.4** — GitHub app integration, custom left nav + main content
- [ ] **6.5** — Helm chart reference in `toc.yaml`, at least 1 undocumented field
- [ ] **6.6** — Terraform modules in EPv2 docs, toggled by license field
- [ ] **6.8** — Self-serve sign-up flow, customer appears in Vendor Portal
- [ ] **6.9** — End-to-end install via Enterprise Portal (Helm + Embedded Cluster paths)
- [ ] **6.10** — Upgrade instructions verified without downtime (Helm + EC)

## Tier 6: Operationalize It

- [ ] **7.1** — Notifications: email + webhook on account activity
- [ ] **7.2** — Explain security posture (CVEs, how to reduce)
- [ ] **7.3** — Sign images
- [ ] **7.4** — Air-gapped network policy report showing 0 outbound requests
