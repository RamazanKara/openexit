# Contributing

Thanks for helping make OpenExit useful and trustworthy.

## Development

```bash
make test
make lint
make build
```

Keep changes local-first and deterministic. New collectors must default to read-only behavior, must never store credentials, and must redact raw evidence before it is written to disk.

## Design Principles

- Prefer explicit risk and manual-review flags over optimistic conversion.
- Keep AI optional and outside the source of truth.
- Add tests for new analyzer rules, collectors, and generated artifacts.
- Generated target files must be labeled as candidates.

## Pull Requests

Include:

- What changed.
- Why it matters.
- Validation performed.
- Any known limitations or manual-review areas.
