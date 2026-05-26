# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Validate -> Export
```

The core engine is written in Go. The primary Datadog live-collector path targets Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches.

GitHub Enterprise to Forgejo keeps the same local-first collect, normalize, analyze, generate, validate, export pipeline. It supports fixture import and a read-only live GitHub/GitHub Enterprise collector for repository migration inventory.

Okta/Auth0 to Keycloak/Zitadel reuses the same normalized inventory and evidence model, but generates identity-specific planning artifacts and a candidate realm/client YAML instead of Datadog target configs. It supports fixture import and a read-only live Okta collector; Auth0 remains fixture-only.

Cloudflare/Akamai to Varnish/HAProxy/Coraza generates edge-specific VCL, HAProxy, and Coraza candidate files plus cache and WAF review reports. It supports fixture import and a read-only live Cloudflare collector; Akamai remains fixture-only.

OpenAI/Anthropic to vLLM/LiteLLM is a fixture-complete assessment path. It generates AI provider readiness, LiteLLM routing, vLLM sizing, evaluation, and data sensitivity artifacts.

AI assist is optional and never required for validation or export.
