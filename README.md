# OpenExit

Local-first migration assessments from proprietary SaaS platforms to open-source infrastructure.

OpenExit collects migration inventory, normalizes it, analyzes migration risks, generates candidate target files, validates outputs, and exports a local evidence bundle.

The Datadog to Grafana LGTM path includes both fixture import and a read-only live Datadog collector. The GitHub Enterprise to Forgejo path includes fixture import and a read-only GitHub/GitHub Enterprise collector for repository migration inventory. The Okta/Auth0 to Keycloak/Zitadel path includes fixture import and read-only live Okta and Auth0 collectors. The Cloudflare/Akamai to Varnish/HAProxy/Coraza path includes fixture import and read-only live Cloudflare and Akamai collectors. The OpenAI/Anthropic path includes fixture import and read-only OpenAI and Anthropic aggregate usage collectors.

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
./bin/openexit map --project ./demo
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
- `openexit collect okta --project <project-dir> --org-url https://dev-123456.okta.com [--token-env OKTA_API_TOKEN] [--break-glass-user admin@example.com]`
- `openexit collect auth0 --project <project-dir> --domain https://example.us.auth0.com [--token-env AUTH0_MANAGEMENT_TOKEN] [--break-glass-user admin@example.com]`
- `openexit collect identity-fixture --project <project-dir> --input <file>`
- `openexit collect cloudflare --project <project-dir> --zone-id <zone-id> [--token-env CLOUDFLARE_API_TOKEN]`
- `openexit collect akamai --project <project-dir> [--zone example.com] [--property-id prp_123] [--security-config-id 123:7]`
- `openexit collect edge-fixture --project <project-dir> --input <file>`
- `openexit collect openai --project <project-dir> [--admin-key-env OPENAI_ADMIN_KEY] [--days 30] [--owner team@example.com]`
- `openexit collect anthropic --project <project-dir> [--admin-key-env ANTHROPIC_ADMIN_KEY] [--days 30] [--workspace-id wrkspc_...]`
- `openexit collect ai-fixture --project <project-dir> --input <file>`
- `openexit collect datadog --project <project-dir> --site datadoghq.eu --api-key-env DATADOG_API_KEY --app-key-env DATADOG_APP_KEY`
- `openexit assess --project <project-dir> --target grafana-lgtm`
- `openexit map --project <project-dir>`
- `openexit generate --project <project-dir> --all`
- `openexit validate --project <project-dir>`
- `openexit export --project <project-dir> --format zip --out <file>`
- `openexit assist summarize --project <project-dir> --provider noop`

The Datadog, GitHub, Okta, Auth0, Cloudflare, Akamai, OpenAI, and Anthropic collectors are read-only. API tokens are read from environment variables or local credential files, are not printed, and are not stored.
When `--target` is omitted during `init`, OpenExit selects the standard target for the chosen source.

## Supported Paths

| Source | Target | Status | Collector |
| --- | --- | --- | --- |
| Datadog | Grafana LGTM, Prometheus-compatible alerting, OpenTelemetry Collector/Alloy | Primary path | Fixture and read-only live Datadog collector |
| GitHub Enterprise | Forgejo | Repository migration assessment path | Fixture and read-only live GitHub/GitHub Enterprise collector |
| Okta/Auth0 | Keycloak/Zitadel | Identity migration assessment path | Fixture and read-only live Okta/Auth0 collectors |
| Cloudflare/Akamai | Varnish/HAProxy/Coraza | Edge migration assessment path | Fixture and read-only live Cloudflare/Akamai collectors |
| OpenAI/Anthropic | vLLM/LiteLLM | AI provider migration assessment path | Fixture and read-only live OpenAI/Anthropic aggregate usage collectors |

Fixture workflows run the full local OpenExit workflow with sample or customer-provided JSON fixture data. They are assessment and planning tools for offline review.

## Current Scope

Included in the current implementation:

- CLI skeleton and project init/status.
- Typed project, inventory, assessment, mapping, and validation manifests.
- Fixture-based Datadog inventory import.
- Read-only Datadog collection for dashboards, monitors, and SLOs.
- Read-only GitHub/GitHub Enterprise collection for repositories, teams, branch protection, Actions workflows, secret metadata, runners, and deploy keys.
- Read-only Okta collection for applications, groups, policy/rule metadata, org MFA factors, and explicit break-glass user metadata.
- Read-only Auth0 collection for clients, roles, action/rule metadata, Guardian MFA factors, and explicit break-glass user metadata.
- Read-only Cloudflare collection for DNS records, WAF rulesets, cache rules, redirects, inferred origins, TLS settings, bot rules, and page rules.
- Read-only Akamai collection for Edge DNS recordsets, Property Manager hostnames/rules, origins, cache behaviors, redirects, TLS/HSTS metadata, Bot Manager behavior metadata, and optional AppSec custom-rule metadata.
- Read-only OpenAI collection for model-grouped aggregate completions usage, token volumes, available model metadata, and hourly peak estimates.
- Read-only Anthropic collection for model-grouped Messages API token usage, server web-search tool metadata, filters, and hourly peak estimates.
- Deterministic risk assessment.
- Deterministic source-to-target mapping manifest and summary.
- Markdown handover artifacts.
- Grafana dashboard candidate JSON.
- Prometheus alert rule candidate YAML with simple Datadog threshold conversion hints.
- OpenTelemetry Collector sketch.
- ArgoCD starter manifest.
- Typed migration plan manifest and phase-gate Markdown plan.
- Validation report with YAML/JSON parsing, evidence ref checks, secret scan, and optional `promtool`/`kubeconform` checks.
- Evidence bundle export.
- No-op assist provider and explicit opt-in LiteLLM assist.
- GitHub Enterprise to Forgejo assessment path with fixture import and live repository inventory collection.
- Okta/Auth0 to Keycloak/Zitadel assessment path with fixture import and live Okta/Auth0 identity inventory collection.
- Cloudflare/Akamai to Varnish/HAProxy/Coraza assessment path with fixture import and live Cloudflare/Akamai edge inventory collection.
- OpenAI/Anthropic to vLLM/LiteLLM assessment path with fixture import and live OpenAI/Anthropic aggregate usage inventory collection.

Not included in the current release:

- Automatic cutover.
- Production apply.
- Hosted portal.
- Perfect Datadog to Grafana parity.
- AI-required decision making.

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

The assessment engine includes dashboard, monitor, SLO, cost, scale, identity, edge, repository, and AI provider risk rules from the implementation plan. Findings have stable IDs, severity, affected assets, evidence refs, and recommendations. `openexit map` writes a typed mapping manifest under `mapping/`; `generate --all` refreshes mapping and writes a typed migration plan under `assessment/` with assessment, pilot, shadow, and cutover phase gates.

## Release Process

The release checklist lives in `docs/release.md`. A release build should pass `make verify`, `make release-dist VERSION=0.1.0`, the Datadog definition-of-done pipeline, and validation/export for every supported assessment path.

## Assessment Paths

GitHub Enterprise to Forgejo collects repository, team, branch protection, Actions workflow, secret metadata, runner, and deploy key metadata from live GitHub/GitHub Enterprise APIs or local fixtures. GitHub App metadata is currently fixture-only. It generates Forgejo migration assessment, CI compatibility, branch protection mapping, runner migration, repository ownership reports, and a Forgejo migration candidate YAML.

Okta/Auth0 to Keycloak/Zitadel collects applications, SAML/OIDC client metadata, groups, policies, MFA settings, redirect URIs, owners, and break-glass account metadata from live Okta/Auth0 APIs or local fixtures. It generates identity migration risk, realm/client candidate config, break-glass, cutover, and rollback artifacts.

Cloudflare/Akamai to Varnish/HAProxy/Coraza collects DNS records, WAF rules, cache rules, redirects, origins, TLS settings, bot rules, and page rules from live Cloudflare/Akamai APIs or local fixtures. The Akamai collector uses read-only EdgeGrid-authenticated calls for Edge DNS, Property Manager, and optional AppSec metadata. It generates VCL, HAProxy, Coraza, cache parity, and WAF enforcement review artifacts.

OpenAI/Anthropic to vLLM/LiteLLM collects model usage classes, token volumes, latency expectations, sensitive prompt categories, tool usage, and fallback behavior from local fixtures. It can also collect model-grouped aggregate OpenAI completions usage, aggregate Anthropic Messages API usage, available model metadata where exposed, server web-search tool metadata, and hourly peak estimates from live provider APIs without storing prompts or credentials. The path generates self-hosted LLM readiness, LiteLLM routing, vLLM sizing, evaluation, and data sensitivity artifacts.

## License

Apache-2.0.
