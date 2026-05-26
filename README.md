# OpenExit

Local-first migration assessments from proprietary SaaS platforms to open-source infrastructure.

OpenExit collects migration inventory, normalizes it, analyzes migration risks, generates candidate target files, validates outputs, and exports a local evidence bundle.

The Datadog to Grafana LGTM path includes both fixture import and a read-only live Datadog collector. The GitHub Enterprise to Forgejo path includes fixture import and a read-only GitHub/GitHub Enterprise collector for repository migration inventory. The Okta/Auth0, Cloudflare/Akamai, and OpenAI/Anthropic paths are feature-complete for local fixture assessment workflows: they collect fixture inventory, generate the planned artifacts, validate evidence, and export bundles, but they do not include live SaaS collectors yet.

## Safety Model

- No production writes.
- No one-click migration.
- No hidden hosted backend.
- No credential storage.
- No direct SaaS deletion.
- No AI dependency.
- No generated config is production-ready without review.

## Quick Start

```bash
make build
./bin/openexit init ./demo --source datadog --target grafana-lgtm
./bin/openexit collect fixture --project ./demo --input ./testdata/datadog/small.json
./bin/openexit assess --project ./demo --target grafana-lgtm
./bin/openexit generate --project ./demo --all
./bin/openexit validate --project ./demo
./bin/openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

## Install From Source

```bash
git clone https://github.com/RamazanKara/openexit.git
cd openexit
make verify
make build VERSION=0.1.0
./bin/openexit version
```

Release candidates can be built locally with:

```bash
make release-dist VERSION=0.1.0
```

This writes OS/architecture binaries and `dist/SHA256SUMS`.

## Commands

- `openexit version`
- `openexit init <project-dir> [--source <type> --target <type>]`
- `openexit status --project <project-dir>`
- `openexit collect fixture --project <project-dir> --input <file>`
- `openexit collect github --project <project-dir> --owner <org> [--base-url https://github.example.com/api/v3] [--token-env GITHUB_TOKEN] [--repo owner/name]`
- `openexit collect github-fixture --project <project-dir> --input <file>`
- `openexit collect identity-fixture --project <project-dir> --input <file>`
- `openexit collect edge-fixture --project <project-dir> --input <file>`
- `openexit collect ai-fixture --project <project-dir> --input <file>`
- `openexit collect datadog --project <project-dir> --site datadoghq.eu --api-key-env DATADOG_API_KEY --app-key-env DATADOG_APP_KEY`
- `openexit assess --project <project-dir> --target grafana-lgtm`
- `openexit generate --project <project-dir> --all`
- `openexit validate --project <project-dir>`
- `openexit export --project <project-dir> --format zip --out <file>`
- `openexit assist summarize --project <project-dir> --provider noop`

The Datadog and GitHub collectors are read-only. API tokens are read from environment variables, are not printed, and are not stored.
When `--target` is omitted during `init`, OpenExit selects the standard target for the chosen source.

## Supported Paths

| Source | Target | Status | Collector |
| --- | --- | --- | --- |
| Datadog | Grafana LGTM, Prometheus-compatible alerting, OpenTelemetry Collector/Alloy | Primary path | Fixture and read-only live Datadog collector |
| GitHub Enterprise | Forgejo | Repository migration assessment path | Fixture and read-only live GitHub/GitHub Enterprise collector |
| Okta/Auth0 | Keycloak/Zitadel | Fixture-complete assessment path | Fixture only |
| Cloudflare/Akamai | Varnish/HAProxy/Coraza | Fixture-complete assessment path | Fixture only |
| OpenAI/Anthropic | vLLM/LiteLLM | Fixture-complete assessment path | Fixture only |

Fixture-only paths run the full local OpenExit workflow with sample or customer-provided JSON fixture data. They are assessment and planning tools, not live collectors.

## Current Scope

Included in the current implementation:

- CLI skeleton and project init/status.
- Typed project, inventory, assessment, mapping, and validation manifests.
- Fixture-based Datadog inventory import.
- Read-only Datadog collection for dashboards, monitors, and SLOs.
- Read-only GitHub/GitHub Enterprise collection for repositories, teams, branch protection, Actions workflows, secret metadata, runners, and deploy keys.
- Deterministic risk assessment.
- Markdown handover artifacts.
- Grafana dashboard candidate JSON.
- Prometheus alert rule candidate YAML with simple Datadog threshold conversion hints.
- OpenTelemetry Collector sketch.
- ArgoCD starter manifest.
- Validation report with YAML/JSON parsing, evidence ref checks, secret scan, and optional `promtool`/`kubeconform` checks.
- Evidence bundle export.
- No-op assist provider and explicit opt-in LiteLLM assist.
- GitHub Enterprise to Forgejo assessment path with fixture import and live repository inventory collection.
- Fixture-complete Okta/Auth0 to Keycloak/Zitadel assessment path.
- Fixture-complete Cloudflare/Akamai to Varnish/HAProxy/Coraza assessment path.
- Fixture-complete OpenAI/Anthropic to vLLM/LiteLLM assessment path.

Not included in the current release:

- Automatic cutover.
- Production apply.
- Hosted portal.
- Perfect Datadog to Grafana parity.
- AI-required decision making.
- Live collectors for Okta/Auth0, Cloudflare/Akamai, or OpenAI/Anthropic.

## Optional Assist

`openexit assist summarize` defaults to the local no-op provider. LiteLLM is available only when `openexit.yaml` explicitly sets `policy.allowAI: true`, `assist.enabled: true`, `assist.provider: litellm`, and `assist.allowExternalProvider: true`.

Assist inputs are redacted before provider calls, outputs must use `.ai.md`, and deterministic artifacts are never overwritten.

## Candidate Conversion Policy

OpenExit is intentionally conservative:

- Simple Datadog metric thresholds can be translated into PromQL candidates.
- Complex functions such as anomaly, outlier, forecast, timeshift, or composite behavior stay as `vector(0)` placeholders with source queries preserved.
- Every generated alert remains labeled `openexit_candidate=true` and `production_ready=false`.
- Human review and shadowing are required before operational use.

## Risk Coverage

The assessment engine includes dashboard, monitor, SLO, cost, scale, identity, edge, repository, and AI provider risk rules from the implementation plan. Findings have stable IDs, severity, affected assets, evidence refs, and recommendations.

## Release Process

The release checklist lives in `docs/release.md`. A release build should pass `make verify`, `make release-dist VERSION=0.1.0`, the Datadog definition-of-done pipeline, and validation/export for every fixture-only path.

## Assessment Paths

GitHub Enterprise to Forgejo collects repository, team, branch protection, Actions workflow, secret metadata, runner, and deploy key metadata from live GitHub/GitHub Enterprise APIs or local fixtures. GitHub App metadata is currently fixture-only. It generates Forgejo migration assessment, CI compatibility, branch protection mapping, runner migration, repository ownership reports, and a Forgejo migration candidate YAML.

Okta/Auth0 to Keycloak/Zitadel collects applications, SAML/OIDC client metadata, groups, policies, MFA settings, redirect URIs, owners, and break-glass account metadata from local fixtures. It generates identity migration risk, realm/client candidate config, break-glass, cutover, and rollback artifacts.

Cloudflare/Akamai to Varnish/HAProxy/Coraza collects DNS records, WAF rules, cache rules, redirects, origins, TLS settings, bot rules, and page rules from local fixtures. It generates VCL, HAProxy, Coraza, cache parity, and WAF enforcement review artifacts.

OpenAI/Anthropic to vLLM/LiteLLM collects model usage classes, token volumes, latency expectations, sensitive prompt categories, tool usage, and fallback behavior from local fixtures. It generates self-hosted LLM readiness, LiteLLM routing, vLLM sizing, evaluation, and data sensitivity artifacts.

## License

Apache-2.0.
