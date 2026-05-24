# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Validate -> Export
```

The core engine is written in Go. The first supported path is Datadog to Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches.

The v0.2 preview path adds fixture-based GitHub Enterprise to Forgejo assessment while keeping the same local-first collect, normalize, analyze, generate, validate, export pipeline.

The v0.3 preview path adds fixture-based Okta/Auth0 to Keycloak/Zitadel assessment. It reuses the same normalized inventory and evidence model, but generates identity-specific planning artifacts and a candidate realm/client YAML instead of Datadog target configs.

The v0.4 preview path adds fixture-based Cloudflare/Akamai to Varnish/HAProxy/Coraza assessment. It generates edge-specific VCL, HAProxy, and Coraza candidate files plus cache and WAF review reports.

AI assist is optional and never required for validation or export.
