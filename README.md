# OpenExit

Local-first migration assessments from proprietary SaaS platforms to open-source infrastructure.

OpenExit v0.1 focuses on Datadog to Grafana LGTM, Prometheus-compatible alerting, and OpenTelemetry Collector or Grafana Alloy. It collects inventory, normalizes it, analyzes migration risks, generates candidate target files, validates outputs, and exports a local evidence bundle.

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
./bin/openexit init ./demo
./bin/openexit collect fixture --project ./demo --input ./testdata/datadog/small.json
./bin/openexit assess --project ./demo --target grafana-lgtm
./bin/openexit generate --project ./demo --all
./bin/openexit validate --project ./demo
./bin/openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

## Commands

- `openexit version`
- `openexit init <project-dir>`
- `openexit status --project <project-dir>`
- `openexit collect fixture --project <project-dir> --input <file>`
- `openexit collect github-fixture --project <project-dir> --input <file>`
- `openexit collect identity-fixture --project <project-dir> --input <file>`
- `openexit collect edge-fixture --project <project-dir> --input <file>`
- `openexit collect datadog --project <project-dir> --site datadoghq.eu --api-key-env DATADOG_API_KEY --app-key-env DATADOG_APP_KEY`
- `openexit assess --project <project-dir> --target grafana-lgtm`
- `openexit generate --project <project-dir> --all`
- `openexit validate --project <project-dir>`
- `openexit export --project <project-dir> --format zip --out <file>`
- `openexit assist summarize --project <project-dir> --provider noop`

The Datadog collector is read-only. API keys are read from environment variables, are not printed, and are not stored.

## Current Scope

Included in this first implementation:

- CLI skeleton and project init/status.
- Typed project, inventory, assessment, mapping, and validation manifests.
- Fixture-based Datadog inventory import.
- Read-only Datadog collection for dashboards, monitors, and SLOs.
- Deterministic risk assessment.
- Markdown handover artifacts.
- Grafana dashboard candidate JSON.
- Prometheus alert rule candidate YAML with simple Datadog threshold conversion hints.
- OpenTelemetry Collector sketch.
- ArgoCD starter manifest.
- Validation report with YAML/JSON parsing, evidence ref checks, secret scan, and optional `promtool`/`kubeconform` checks.
- Evidence bundle export.
- No-op assist provider and explicit opt-in LiteLLM assist.
- Fixture-based GitHub Enterprise to Forgejo assessment preview.
- Fixture-based Okta/Auth0 to Keycloak/Zitadel assessment preview.
- Fixture-based Cloudflare/Akamai to Varnish/HAProxy/Coraza assessment preview.

Not included in v0.1:

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

The v0.1 assessment engine includes dashboard, monitor, SLO, cost, scale, and migration risk rules from the implementation plan. Findings have stable IDs, severity, affected assets, evidence refs, and recommendations.

## v0.2 Preview

The first v0.2 scaffold supports GitHub Enterprise fixture inventory for Forgejo migration assessment. It collects repository, team, branch protection, Actions workflow, secret metadata, runner, deploy key, and GitHub App metadata from local fixtures, then generates Forgejo migration reports without calling a live API.

## v0.3 Preview

The v0.3 scaffold supports Okta/Auth0-like identity fixture inventory for Keycloak/Zitadel assessment. It collects applications, SAML/OIDC client metadata, groups, policies, MFA settings, redirect URIs, owners, and break-glass account metadata from local fixtures, then generates identity migration reports and a candidate realm/client YAML without calling a live API.

## v0.4 Preview

The v0.4 scaffold supports Cloudflare/Akamai-like edge fixture inventory for Varnish, HAProxy, and Coraza assessment. It collects DNS records, WAF rules, cache rules, redirects, origins, TLS settings, bot rules, and page rules from local fixtures, then generates VCL, HAProxy, Coraza, cache parity, and WAF enforcement review artifacts without calling a live API.

## License

Apache-2.0.
