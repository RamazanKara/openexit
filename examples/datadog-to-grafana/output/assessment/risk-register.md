# Risk Register

- Project: output
- Source: datadog
- Target: grafana-lgtm
- Generated: 2026-05-27T04:39:07Z
- Complexity: high (80)

## Assumptions
- OpenExit is local-first and performs no production writes.
- Generated target files are candidates and require human review.
- Evidence references point to redacted local source snapshots.

## Risks
### dd.cost.log-volume-unknown.001
- Severity: low
- Title: Log volume is unknown
- Affected assets: inventory:logs
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Capture daily ingest, retention, label strategy, and query expectations before target sizing.

### dd.cost.retention-unknown.001
- Severity: low
- Title: Retention requirements are not captured
- Affected assets: inventory:retention
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Document retention and volume assumptions before sizing Grafana LGTM components.

### dd.cost.trace-volume-unknown.001
- Severity: low
- Title: Trace volume is unknown
- Affected assets: inventory:traces
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Capture spans per second, retention, sampling policy, and search expectations before target sizing.

### dd.dashboard.unsupported-widget.001
- Severity: medium
- Title: Unsupported dashboard widgets found
- Affected assets: dashboard:abc-123
- Evidence: evidence://datadog/dashboard/abc-123
- Recommendation: Generate a Grafana dashboard candidate and manually review unsupported widgets before parity sign-off.

### dd.monitor.anomaly-detection.001
- Severity: high
- Title: Anomaly detection monitor needs manual replacement
- Affected assets: monitor:789012
- Evidence: evidence://datadog/monitor/789012
- Recommendation: Design an explicit Prometheus/Grafana alerting strategy and shadow it before cutover.

### dd.monitor.manual-routing-needed.001
- Severity: medium
- Title: Manual alert routing review is needed
- Affected assets: monitor:789012
- Evidence: evidence://datadog/monitor/789012
- Recommendation: Create explicit target contact points and routing policies, then test them without production paging.

### dd.monitor.missing-owner.001
- Severity: medium
- Title: Monitor ownership is unclear
- Affected assets: monitor:789012
- Evidence: evidence://datadog/monitor/789012
- Recommendation: Assign an alert owner before migrating routing or paging behavior.

### dd.monitor.missing-runbook.001
- Severity: low
- Title: Monitor has no runbook link
- Affected assets: monitor:789012
- Evidence: evidence://datadog/monitor/789012
- Recommendation: Add or confirm operational guidance before alert cutover.

### dd.monitor.query.translation.001
- Severity: high
- Title: Datadog-specific monitor query requires manual conversion
- Affected assets: monitor:123456
- Evidence: evidence://datadog/monitor/123456
- Recommendation: Create a PromQL candidate and require SRE review before alert shadowing.

### dd.monitor.unknown-notification-target.001
- Severity: medium
- Title: Notification target requires routing review
- Affected assets: monitor:789012
- Evidence: evidence://datadog/monitor/789012
- Recommendation: Map notification routing manually and test paging behavior during shadowing.

### dd.slo.burn-rate-alert-missing.001
- Severity: medium
- Title: SLO burn-rate alert mapping is missing
- Affected assets: slo:slo-123
- Evidence: evidence://datadog/slo/slo-123
- Recommendation: Create and shadow target burn-rate alerts before SLO cutover.

### dd.slo.dashboard-missing.001
- Severity: low
- Title: SLO dashboard mapping is missing
- Affected assets: slo:slo-123
- Evidence: evidence://datadog/slo/slo-123
- Recommendation: Add or confirm target SLO dashboard coverage for migration readiness.

### dd.slo.target-needs-review.001
- Severity: medium
- Title: SLO target needs migration review
- Affected assets: slo:slo-123
- Evidence: evidence://datadog/slo/slo-123
- Recommendation: Review SLO semantics and create matching dashboards and burn-rate alerts before cutover.

### dd.slo.without-clear-sli.001
- Severity: medium
- Title: SLO has no clear SLI mapping
- Affected assets: slo:slo-123
- Evidence: evidence://datadog/slo/slo-123
- Recommendation: Identify the target SLI query and validate it against the source SLO before migration.

### migration.alert-shadowing-required.001
- Severity: medium
- Title: Alert shadowing is required
- Affected assets: inventory:monitors
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Run alert candidates without production paging and compare behavior against Datadog over a representative period.

### migration.dashboard-parity-required.001
- Severity: medium
- Title: Dashboard parity review is required
- Affected assets: inventory:dashboards
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Review candidate dashboards with service owners and record acceptable differences.

### migration.manual-query-review-required.001
- Severity: high
- Title: Manual query review is required
- Affected assets: inventory:queries
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Review query candidates with SRE owners and shadow target alert behavior before cutover.

### migration.notification-routing-review-required.001
- Severity: medium
- Title: Notification routing review is required
- Affected assets: inventory:notification-targets
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Create target routing policies and test them before any production paging change.

### migration.oncall-ownership-unclear.001
- Severity: medium
- Title: On-call ownership is unclear
- Affected assets: inventory:ownership
- Evidence: evidence://datadog/dashboard/abc-123, evidence://datadog/monitor/123456, evidence://datadog/monitor/789012
- Recommendation: Assign owners before alert routing, dashboard parity review, and cutover planning.

## Generated by OpenExit
This artifact was generated deterministically from local OpenExit inventory and assessment manifests. It is not production-ready until reviewed.
