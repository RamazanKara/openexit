# Release Readiness

This checklist prepares OpenExit for its first public release.

## Scope Audit

The implementation plan names Datadog to Grafana LGTM, Prometheus-compatible alerting, and OpenTelemetry Collector/Alloy as the v0.1 primary release path.

Release-blocking v0.1 requirements:

- CLI skeleton, project init/status, version command.
- Datadog fixture collector and read-only live Datadog collector.
- Inventory and assessment manifests with typed validation.
- Risk and manual-review analyzers.
- Markdown handover artifacts.
- Grafana, Prometheus, OpenTelemetry, and ArgoCD candidate generators.
- Validation engine with YAML/JSON parsing, evidence refs, secret scan, optional promtool, and optional kubeconform.
- Evidence bundle export with checksums and OpenExit version metadata.
- No-op AI assist and optional external assist behind explicit opt-in.
- Documentation, examples, CI, release draft workflow, and reproducible release artifacts.

Preview paths present in this repository:

- GitHub Enterprise to Forgejo.
- Okta/Auth0 to Keycloak/Zitadel.
- Cloudflare/Akamai to Varnish/HAProxy/Coraza.

These previews are fixture-first only and are not part of the stable v0.1 Datadog guarantee.

## Release Checklist

- [ ] `git status --short --branch` is clean and on the intended release branch.
- [ ] `make verify VERSION=0.1.0` passes.
- [ ] `make lint` runs `gofmt`, `golangci-lint`, and `go vet`.
- [ ] `make release-dist VERSION=0.1.0` produces binaries and `dist/SHA256SUMS`.
- [ ] Datadog definition-of-done pipeline passes:
  `init`, `collect fixture`, `assess`, `generate --all`, `validate`, `export`.
- [ ] Preview fixture pipelines validate for GitHub, identity, and edge paths.
- [ ] `openexit version` prints name, version, commit, and date from release build flags.
- [ ] `README.md`, `docs/cli.md`, `docs/security.md`, and this checklist reflect current behavior.
- [ ] `examples/datadog-to-grafana/README.md` reproduces the primary local demo.
- [ ] `CHANGELOG.md` has a `0.1.0` section.
- [ ] Exported bundle README includes version, commit, build date, bundle timestamp, and candidate warning.
- [ ] No credentials, tokens, passwords, or private keys are present in fixtures, generated files, docs, or bundles.
- [ ] Draft release notes have been reviewed.

## Draft Release Body

OpenExit `v0.1.0` is the first public release of the local-first SaaS-to-open-source migration assessment CLI.

Primary supported path:

- Datadog to Grafana LGTM, Prometheus-compatible alerting, and OpenTelemetry Collector/Alloy candidate artifacts.

Fixture-first previews:

- GitHub Enterprise to Forgejo.
- Okta/Auth0 to Keycloak/Zitadel.
- Cloudflare/Akamai to Varnish/HAProxy/Coraza.

Safety model:

- No production writes.
- No credential storage.
- No hidden hosted backend.
- No AI dependency.
- Generated configs are candidates and require human review.

Verification before publishing:

```bash
make verify VERSION=0.1.0
make release-dist VERSION=0.1.0
```
