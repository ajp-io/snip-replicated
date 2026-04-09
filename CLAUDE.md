# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Replicated Bootcamp project — a structured curriculum for building, packaging, and distributing an enterprise application using the [Replicated](https://replicated.com) platform. The `rubric.md` defines acceptance criteria across 7 tiers.

## What Gets Built

The project requires authoring from scratch:

- **Application**: A custom web app with backend + persistent storage (not an off-the-shelf image)
- **Helm chart**: Authored by hand with subcharts, `values.schema.json`, conditional BYO vs. embedded stateful component
- **Replicated SDK**: Embedded as a subchart, renamed for branding (`<app>-sdk`)
- **KOTS config screen**: At least 3 meaningful capabilities wired to Helm values
- **Support bundle spec**: Collectors, analyzers, health endpoint integration
- **Preflight checks**: 5 required checks (external DB, endpoint, CPU/mem, K8s version, distro)
- **CI/CD**: GitHub Actions using Replicated's actions for PR and release workflows

## Tier Progression

| Tier | Focus |
|------|-------|
| 0 | Build the app, Helm chart, subcharts, TLS, health endpoint, K8s best practices |
| 1 | CI/CD, private registry, Replicated RBAC, PR + release workflows |
| 2 | Replicated SDK, image proxy via custom domain, metrics, license entitlements, ingress |
| 3 | Preflight checks, support bundles, log collectors, health analyzers |
| 4 | Embedded Cluster v3 on a VM, air-gapped install, upgrade |
| 5 | KOTS config screen, generated passwords, validation, external DB toggle |
| 6 | Enterprise Portal v2 branding, custom email, security center, Helm/Terraform docs |
| 7 | Notifications, image signing, network policy air-gap validation |

## Key Constraints from Rubric

- Helm chart must be authored (not a fork/upstream chart used as-is)
- At least 2 open-source subcharts; stateful component subchart is default-on with BYO as opt-in
- Replicated SDK subchart must be renamed: deployment must be named `<your-app>-sdk`
- All container images must be proxied through a custom domain aliasing `proxy.replicated.com`
- License entitlement checks must query the SDK at runtime — not passed through Helm values or env vars
- Health endpoint must be reachable in-cluster (used by support bundle `http` collector)
- Generated default values (e.g., DB password) must survive upgrades without changing
- Embedded Cluster tasks use Embedded Cluster v3

## Friction Log

Maintain a `friction-log.md` file at the root of this repo. Any time a problem, blocker, confusing behavior, or unexpected difficulty is encountered — whether with the app, Replicated platform, tooling, or rubric requirements — add an entry. Each entry should include what was attempted, what went wrong, and how it was resolved (or that it remains unresolved).

## Replicated Platform Notes

- Use CMX for VM targets and cluster targets
- `.replicated` file describes app layout for GitHub Actions integration
- Custom RBAC policy should be scoped for the CI service account token
- Enterprise Portal tasks use Enterprise Portal v2
- Support bundle uploads use the Replicated SDK (not manual upload)

## Install Process (Helm via Replicated)

**Always install through Replicated — never raw `helm install` from local chart.**

The full process:

1. **Create a release** — promote the chart to the target channel (e.g. Dev) in Vendor Portal, or via CLI:
   ```bash
   replicated release create --promote Dev --chart chart/snip --version <version>
   ```

2. **Create a CMX cluster** (if none exists):
   ```bash
   replicated cluster create --name <name> --distribution eks --version 1.32 --node-count 3
   replicated cluster kubeconfig <cluster-id>   # sets kubectl context
   ```

3. **Login to Replicated registry** with the license ID as both username and password:
   ```bash
   helm registry login registry.replicated.com \
     --username <license-id> \
     --password <license-id>
   ```

4. **Install** from the Replicated OCI registry:
   ```bash
   helm install snip oci://registry.replicated.com/snip-enterprise/dev/snip \
     --version <chart-version> \
     --namespace snip --create-namespace \
     --set baseURL=https://<cluster-host>
   ```

   The license ID is automatically injected as `global.replicated.*` values by the registry at pull time.

5. **Verify**:
   ```bash
   kubectl get pods -n snip
   kubectl get deployment snip-sdk -n snip
   kubectl get pods -A -o custom-columns='STATUS:.status.phase,IMAGE:.spec.containers[*].image'
   ```

**Test customer license ID**: stored in `.env.local` as `REPLICATED_LICENSE_ID`
**Test channel**: Dev

## Development Workflow (Code Changes → Live Cluster)

When making changes to the app or chart, follow this exact sequence every time:

### 1. Bump versions
- **App image tag** in `chart/snip/values.yaml` → `image.tag`
- **Chart version** in `chart/snip/Chart.yaml` → `version`
- Both must be bumped together; skipping either causes stale deploys.

### 2. Build and push the image
```bash
docker build -t ajpio/snip:<new-tag> .
source .env.local
echo "$DOCKERHUB_TOKEN" | docker login -u ajpio --password-stdin
docker push ajpio/snip:<new-tag>
```

### 3. Package and release the chart
```bash
cd chart && helm package snip   # produces snip-<version>.tgz
REPLICATED_API_TOKEN=<token> replicated release create \
  --app snip-enterprise \
  --promote Dev \
  --chart snip-<version>.tgz \
  --version <chart-version>
```

### 4. Upgrade the cluster
**Never use `--reuse-values` alone** — it swallows new chart defaults (including updated image tags). Always pass changed values explicitly:
```bash
helm upgrade snip oci://registry.replicated.com/snip-enterprise/dev/snip \
  --version <chart-version> \
  --namespace snip \
  --reuse-values \
  --set image.tag=<new-tag>
```

### 5. Verify the rollout
```bash
kubectl rollout status deployment/snip -n snip
kubectl get pods -n snip -o custom-columns='NAME:.metadata.name,IMAGE:.spec.containers[*].image'
# Confirm the snip pod shows the new image tag before testing.
```

### 6. Restart port-forward after pod replacement
The port-forward dies when the pod is replaced. After any `helm upgrade`:
```bash
pkill -f "port-forward svc/snip" 2>/dev/null; true
kubectl port-forward svc/snip 8080:80 -n snip > /tmp/port-forward.log 2>&1 &
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz  # expect 200
```

### "A new version is available" SDK banner
The Replicated SDK shows this when the channel has a release sequence newer than what the SDK believes is installed. It appears after every new release, even if you just upgraded to it — the SDK polls on a delay. It is **not** an indication that the upgrade failed; verify by checking the running pod image instead.
