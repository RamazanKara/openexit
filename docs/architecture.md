# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Validate -> Export
```

The core engine is written in Go. The primary live-collector path is Datadog to Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches.

GitHub Enterprise to Forgejo is a fixture-complete assessment path that keeps the same local-first collect, normalize, analyze, generate, validate, export pipeline.

Okta/Auth0 to Keycloak/Zitadel is a fixture-complete assessment path. It reuses the same normalized inventory and evidence model, but generates identity-specific planning artifacts and a candidate realm/client YAML instead of Datadog target configs.

Cloudflare/Akamai to Varnish/HAProxy/Coraza is a fixture-complete assessment path. It generates edge-specific VCL, HAProxy, and Coraza candidate files plus cache and WAF review reports.

OpenAI/Anthropic to vLLM/LiteLLM is a fixture-complete assessment path. It generates AI provider readiness, LiteLLM routing, vLLM sizing, evaluation, and data sensitivity artifacts.

AI assist is optional and never required for validation or export.
