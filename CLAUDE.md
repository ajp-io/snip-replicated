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
