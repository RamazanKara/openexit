# CLI

## Primary v0.1 Workflow

```bash
openexit datadog scan [flags]
openexit datadog plan --target grafana-lgtm [flags]
openexit datadog export --out migration/ [flags]
```

### `datadog scan`

Inventories the versioned Datadog observability catalog through a GET-only client and writes redacted evidence under `.openexit/`.

```text
--workdir string       state directory (default ".openexit")
--site string          Datadog site (default "datadoghq.com")
--api-key-env string   API-key environment variable (default "DATADOG_API_KEY")
--app-key-env string   application-key environment variable (default "DATADOG_APP_KEY")
--fixture string       local fixture instead of the live API
--allow-partial        accept explicitly incomplete endpoint coverage
```

Without `--allow-partial`, an incomplete scan is still persisted for diagnosis but the command exits non-zero. Every newly persisted scan—including a fail-closed partial scan—invalidates the previous plan, generated files, validation, and report.

### `datadog plan`

Reads the current inventory and emits deterministic Grafana, Prometheus, Alloy, and OpenTelemetry candidates, a machine-readable conversion ledger, validation results, and `index.html`.

```text
--workdir string   state directory (default ".openexit")
--target string    target (default and only v0.1 value: "grafana-lgtm")
--allow-partial    plan from an explicitly accepted partial inventory
```

The command exits non-zero if a critical validation check fails. No deployment or source write is performed.

### `datadog export`

Reruns validation and copies the fixed migration payload into a review directory.

```text
--out string       required output directory
--workdir string   state directory (default ".openexit")
--force            transactionally replace an existing output directory
--allow-partial    export an explicitly accepted partial plan
```

The export includes a schema-backed `manifest.json` and `SHA256SUMS`. It rejects stale plans, critical validation failures, unsafe paths, symlinks, and secret-like content.

### JSON output

Add `--json` to `datadog scan`, `plan`, or `export` to emit exactly the persisted inventory, migration plan, or bundle manifest as one JSON document on stdout. Existing text output is the default. The JSON documents use the existing [public schemas](schemas.md).

```bash
openexit datadog scan --fixture testdata/datadog/small.json --json > inventory.json
openexit datadog plan --json > plan.json
openexit datadog export --out migration/ --json > bundle.json
```

A persisted partial scan or failed plan is still emitted with a nonzero exit status. Failures before a result exists leave stdout empty; diagnostics go to stderr. Check the exit status before treating output as successful. Keep redirected files outside the work and export directories.

### `datadog explain <source-ref>`

Shows the recorded status, reason codes, semantic changes, widget/query decisions, source and target queries, review guidance, evidence path, and generated outputs for one resource. Use the full `sourceRef` from the plan, such as `datadog:monitor:123456`.

```bash
openexit datadog explain datadog:monitor:123456
openexit datadog explain datadog:dashboard:abc-123 --json
openexit datadog explain datadog:monitor:123456 --workdir migration/
```

`--workdir` defaults to `.openexit` and also accepts an exported migration directory. `--json` emits the resource's conversion record. This command is local and read-only; it refuses missing, malformed, or stale plans and reports unknown references. It explains the saved decision without rerunning conversion or approving the generated candidates.

## Runtime and Release Utilities

- `openexit version`
- `openexit doctor [--json] [--strict]`
- `openexit completion bash|zsh|fish|powershell`
- `openexit sbom [--out SBOM.cdx.json]`
- `openexit verify-bundle <migration-directory|legacy-bundle.zip> [--json]`
- `openexit release-manifest [flags]`
- `openexit verify-release <manifest.json> [flags]`

`doctor` verifies build metadata, embedded schema compilation, and optional local validators.

`verify-bundle migration/` checks an exported directory offline without changing files. It validates `manifest.json` against the embedded migration-bundle schema, checks file sizes and SHA-256 digests, and requires complete `SHA256SUMS` coverage including the manifest. Missing or extra payload files, duplicate entries, unsafe paths, symlinks, and malformed checksums fail verification. Checksum syntax errors include line numbers; LF and CRLF checksum files are accepted. JSON reports include `status`, `errors`, `manifestFiles`, `checksumEntries`, and `directoryFiles`; failures retain a nonzero exit. Legacy zip verification keeps its existing behavior.

Checksums establish integrity relative to the supplied manifest, not publisher authenticity. Verification does not rerun migration validation or certify production readiness.

## Experimental Multi-provider Engine

The previous project-oriented commands are grouped under:

```bash
openexit experimental --help
```

This includes `init`, `demo`, `status`, `run`, `collect`, `assess`, `map`, `generate`, `validate`, `export`, and optional `assist` commands for the GitHub Enterprise, identity, edge, AI-provider, and legacy Datadog assessment paths.

Hidden root aliases remain executable for backward compatibility, but they are not part of the focused Datadog v0.1 interface.
