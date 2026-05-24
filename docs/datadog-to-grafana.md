# Datadog To Grafana LGTM

OpenExit generates candidates only:

- Grafana dashboard JSON under `generated-config/grafana`.
- Prometheus rule candidates under `generated-config/prometheus`.
- OpenTelemetry Collector sketch under `generated-config/opentelemetry`.
- ArgoCD starter manifest under `generated-config/argocd`.

Every uncertain conversion is marked for manual review.

## Alert Rules

OpenExit attempts a conservative conversion for simple threshold monitors such as:

```text
sum(last_5m):sum:trace.http.request.errors{env:prod}.as_count() > 10
```

The generated PromQL remains a candidate and includes review annotations. Complex Datadog functions are not converted automatically; the source query is preserved with a `vector(0)` placeholder so the missing work is visible.

## Live Collection

The live Datadog collector stores redacted evidence for dashboards, monitors, and SLOs. Evidence refs in generated assessments resolve to local files under `evidence/datadog/`.

## Risk Rules

OpenExit v0.1 flags the migration risks listed in the implementation plan:

- Dashboard risks: unsupported widgets, large dashboards, missing owners, unknown data sources, and complex template variables.
- Monitor risks: Datadog-specific syntax, anomaly/outlier/forecast functions, composite monitors, missing owners, missing runbooks, unknown notification targets, and manual routing needs.
- SLO risks: unclear SLI mapping, target review, missing burn-rate alert mapping, and missing dashboard mapping.
- Cost and scale risks: high-cardinality tags, unknown retention, many custom metrics, unknown log volume, and unknown trace volume.
- Migration risks: alert shadowing, dashboard parity review, manual query review, notification routing review, and unclear on-call ownership.
