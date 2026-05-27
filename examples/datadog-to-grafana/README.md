# Datadog To Grafana Example

This example uses a small redacted Datadog-like fixture and writes all generated output to a local project directory.

Run from the repository root:

```bash
make example VERSION=0.1.0-dev
```

This refreshes `examples/datadog-to-grafana/output/` and writes an ignored bundle to `examples/datadog-to-grafana/openexit-example.zip`.

The generated files are candidates only. Review every dashboard, alert rule, collector sketch, migration-plan phase gate, and runbook before operational use.
