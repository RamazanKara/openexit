# Changelog

## Unreleased

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
- Updated CI and release workflows to Node.js 24-native GitHub Actions.

## 0.1.0 - 2026-05-24

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
