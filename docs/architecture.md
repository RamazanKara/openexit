# Architecture

OpenExit uses a deterministic pipeline:

```text
Collect -> Normalize -> Analyze -> Map -> Generate -> Validate -> Export
```

The core engine is written in Go. The first supported path is Datadog to Grafana LGTM with Prometheus-compatible alerting and OpenTelemetry Collector or Alloy sketches.

AI assist is optional and never required for validation or export.
