# Security

OpenExit v0.1 is a local, read-only Datadog migration planner.

## Source Safety

- The primary Datadog client implements GET requests only.
- API and application keys are read from named environment variables and sent only as `DD-API-KEY` and `DD-APPLICATION-KEY` request headers.
- Credentials are never written to inventory, evidence, reports, logs, or generated configuration.
- API errors include the method, request path, and status code, but never include response bodies.
- Pagination is accepted only when it remains on the configured Datadog scheme and host.
- OpenExit performs no Datadog create, update, delete, mute, deploy, or cutover operation.

## Local Evidence

API objects are structurally redacted before they are persisted. Credential-bearing keys, secret-variable values, bearer tokens, Datadog-key-like values, and private-key material are replaced with `[REDACTED]`. Secret-variable identity fields remain intact so resources do not collapse into an unusable anonymous record.

Each evidence file receives a SHA-256 digest recorded in the inventory. Planning and export recompute those digests and fail if evidence was changed.

Treat evidence as potentially sensitive operational metadata even after credential redaction. Store and share exported migration directories according to your organization’s observability-data policy.

## Generated Candidate Safety

- Grafana dashboards carry `productionReady: false` in OpenExit metadata.
- Prometheus rules carry `openexit_candidate=true` and `production_ready=false` and preserve the source query.
- Unsupported monitor behavior emits no executable rule; fake placeholders such as `vector(0)` are rejected.
- Alloy and OpenTelemetry files contain environment placeholders, not source credentials.
- There is no AI conversion, hosted backend, or automatic deployment in the v0.1 path.

## Validation and Export

Validation checks evidence integrity, plan identity, conversion coverage, generated-file provenance, candidate safety metadata, YAML/JSON shape, embedded JSON Schemas, report links, secret-like values, path traversal, and symlinks.

Export reruns validation against current disk state and copies only these fixed sections: inventory, evidence, plan, generated candidates, validation, `index.html`, and `README.md`. It refuses symlinks and unsafe output targets. Existing export directories require explicit `--force` and are replaced transactionally.

The exported `manifest.json` records size, SHA-256, and source references for payload files. `SHA256SUMS` covers the payload and manifest.

## Experimental Providers

The legacy multi-provider engine has separate provider-specific security behavior documented in its collectors and tests. It is exposed under `openexit experimental` and is outside the primary v0.1 security contract.
