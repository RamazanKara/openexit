# Security

OpenExit is designed for local-first assessment work.

- Collectors must not make production writes.
- Live Datadog, GitHub, Okta, Auth0, Cloudflare, and OpenAI credentials are read from environment variables and are never written to project files.
- The GitHub collector records secret names and consumers only; it never requests or stores secret values.
- The Okta collector records client and policy metadata only; it never requests or stores client secrets, passwords, factor secrets, or token values.
- The Auth0 collector records client, role, action/rule, Guardian MFA, and explicit break-glass user metadata only; it does not persist client secrets, action code, rule scripts, user passwords, MFA secrets, or token values.
- The Cloudflare collector records zone configuration metadata only; it never mutates DNS, rulesets, page rules, or zone settings.
- The OpenAI collector records aggregate usage and model metadata only; it never requests or stores raw prompts, completions, or API key values.
- Raw source evidence is redacted before it is written locally.
- Validation scans JSON, YAML, Markdown, text, VCL, HAProxy, and Coraza-style generated artifacts for secret-like values.
- Exported bundles contain local manifests, generated candidates, validation output, checksums, and redacted evidence.
- AI assist is optional, disabled by default, and never part of deterministic validation or export.

Generated configs are candidates only. Review them before any operational use.
