# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Validate -> Export
```

The core engine is written in Go. The first supported path is Datadog to Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches.

The v0.2 preview path adds fixture-based GitHub Enterprise to Forgejo assessment while keeping the same local-first collect, normalize, analyze, generate, validate, export pipeline.

AI assist is optional and never required for validation or export.
