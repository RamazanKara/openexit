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

## Runtime and Release Utilities

- `openexit version`
- `openexit doctor [--json] [--strict]`
- `openexit completion bash|zsh|fish|powershell`
- `openexit sbom [--out SBOM.cdx.json]`
- `openexit verify-bundle <legacy-bundle.zip> [--json]`
- `openexit release-manifest [flags]`
- `openexit verify-release <manifest.json> [flags]`

`doctor` verifies build metadata, embedded schema compilation, and optional local validators. Release commands and legacy zip verification are retained for distribution compatibility.

## Experimental Multi-provider Engine

The previous project-oriented commands are grouped under:

```bash
openexit experimental --help
```

This includes `init`, `demo`, `status`, `run`, `collect`, `assess`, `map`, `generate`, `validate`, `export`, and optional `assist` commands for the GitHub Enterprise, identity, edge, AI-provider, and legacy Datadog assessment paths.

Hidden root aliases remain executable for backward compatibility, but they are not part of the focused Datadog v0.1 interface.
