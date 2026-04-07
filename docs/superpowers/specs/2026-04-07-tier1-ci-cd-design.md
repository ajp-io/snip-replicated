# Tier 1 CI/CD Design — Snip Replicated Bootcamp

**Date:** 2026-04-07  
**Scope:** Rubric tasks 1.1, 1.2, 1.3, 1.4

---

## Overview

Set up automated CI/CD using GitHub Actions to build and push the `snip` container image to DockerHub, create Replicated releases on PRs and merges, and configure notifications for Stable channel promotions.

---

## Context

- **App:** `snip` — Go URL shortener, Helm chart at `chart/snip/`
- **GitHub repo:** `ajp-io/snips-replicated` → rename to `ajp-io/snip`
- **DockerHub:** `ajpio/snip` (private repo)
- **Replicated app slug:** `snip-enterprise`
- **Registry pattern:** image tagged with Git SHA on PRs; also tagged `latest` on main

---

## GitHub Secrets Required

| Secret | Value |
|--------|-------|
| `DOCKERHUB_USERNAME` | `ajpio` |
| `DOCKERHUB_TOKEN` | DockerHub personal access token |
| `REPLICATED_API_TOKEN` | Replicated Vendor Portal API token |

---

## Files to Create

### `.replicated` (repo root)

Describes app layout for Replicated GitHub Actions. Specifies:
- App slug: `snip-enterprise`
- Chart path: `chart/snip`

### `.github/workflows/pr.yml` (task 1.2)

Triggered on: `pull_request` to `main`

Steps:
1. Checkout code
2. Log in to DockerHub
3. Build and push image tagged `ajpio/snip:<sha>`
4. Create a Replicated release using `replicatedhq/replicated-actions/create-release`
5. Run compatibility test using `replicatedhq/replicated-actions/compatibility-matrix`

### `.github/workflows/release.yml` (task 1.3)

Triggered on: `push` to `main`

Steps:
1. Checkout code
2. Log in to DockerHub
3. Build and push image tagged `ajpio/snip:<sha>` and `ajpio/snip:latest`
4. Create a Replicated release
5. Run compatibility test
6. Promote release to `Unstable` channel using `replicatedhq/replicated-actions/promote-release`

---

## Replicated RBAC Policy (task 1.2)

Create a custom policy in Vendor Portal named `ci-policy` scoped to only what CI needs:
- `release/create`
- `release/promote`
- `release/list`
- No customer, billing, team, or registry management permissions

Assign this policy to a dedicated service account. The `REPLICATED_API_TOKEN` secret uses this service account's token.

---

## Notifications (task 1.4)

Configured in Vendor Portal → Notifications (not in code):
- Trigger: release promoted to `Stable` channel
- Recipient: `@replicated.com` email address
- No workflow changes required

---

## Image Tagging Strategy

| Event | Tags pushed |
|-------|-------------|
| PR | `ajpio/snip:<git-sha>` |
| Merge to main | `ajpio/snip:<git-sha>`, `ajpio/snip:latest` |

The chart `values.yaml` `image.tag` is updated at release time via Replicated's release creation (or overridden in the `.replicated` file). The SHA-tagged image is what gets packaged into the release.

---

## chart/snip/values.yaml Change Required

`image.repository` is currently `ttl.sh/snip` (ephemeral public registry). Must be updated to `ajpio/snip` before CI workflows are meaningful. The SHA tag is passed to Replicated's `create-release` action as a helm value override so each release references the exact built image.

---

## Out of Scope

- No semantic versioning or changelog automation
- No deployment step (Replicated handles that)
- Notification email config is manual (Vendor Portal UI)
