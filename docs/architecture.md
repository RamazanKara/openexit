# Architecture

The primary OpenExit v0.1 pipeline is intentionally small:

```text
Datadog GET-only scan
        ↓
versioned inventory + redacted evidence
        ↓
deterministic conversion ledger
        ↓
Grafana / Prometheus / Alloy / OpenTelemetry candidates
        ↓
schema + provenance + safety validation
        ↓
static HTML report + transactional directory export
```

The engine is written in Go and keeps state under `.openexit/`. Inventory and plan identities are content-derived SHA-256 digests. The inventory timestamp is the scan time; planning reuses that timestamp instead of introducing nondeterministic wall-clock output.

## Boundaries

- The Datadog client exposes GET only and rejects pagination URLs that change scheme or host.
- Scan output is staged before inventory/evidence replacement. A new scan invalidates stale planned output.
- Converters accept only explicitly recognized syntax. Unsupported behavior produces a manual ledger entry, not a guessed executable file.
- Generated target files carry source references and candidate safety metadata.
- Validation checks source-to-output coverage in both directions: every inventory resource has one decision, and every generated file is linked from the plan or declared as a target baseline.
- Export revalidates current disk state, stages a fixed payload, writes per-file provenance/digests, and installs the directory atomically.

Public Draft 7 JSON Schemas are embedded in release binaries, so inventory, plan, validation, and bundle manifests can be validated without a repository checkout.

## Experimental Engine

The repository retains the earlier multi-provider collect/normalize/analyze/map/generate/validate/export engine under `openexit experimental`. It supports GitHub Enterprise, identity, edge, AI-provider, and legacy Datadog assessment paths. That engine is maintained for compatibility but is not part of the v0.1 product boundary.
