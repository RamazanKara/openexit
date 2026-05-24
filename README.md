# OpenExit

Local-first migration assessments from proprietary SaaS platforms to open-source infrastructure.

OpenExit v0.1 focuses on Datadog to Grafana LGTM, Prometheus-compatible alerting, and OpenTelemetry Collector or Grafana Alloy. It collects inventory, normalizes it, analyzes migration risks, generates candidate target files, validates outputs, and exports a local evidence bundle.

## Safety Model

- No production writes.
- No one-click migration.
- No hidden hosted backend.
- No credential storage.
- No direct SaaS deletion.
- No AI dependency.
- No generated config is production-ready without review.

## Quick Start

```bash
make build
./bin/openexit init ./demo
./bin/openexit collect fixture --project ./demo --input ./testdata/datadog/small.json
./bin/openexit assess --project ./demo --target grafana-lgtm
./bin/openexit generate --project ./demo --all
./bin/openexit validate --project ./demo
./bin/openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

## Commands

- `openexit version`
- `openexit init <project-dir>`
- `openexit status --project <project-dir>`
- `openexit collect fixture --project <project-dir> --input <file>`
- `openexit collect datadog --project <project-dir> --site datadoghq.eu --api-key-env DATADOG_API_KEY --app-key-env DATADOG_APP_KEY`
- `openexit assess --project <project-dir> --target grafana-lgtm`
- `openexit generate --project <project-dir> --all`
- `openexit validate --project <project-dir>`
- `openexit export --project <project-dir> --format zip --out <file>`
- `openexit assist summarize --project <project-dir> --provider noop`

The Datadog collector is read-only. API keys are read from environment variables, are not printed, and are not stored.

## Current Scope

Included in this first implementation:

- CLI skeleton and project init/status.
- Typed project, inventory, assessment, mapping, and validation manifests.
- Fixture-based Datadog inventory import.
- Read-only Datadog collection for dashboards, monitors, and SLOs.
- Deterministic risk assessment.
- Markdown handover artifacts.
- Grafana dashboard candidate JSON.
- Prometheus alert rule candidate YAML.
- OpenTelemetry Collector sketch.
- ArgoCD starter manifest.
- Validation report with YAML/JSON parsing, evidence ref checks, secret scan, and optional `promtool`/`kubeconform` checks.
- Evidence bundle export.
- No-op assist provider.

Not included in v0.1:

- Automatic cutover.
- Production apply.
- Hosted portal.
- Perfect Datadog to Grafana parity.
- AI-required decision making.

## License

Apache-2.0.
