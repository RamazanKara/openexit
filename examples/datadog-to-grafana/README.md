# Datadog To Grafana Example

This example uses a small redacted Datadog-like fixture and writes all generated output to a local project directory.

Run from the repository root:

```bash
make build VERSION=0.1.0-dev
./bin/openexit init ./demo
./bin/openexit collect fixture --project ./demo --input ./examples/datadog-to-grafana/input/datadog-fixture.json
./bin/openexit assess --project ./demo --target grafana-lgtm
./bin/openexit generate --project ./demo --all
./bin/openexit validate --project ./demo
./bin/openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

The generated files are candidates only. Review every dashboard, alert rule, collector sketch, and runbook before operational use.
