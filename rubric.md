# Bootcamp Rubric

## Get Started

You'll need to create a new vendor portal account, go into admin, give yourself $1000 CMX credits, remove your trial expiration. Turn on the necessary entitlements and features. Add `superci-replicated` as your GitHub Collab Repo for Support Request. You'll need to use CMX as your VM target & cluster target.

---

## Tier 0: Build It

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 0.1 | Build a custom, sufficiently complex web application with a stateful component | Show the app running locally; show a custom Dockerfile that builds the image | Must be a real application with a backend and persistent storage, not a static site or off-the-shelf image used as-is |
| 0.2 | Helm chart packages and deploys the application | Open the app in a browser; `helm lint` returns no errors; show `values.schema.json` | Chart must be authored by you, not just an upstream chart used as-is |
| 0.3 | Include at least 2 open-source Helm subcharts (not a fork), 1 should provide the stateful component; embedded as default and BYO as opt-in | Show the subchart declared with a proper conditional. Install with BYO (show no stateful component pod, app is using the external instance). Set the Helm value to enable the embedded component and show the stateful component pod Running. | |
| 0.4 | Kubernetes best practices | Show liveness and readiness probes defined in the chart; show resource requests and limits set on all containers; delete the app pod and show data is still present when it comes back; show a dedicated HTTP health endpoint (e.g. `/health` or `/healthz`) returning a structured response | The health endpoint must be reachable in-cluster — it will be checked by the support bundle in task 3.2b. Probes may hit this endpoint, but having the endpoint is required regardless of how probes are configured. |
| 0.5 | App served over HTTPS with options for 1) an automatically provisioned certificate, 2) manually generated or uploaded, 3) generated self-signed certificates (optional) | Open the app at `https://<your-domain>` and show a valid TLS certificate in the browser | |
| 0.6 | App waits for the database before starting | Ensure no pods crashloop on startup (app logic or init container in the app pod spec that checks the database is reachable before the main container starts) | Without this, the app crash-loops on first install whenever the database takes longer to start than the app |
| 0.7 | At least 2 user-facing, demoable features | Demo each feature end to end in the app | Must be real features a production app would have, not cosmetic changes |

---

## Tier 1: Automate It

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 1.1 | Container images built and pushed to a private registry in CI | Show CI run log with image build and push steps; show the image in your private registry | Create your own private registry (e.g. GCP Artifact Registry, ECR, or similar). No manual image builds. |
| 1.2 | Scoped Replicated RBAC policy — Create a custom reduced-scope RBAC policy that will limit the capabilities of your CI Service Account token | Custom policy, assigned to the Service Account | Make sure you have this feature on |
| 1.2 | PR workflow: use a `.replicated` file to describe your app layout, create a release, and test it; implemented using Replicated's GitHub Actions (or the CLI) | Show a passing Actions run triggered by a PR | |
| 1.3 | Release workflow: creates a release, tests it, and promotes to Unstable on merge to main; implemented using Replicated's GitHub Actions | Show a passing Actions run triggered by a merge to main | |
| 1.4 | Notifications — enable email notifications when new releases are promoted to the Stable channel | Only when promoted to Stable, you should receive an email to your `@replicated.com` address | |

---

## Tier 2: Ship It with Helm

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 2.1 | Replicated SDK deployed as a subchart and renamed for branding | `kubectl get deployment <app>-sdk -n <namespace>` succeeds and shows the pod Running | The deployment must be named `<your-app>-sdk` |
| 2.2 | All container images proxied through your custom domain | Run `kubectl get pods -A -o custom-columns='STATUS:.status.phase,IMAGE:.spec.containers[*].image'` and show every app image starts with your custom proxy domain and is running | Configure a custom domain in the Vendor Portal to alias `proxy.replicated.com`. Use a domain you own or expense one. Subchart images are the easy thing to miss. |
| 2.4 | App sends custom metrics visible in Vendor Portal | Show at least one meaningful app metric appearing on the Instance Details page in the Vendor Portal | Must be meaningful app metrics from real activity, not synthetic test data |
| 2.5 | License entitlement gates a real product feature | Define a custom license field in Vendor Portal. Application code reads the field value from the SDK during runtime and conditionally enables or disables the feature. Install with entitlement disabled, show the feature is unavailable. Update the license to enable it, show the feature becomes available without redeploying. | Application code must query the SDK directly; do not pass the field through Helm values or environment variables |
| 2.6 | Update available banner | When an update is available, check with the license and show a banner in the app | |
| 2.6 | License validity enforced via SDK | Application code checks license expiry and validity via the SDK. Show the app surfacing a clear warning or blocking access when the license is expired or invalid. Show normal operation with a valid license. | The app must actively check and respond to license state; a passive display of license data is not sufficient |
| 2.7 | Optional ingress | Make an Ingress resource optional & off by default in your chart. When enabled, route traffic to your app. App must be accessible via the ingress. | |
| 2.8 | Service type is configurable | | |
| 2.9 | Instance is live, name it, tag it, show it some love | Instance is live in a K8s cluster, it reports back as healthy, shows your custom metrics | |
| 2.10 | Make sure your services show up as healthy in the instance reporting | | |

---

## Tier 3: Support It

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 3.1 | Preflight checks covering required deployment concerns with clear, actionable pass/warn/fail messages | Show preflights running twice: once where all checks fail (or show a few scenarios), then again with all checks passing. Failure messages must explain what went wrong and how to fix it. All five checks are required: (1) external database connectivity — only runs when external DB is configured; fails with actionable message if unreachable; (2) required external endpoint connectivity (e.g. auth provider or SMTP), fails with actionable message if unreachable; (3) cluster resource check — validates sufficient CPU and memory; (4) Kubernetes version check — fails if below minimum supported version; (5) distribution check — explicitly fails on unsupported distributions (at minimum docker-desktop and microk8s) with a message naming the unsupported distribution and linking to supported options | |
| 3.2 | Log collection covers all app components | Show the support bundle spec has a separate logs collector for each major component (app, stateful service, any operator). Each collector sets `maxLines` or `maxAge` limits. Run the bundle and show each component's log directory is present and non-empty in the output. | |
| 3.3 | Health endpoint checked with `http` collector and `textAnalyze` | The bundle spec includes an `http` collector calling the app's health or readiness endpoint via the in-cluster service DNS name. A `textAnalyze` analyzer parses the collected response file and produces a pass/fail result based on the expected status string. Show a passing result with the app healthy, and a failing result with the app down or unhealthy. | |
| 3.4 | Status analyzers for all workload types | The bundle spec includes a status analyzer for each major workload, using whichever types apply: `deploymentStatus`, `statefulsetStatus`, `jobStatus`, and `replicasetStatus`. Not all apps will have all resource types — include the ones that exist. Failure messages must name the component and describe the operational impact. Demo by inducing a failure, running the bundle, and showing the analyzer surfaces it with an actionable message. | |
| 3.5 | `textAnalyze` catches a known app failure pattern | The bundle spec includes at least one `textAnalyze` analyzer that searches collected log files with a regex for a failure mode specific to the app's components. The failure message must explain what went wrong and include a remediation step or documentation link. Show the analyzer firing on a log file containing the pattern. | The pattern must be specific to the app's failure modes. If it doesn't appear in the app's logs, it does not qualify. |
| 3.6 | Storage class and node readiness verified | The bundle spec includes a `storageClass` analyzer that fails when no default storage class is present, and a `nodeResources` analyzer that fails when any node is not Ready. Both must have clear, actionable failure messages. | |
| 3.7 | Support bundle generated from app UI and uploaded to Vendor Portal | Add a "Generate Support Bundle" action in the app UI. Triggering it collects a support bundle and uploads it to the Vendor Portal automatically using the SDK. Show the resulting bundle appearing on the Instance Details page in the Vendor Portal, then open it and walk through the collected data and analyzer results. | |

---

## Tier 4: Ship It on a VM

> All embedded cluster tasks use Embedded Cluster v3.

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 4.1 | App installs on a bare VM using embedded cluster and is accessible | Starting from a fresh VM, complete the embedded cluster install. Show `sudo k0s kubectl get pods -A` with all pods Running, then open the app in a browser. | |
| 4.2 | In-place upgrade without data loss | Install release 1. Create some data in the app. Trigger the upgrade to release 2 via the installer. Show the data still present and all pods Running after upgrade. | |
| 4.3 | Air-gapped install | Build an air gap bundle from your release. Transfer it to a VM. Complete the install using only the bundle, including the embedded dependencies. Show all pods Running and open the app in a browser. | |
| 4.6 | App icon and name set correctly | Screenshot of the installer showing the correct icon and app name | |
| 4.7 | License entitlement gates a configurable feature | Define a license field in the Vendor Portal that controls access to a specific app feature. With the entitlement disabled, the feature's config screen item is hidden or locked and the feature is unavailable. Update the license to enable the entitlement, show the config item is now accessible and the feature can be configured and used. | Embedded cluster path uses KOTS `LicenseFieldValue` template in `helmchart.yaml` |

---

## Config Screen

> The config screen must have at least 3 meaningful capabilities wired through to Helm.

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 5.0 | External stateful component toggle with conditional fields | Install twice: once with the embedded component (show pod Running in `sudo k0s kubectl get pods -A`), once with the external component (show no pod, app is using the external instance). Show the config screen: selecting external reveals connection fields (host, port, credentials); selecting embedded hides them. | |
| 5.1 | Configurable app feature wired through the config screen | Enable the feature via the config screen and show it working in the app. Disable it and show it is gone. | At least 2 features required to meet the config screen threshold. Features should be non-trivial. |
| 5.2 | Generated default value survives upgrade | Use a config item that auto-generates a default embedded database password on first install. Perform an upgrade and show the app still connects to the database successfully without reconfiguring. | Classic failure: the generated value changes on upgrade, so the app loses its database connection. |
| 5.3 | Input validation | At least one config item must use regex validation. Show the config screen blocking progress with a clear validation message when an invalid value is entered, and accepting the value when it matches the expected format. | |
| 5.4 | Help text on all config items | Show the config screen with `help_text` present on every item. Each must describe what the field does and what a valid value looks like — not just restate the field label. | |

---

## Tier 5: Deliver It

> We will use Enterprise Portal v2. See docs.

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 6.1 | Enterprise portal branding & identity | Screenshot of the Enterprise Portal showing at minimum: custom logo, favicon, title, and primary/secondary colors applied | |
| 6.2 | Enterprise portal custom email sender | Trigger an invitation email and show it arriving from your domain, not a Replicated address, fully configured for deliverability | You'll need a domain you own, or you can purchase one and expense it |
| 6.3 | Enterprise portal security center | Log in as a customer and visit Security Center to see the vulnerabilities in the application. If you have any vulns in your application image, you need to get access to the language-level securebuild image and base your application on it to reduce your CVEs. | |
| 6.4 | Enterprise portal custom setup / instructions | Create a GitHub repo, integrate the GitHub app into your vendor team account, and customize the left nav and main content in Enterprise Portal to make it relevant to your application installation and operating instructions. | |
| 6.5 | Add Helm Chart reference to EPv2 docs | Include an automated (generated) helm chart reference in your `toc.yaml` that generates a custom Helm chart reference. Ensure at least 1 field is not documented in the reference. | |
| 6.6 | Add Terraform modules to EPv2 docs | Include generated and automated Terraform modules in your Enterprise Portal v2 site, enabled or disabled by a custom license field. | NOTE: The Terraform does not need to work. Do not spend a lot of time making it work — a simple Claude-generated "fake" Terraform module is sufficient. |
| 6.8 | Enterprise portal self-serve sign-up | Share the sign-up URL, complete the sign-up flow as a new user, and show the resulting customer record appearing in the Vendor Portal Customers page. | |
| 6.9 | End-to-end install via Enterprise Portal for both install types | Invite a customer to the Enterprise Portal. As that customer, follow the install instructions through to a running app — once for the Helm install path and once for the embedded cluster install path. | Tests that the portal instructions are accurate and complete enough for a real customer to follow |
| 6.10 | Upgrade | Test that upgrade instructions work without downtime in the application (Helm and EC) | |

---

## Tier 6: Operationalize It

| # | Task | Acceptance Criteria | Notes |
|---|------|---------------------|-------|
| 7.1 | Create notifications | Show email and webhook notifications triggered on various account activity | |
| 7.2 | Explain your security posture | Be able to speak to the CVEs in your application and how you could reduce them | |
| 7.3 | Sign your images | | |
| 7.4 | Use the network policy option in CMX to confirm that air-gapped installations make 0 outbound requests | Run the application and exercise all functionality and deliver a network policy report that shows no outbound access | |
