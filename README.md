# OpenExit

**Generate a reviewable, read-only migration plan from Datadog to Grafana, Prometheus, and OpenTelemetry.**

OpenExit v0.1 does one job: inventory a Datadog organization, translate the deterministic subset, and produce a static migration report that shows exactly what changed and what still needs human work.

It does not deploy anything. It does not write to Datadog. It does not use AI to guess conversions.

## Workflow

```bash
openexit datadog scan
openexit datadog plan --target grafana-lgtm
openexit datadog export --out migration/
```

Open `migration/index.html` in any browser. The report is self-contained and can be attached to an issue, reviewed in a pull request artifact, or shared with an observability team without running OpenExit.

## What You Get

```text
migration/
├── index.html                              # static migration report
├── README.md                               # reviewer handoff
├── inventory/datadog.inventory.json        # catalog/resource inventory and coverage
├── plan/openexit.plan.json                 # conversion ledger and readiness score
├── generated/
│   ├── grafana/dashboards/*.json           # reviewable Grafana candidates
│   ├── prometheus/rules/*.yaml             # safe-subset alert candidates
│   ├── alloy/config.alloy                   # credential-free Alloy baseline
│   └── opentelemetry/collector.yaml         # credential-free OTel baseline
├── evidence/datadog/**                     # redacted source evidence
├── validation/validation.json               # machine-readable validation results
├── manifest.json                            # file digests and source-resource links
└── SHA256SUMS                               # bundle integrity checks
```

Every Datadog resource gets one conversion status:

| Status | Meaning |
| --- | --- |
| `exact` | The represented behavior is preserved without a known semantic change. |
| `approximate` | OpenExit emitted a candidate and documented semantics that must be reviewed. |
| `manual` | The resource is inventoried, but no executable guess is emitted for the unsafe part. |
| `unsupported` | The capability is outside the Grafana LGTM v0.1 target. |

Complex anomaly, outlier, forecast, composite, formula, and unsupported query behavior remains manual. OpenExit never hides missing work behind `vector(0)` or another fake executable result.

## Why the Output Is Reviewable

- Each source-derived generated file links back to one or more stable `datadog:<kind>:<id>` source references; the two credential-free telemetry baselines are explicitly identified as target baselines when no source configuration applies.
- Each source reference links to redacted local evidence and, where possible, the Datadog UI.
- Dashboard conversion is recorded per widget and per query, including mixed converted/manual widgets.
- Alert candidates preserve the source query and carry `openexit_candidate=true` and `production_ready=false`.
- Semantic changes, reason codes, and manual review instructions are part of the machine-readable plan.
- Inventory, plan, validation, and export manifests are checked against embedded JSON Schemas.
- Export recomputes validation and refuses stale plans, failed checks, unsafe paths, symlinks, or secret-like output.

## Datadog Inventory

The live scanner uses GET requests only. Its versioned catalog evaluates:

- dashboards, dashboard lists, and powerpacks;
- monitors, monitor policies, and downtimes;
- SLOs and SLO corrections;
- notebooks and Synthetic Monitoring resources;
- active metric metadata;
- log pipelines, pipeline order, indexes, archives, and log-based metrics;
- APM retention filters and span-based metrics;
- service definitions;
- installed integrations and AWS, Azure, and GCP integration accounts.

The inventory records status, resource count, and any error for every endpoint. A `401`, `403`, decode failure, detail failure, or interrupted page makes the scan incomplete and the command fails closed. Use `--allow-partial` only when you intentionally want that limitation carried into the plan and readiness score.

## Exit Readiness

The report publishes the score inputs and formula:

```text
score = round(100 × C × (0.9 × T + 0.1 × V))
```

- `C` is completed inventory families divided by catalog families. Endpoint states `complete`, `empty`, and `not_available` count as evaluated; `partial`, `permission_denied`, and `error` do not.
- `T` is `(2 × exact + approximate) / (2 × inventoried resources)`. Manual and unsupported resources contribute zero.
- `V` is passed critical validation checks divided by critical checks.
- Any critical validation failure caps the score at 49 and blocks export.

The score is migration-plan coverage, not cutover approval or production readiness.

## Install

Build from source with Go 1.25 or newer:

```bash
git clone https://github.com/RamazanKara/openexit.git
cd openexit
make build
./bin/openexit version
```

Release binaries can also be installed with:

```bash
curl -fsSL https://github.com/RamazanKara/openexit/releases/latest/download/install.sh | sh
openexit doctor
```

The installer verifies the selected binary against `SHA256SUMS` and `RELEASE_MANIFEST.json` before installation.

## Scan Datadog

Put credentials in environment variables; OpenExit reads them at runtime and never stores them:

```bash
export DATADOG_API_KEY='<read-only-api-key>'
export DATADOG_APP_KEY='<read-only-application-key>'

openexit datadog scan --site datadoghq.eu
```

The application key needs read access to every catalog family you want declared complete. Alternate variable names are supported:

```bash
openexit datadog scan \
  --api-key-env MY_DD_API_KEY \
  --app-key-env MY_DD_APP_KEY
```

State is written to `.openexit/` by default. Use `--workdir` on all three commands to select another location.

## Try It Without Datadog Credentials

```bash
make build
./bin/openexit datadog scan \
  --fixture testdata/datadog/small.json \
  --workdir /tmp/openexit-demo/.openexit
./bin/openexit datadog plan \
  --target grafana-lgtm \
  --workdir /tmp/openexit-demo/.openexit
./bin/openexit datadog export \
  --out /tmp/openexit-demo/migration \
  --workdir /tmp/openexit-demo/.openexit
```

## Safety Contract

- Datadog access is read-only and implemented through a GET-only client.
- API and application keys are sent only in Datadog request headers.
- Error messages never include response bodies or credentials.
- Evidence is structurally redacted before it is written.
- Generated files are review candidates and contain no source credentials.
- Planning and export are local, deterministic operations.
- There is no automatic deployment, cutover, source deletion, hosted backend, or AI conversion in v0.1.

See [the Datadog migration details](docs/datadog-to-grafana.md), [security model](docs/security.md), and [public schemas](docs/schemas.md).

## Experimental Providers

The earlier multi-provider assessment engine remains in the repository for experimentation:

- GitHub Enterprise → Forgejo
- Okta/Auth0 → Keycloak/Zitadel
- Cloudflare/Akamai → Varnish/HAProxy/Coraza
- OpenAI/Anthropic → vLLM/LiteLLM

Discover it with:

```bash
openexit experimental --help
```

Legacy root command aliases remain hidden for compatibility. Experimental providers are not part of the v0.1 product contract and will not be promoted until the Datadog path has real users.

## Development

```bash
make test
make verify
```

`make verify` runs formatting, static analysis, unit and end-to-end tests, a Datadog scan/plan/export smoke test, and compatibility tests for the experimental engine.

## License

Apache-2.0
