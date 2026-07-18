# Datadog to Grafana LGTM

OpenExit v0.1 generates a reviewable migration plan from Datadog to Grafana, Prometheus, Grafana Alloy, and the OpenTelemetry Collector.

## Commands

```bash
openexit datadog scan
openexit datadog plan --target grafana-lgtm
openexit datadog export --out migration/
```

All commands use `.openexit/` as local state unless `--workdir` is provided. A new scan invalidates the previous plan and generated candidates so stale output cannot be mistaken for the current snapshot.

`scan` supports:

- `--site` for Datadog sites such as `datadoghq.com`, `datadoghq.eu`, `us3.datadoghq.com`, `us5.datadoghq.com`, `ap1.datadoghq.com`, `ap2.datadoghq.com`, `uk1.datadoghq.com`, `ddog-gov.com`, and `us2.ddog-gov.com`;
- `--api-key-env` and `--app-key-env` to select credential environment variables;
- `--fixture` for an offline, local fixture snapshot;
- `--allow-partial` to explicitly accept incomplete endpoint coverage.

`plan` supports only `--target grafana-lgtm` in v0.1. `export` writes a directory and refuses to replace an existing target unless `--force` is passed.

## Versioned Inventory Catalog

The inventory records catalog version `datadog-observability/v1`, family status, and endpoint-level status/count/message data.

| Family | GET endpoints |
| --- | --- |
| Dashboards | `/api/v1/dashboard`, `/api/v1/dashboard/lists/manual`, `/api/v2/powerpacks`, plus per-resource detail endpoints |
| Alerting | `/api/v1/monitor`, `/api/v2/monitor/policy`, `/api/v2/downtime` |
| SLOs | `/api/v1/slo`, `/api/v1/slo/correction` |
| Notebooks | `/api/v1/notebooks` |
| Synthetics | `/api/v1/synthetics/tests`, `/api/v1/synthetics/variables`, `/api/v1/synthetics/locations`, plus test details |
| Metrics | `/api/v2/metrics` |
| Logs | pipeline, pipeline-order, index, archive, and log-metric configuration endpoints |
| APM | retention-filter and span-metric configuration endpoints |
| Services | `/api/v2/services/definitions` |
| Integrations | installed integration metadata and AWS, Azure, GCP, and legacy GCP account endpoints |

List endpoints are fully paginated according to their API style: start/count, page/page-size, offset/limit, bracketed page offset or page number, and cursor pagination are all handled explicitly. Dashboard, Powerpack, and Synthetic test details are fetched after listing. Dashboard-list membership is stored as related evidence.

Installed-integration inventory excludes uninstalled marketplace entries. A response that cannot prove completeness marks its endpoint and family partial or failed. OpenExit persists the partial inventory for review but returns an error unless `--allow-partial` is explicit.

## Dashboard Conversion

OpenExit always emits one Grafana dashboard candidate for each Datadog dashboard. It evaluates every discovered widget query or formula, not just the first expression.

- Markdown/note content with a deterministic representation is preserved as an `exact` component.
- Simple metric queries using `avg`, `sum`, `min`, `max`, or `count`, exact tag filters (or the unfiltered `{*}` selector), and simple group-by tags become PromQL candidates marked `approximate`.
- Datadog formulas, event/log/APM query syntax, wildcard tag values, negated tag filters, and unsupported widgets become visible text review panels marked `manual`.
- A dashboard with any manual component has overall `manual` status. Every other generated dashboard is `approximate` because OpenExit normalizes layout and adds Grafana candidate/data-source metadata, even when all represented text components are exact.

Each generated dashboard contains an `openexit` metadata object with source reference, evidence path, status, ruleset version, and `productionReady: false`. Each executable target preserves its original Datadog query and source path.

## Monitor Conversion

Only a deliberately narrow static-threshold grammar becomes a Prometheus rule candidate. It recognizes simple metric windows such as:

```text
sum(last_5m):avg:trace.http.request.errors{env:prod} > 10
```

The result remains `approximate` because metric names, tag-to-label mapping, rollups, missing-data behavior, evaluation delay, routing, and shadowing can differ. Candidate rules preserve the source query and review guidance, and carry these labels:

```yaml
openexit_candidate: "true"
production_ready: "false"
source: datadog
source_ref: datadog:monitor:...
conversion: approximate
```

Anomaly, outlier, forecast, change, timeshift, composite, and other unsupported monitor behavior remains `manual`. No Prometheus file is emitted for it. In particular, OpenExit does not emit `vector(0)` or another executable placeholder.

## Alloy and OpenTelemetry

OpenExit emits credential-free OTLP baselines for Alloy and the OpenTelemetry Collector. Integration, log, APM, and service-definition resources link to both files, but remain `manual`: source-specific receivers, endpoints, authentication, TLS, processors, sizing, and routing cannot be reconstructed safely from Datadog control-plane metadata alone.

The baseline uses the `OPENEXIT_OTLP_ENDPOINT` environment placeholder. It contains no source credentials and is marked as a review candidate.

## Provenance

Every inventory resource has:

- a stable `datadog:<kind>:<id>` reference;
- a redacted evidence file and SHA-256 digest;
- a source URL where Datadog exposes a useful UI location;
- dependency references where they are discoverable.

Every conversion record links the source reference, evidence path, status, reason codes, semantic changes, component decisions, and generated outputs. The export manifest reverses this mapping by listing source references for evidence and generated files.

## Validation and Export Gate

Planning validates:

- inventory and evidence digests;
- inventory-to-plan identity and complete conversion coverage;
- generated-file provenance and path safety;
- Grafana candidate structure and safety metadata;
- Prometheus rule structure, source annotations, and absence of fake placeholders;
- Alloy and OpenTelemetry baseline structure and source links;
- secret-like content and symlinks;
- JSON Schema conformance;
- every local link in the static HTML report.

Export reruns validation against the current workspace. It refuses a stale plan, incomplete catalog without explicit acceptance, failed critical checks, an existing destination without `--force`, symlinks, unsafe paths, or secret-like output. Replacement with `--force` is transactional.

## Exit Readiness

The plan records all score inputs:

```text
C = completed catalog families / catalog families
T = (2 × exact + approximate) / (2 × resources)
V = passed critical validation checks / critical validation checks
score = round(100 × C × (0.9 × T + 0.1 × V))
```

Manual and unsupported resources contribute zero translation points. A critical validation failure caps the score at 49 and blocks export. The score measures reviewable migration coverage, not production readiness.
