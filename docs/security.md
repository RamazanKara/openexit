# Security

OpenExit is designed for local-first assessment work.

- Collectors must not make production writes.
- Live Datadog and GitHub credentials are read from environment variables and are never written to project files.
- The GitHub collector records secret names and consumers only; it never requests or stores secret values.
- Raw source evidence is redacted before it is written locally.
- Validation scans JSON, YAML, Markdown, text, VCL, HAProxy, and Coraza-style generated artifacts for secret-like values.
- Exported bundles contain local manifests, generated candidates, validation output, checksums, and redacted evidence.
- AI assist is optional, disabled by default, and never part of deterministic validation or export.

Generated configs are candidates only. Review them before any operational use.
