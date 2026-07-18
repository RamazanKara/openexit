# Datadog to Grafana LGTM Example

This example uses a small redacted Datadog fixture and runs the focused v0.1 workflow.

Run from the repository root:

```bash
make example VERSION=0.1.0-dev
```

This writes ignored local state to `.openexit/` and a reviewable directory to `migration/`. Open `migration/index.html` to inspect the inventory, conversion ledger, semantic changes, generated candidates, and exit-readiness score.

The equivalent commands are:

```bash
openexit datadog scan --fixture input/datadog-fixture.json
openexit datadog plan --target grafana-lgtm
openexit datadog export --out migration/
```

The checked-in `output/` directory is a snapshot from the earlier experimental multi-stage engine. It remains only for historical compatibility and is not the v0.1 output contract.

Generated files are candidates only. Review every dashboard, alert rule, Alloy/OpenTelemetry baseline, and manual ledger item before operational use.
