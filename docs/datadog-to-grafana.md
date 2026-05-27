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

Validation checks Grafana dashboard candidate JSON against OpenExit's migration safety contract: mapped dashboard paths must exist, source metadata must match inventory, candidate dashboards must stay marked `productionReady=false`, panels must preserve source query hints, and unsupported widgets must be documented in the Grafana README.

Validation also checks Prometheus alert-rule candidates against mapping and inventory. Every generated alert must preserve its Datadog monitor ID and source query, keep `openexit_candidate=true` and `production_ready=false`, include manual review annotations, and keep uncertain conversions on explicit `vector(0)` placeholders.

Validation checks the OpenTelemetry Collector candidate for OTLP receivers, memory limiter and batch processors, metrics and traces pipelines, Mimir and Tempo placeholder exporters, candidate warnings, README production-change guidance, and absence of secret-like content.

## Live Collection

The live Datadog collector stores redacted evidence for dashboards, monitors, and SLOs. Evidence refs in generated assessments resolve to local files under `evidence/datadog/`.

## Risk Rules

OpenExit v0.1 flags the migration risks listed in the implementation plan:

- Dashboard risks: unsupported widgets, large dashboards, missing owners, unknown data sources, and complex template variables.
- Monitor risks: Datadog-specific syntax, anomaly/outlier/forecast functions, composite monitors, missing owners, missing runbooks, unknown notification targets, and manual routing needs.
- SLO risks: unclear SLI mapping, target review, missing burn-rate alert mapping, and missing dashboard mapping.
- Cost and scale risks: high-cardinality tags, unknown retention, many custom metrics, unknown log volume, and unknown trace volume.
- Migration risks: alert shadowing, dashboard parity review, manual query review, notification routing review, and unclear on-call ownership.
