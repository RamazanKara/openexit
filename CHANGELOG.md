# Changelog

## Unreleased

## 0.1.0 - 2026-07-18

- Refocused the v0.1 product on one workflow: `openexit datadog scan`, `plan --target grafana-lgtm`, and `export --out <directory>`.
- Added a versioned, endpoint-auditable Datadog observability catalog with GET-only collection, full pagination, redacted evidence, fail-closed partial-scan handling, and stale-plan invalidation.
- Added deterministic per-resource and per-component conversion statuses (`exact`, `approximate`, `manual`, and `unsupported`) with explicit semantic changes and no fake `vector(0)` alert placeholders.
- Added source-linked Grafana dashboards, Prometheus alert candidates, Alloy/OpenTelemetry baselines, a self-contained HTML migration report, and a transparent exit-readiness formula.
- Added schema-backed provenance and validation manifests plus transactional directory export with per-file source references and `SHA256SUMS`.
- Moved the earlier multi-provider engine under `openexit experimental` while retaining hidden root aliases for compatibility.
- Added `openexit doctor` for local runtime diagnostics covering version metadata, embedded schemas, and optional validator availability.
- Added preview support for fixture-based OpenAI/Anthropic to vLLM/LiteLLM assessment.
- Added explicit source/target project initialization and validation consistency checks for all assessment paths.
- Clarified fixture-only paths as feature-complete local assessment workflows instead of unfinished versioned scaffolds.
- Added early source/target guardrails for collection and assessment commands plus supported path-pair validation.
- Added a Forgejo migration candidate config artifact for the GitHub Enterprise assessment path.
- Added a read-only GitHub/GitHub Enterprise collector for repository, team, branch protection, Actions workflow, secret metadata, runner, and deploy-key inventory.
- Added a read-only Okta collector for application, group, policy/rule, MFA factor, and explicit break-glass identity inventory.
- Added a read-only Auth0 collector for client, role, action/rule, Guardian MFA factor, and explicit break-glass identity inventory.
- Added a read-only Cloudflare collector for DNS, WAF/ruleset, cache, redirect, TLS, origin, bot-rule, and page-rule edge inventory.
- Added a read-only Akamai collector for Edge DNS, Property Manager rule-tree, hostname, origin, cache, redirect, TLS/HSTS, Bot Manager, and optional AppSec custom-rule metadata.
- Added a read-only OpenAI collector for aggregate model usage, token volumes, available model metadata, and peak throughput estimates.
- Added a read-only Anthropic collector for aggregate Messages API usage, token volumes, server web-search tool metadata, filters, and peak throughput estimates.
- Added typed migration plan generation with assessment, pilot, shadow, and cutover phase gates.
- Added explicit source-to-target mapping manifests and `openexit map`.
- Added embedded JSON Schema validation for project, inventory, assessment, mapping, migration plan, and validation report manifests.
- Added Grafana dashboard candidate validation for mapping, source metadata, candidate safety flags, panels, and unsupported-widget notes.
- Added Prometheus alert-rule candidate validation for mapping, source monitor metadata, candidate safety labels, review annotations, and manual placeholders.
- Added OpenTelemetry Collector candidate validation for OTLP receivers, metric/trace pipelines, exporters, candidate warnings, and README production guidance.
- Added ArgoCD candidate validation for Application shape, candidate labels, placeholder repository URL, and disabled automated sync.
- Added validation and export path-safety checks that reject symlinks in evidence bundle inputs.
- Added best-effort live GitHub App installation metadata collection for the GitHub Enterprise to Forgejo path.
- Added referenced metric and tag metadata capture to the live Datadog collector.
- Added best-effort live Datadog integration installation metadata collection.
- Added a reproducible Datadog example refresh target and checked-in full example output.
- Added fixture integration evidence refs and validation coverage for integration evidence.
- Added Forgejo migration candidate validation for the GitHub Enterprise assessment path.
- Added identity realm/client candidate validation for the Okta/Auth0 assessment path.
- Added edge VCL, HAProxy, and Coraza candidate validation for the Cloudflare/Akamai assessment path.
- Added LiteLLM/vLLM config candidate validation for the OpenAI/Anthropic assessment path.
- Added project readiness status with pipeline summaries, export readiness, next actions, and `--json` output.
- Added `openexit run` to execute assessment, mapping, full generation, validation, status reporting, and optional evidence bundle export for collected projects.
- Added `openexit demo` with embedded redacted fixtures for complete release-binary demos across supported assessment paths.
- Added a machine-readable evidence bundle manifest with build metadata, validation summary, and per-file SHA-256 digests.
- Added a public JSON Schema for evidence bundle manifests and test coverage that validates exported manifests against it.
- Added `openexit verify-bundle` for offline evidence bundle verification with text and JSON reports.
- Added `openexit release-manifest` and `openexit verify-release` for schema-backed release artifact manifests and offline binary/checksum verification.
- Added a release installer script that detects the local platform, verifies the selected artifact against `SHA256SUMS` and `RELEASE_MANIFEST.json`, and is smoke-tested by the release gate.
- Added `openexit completion` plus release completion assets for Bash, Zsh, Fish, and PowerShell.
- Extended release manifests and `SHA256SUMS` to cover installer and shell-completion assets as well as platform binaries.
- Added `openexit sbom` and a verified release `SBOM.cdx.json` asset with CycloneDX JSON dependency metadata.
- Added `make release-check` and wired the draft-release workflow to run the full release gate before packaging artifacts.
- Updated push CI to run the same release readiness gate, including smoke pipelines and bundle verification.
- Updated CI and release workflows to Node.js 24-native GitHub Actions.

- Initial OpenExit implementation.
- Added mocked live Datadog collector coverage and evidence ref hardening.
- Added conservative simple Datadog threshold to PromQL candidate conversion.
- Added explicit opt-in LiteLLM assist with redacted input handling and audit-file support.
- Added full v0.1 risk-rule coverage for dashboard, monitor, SLO, cost, scale, and migration risks.
- Added definition-of-done coverage for generated layout, export refusal, and bundle checksum verification.
- Added CI, release artifact builds, version stamping, expanded secret scanning, and release documentation.
- Added preview support for fixture-based GitHub Enterprise to Forgejo assessment.
- Added preview support for fixture-based Okta/Auth0 to Keycloak/Zitadel assessment.
- Added preview support for fixture-based Cloudflare/Akamai to Varnish/HAProxy/Coraza assessment.
