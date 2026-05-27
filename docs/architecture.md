# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Plan -> Validate -> Export
```

The core engine is written in Go. The primary Datadog live-collector path targets Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches. The map stage writes deterministic source-to-target mapping manifests under `mapping/`; `generate --all` refreshes that mapping and also writes a typed migration plan that groups required artifacts into assessment, pilot, shadow, and cutover phase gates. The validator embeds the public JSON Schemas into the release binary so generated manifests can be checked without a checked-out repository.

GitHub Enterprise to Forgejo keeps the same local-first collect, normalize, analyze, generate, validate, export pipeline. It supports fixture import and a read-only live GitHub/GitHub Enterprise collector for repository migration inventory.

Okta/Auth0 to Keycloak/Zitadel reuses the same normalized inventory and evidence model, but generates identity-specific planning artifacts and a candidate realm/client YAML instead of Datadog target configs. It supports fixture import and read-only live Okta/Auth0 collectors.

Cloudflare/Akamai to Varnish/HAProxy/Coraza generates and validates edge-specific VCL, HAProxy, and Coraza candidate files plus cache and WAF review reports. It supports fixture import plus read-only live Cloudflare and Akamai collectors.

OpenAI/Anthropic to vLLM/LiteLLM generates AI provider readiness, validated LiteLLM routing, vLLM sizing, evaluation, and data sensitivity artifacts. It supports fixture import plus read-only live OpenAI/Anthropic aggregate usage collectors.

AI assist is optional and never required for validation or export.
