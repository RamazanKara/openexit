# Changelog

## Unreleased

- Added preview support for fixture-based OpenAI/Anthropic to vLLM/LiteLLM assessment.
- Added explicit source/target project initialization and validation consistency checks for all assessment paths.
- Clarified fixture-only paths as feature-complete local assessment workflows instead of unfinished versioned scaffolds.
- Added early source/target guardrails for collection and assessment commands plus supported path-pair validation.

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
