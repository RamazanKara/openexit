# Contributing

Thanks for helping make OpenExit useful and trustworthy.

## Development

```bash
make test
make lint
make build
make release-dist VERSION=0.1.0-dev
```

Keep changes local-first and deterministic. The primary Datadog client must remain GET-only, must never store credentials, and must redact source evidence before it is written to disk.

## Design Principles

- Prefer explicit risk and manual-review flags over optimistic conversion.
- Keep AI conversion and automatic deployment out of the primary v0.1 workflow.
- Add tests for catalog endpoints, pagination, redaction, converters, provenance, scoring, and generated artifacts.
- Generated target files must be labeled as candidates.
- Never emit executable placeholders for behavior that was not translated.

## Pull Requests

Include:

- What changed.
- Why it matters.
- Validation performed.
- Any known limitations or manual-review areas.

Before a release-oriented PR, run `make verify` and check `docs/release.md`.
