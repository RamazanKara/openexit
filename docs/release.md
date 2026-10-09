# Release Readiness

## v0.1 Product Contract

The release-blocking workflow is:

```bash
openexit datadog scan
openexit datadog plan --target grafana-lgtm
openexit datadog export --out migration/
```

Release scope includes:

- GET-only, paginated Datadog observability inventory with endpoint-level coverage and redacted evidence;
- deterministic `exact`, `approximate`, `manual`, and `unsupported` decisions for every inventoried resource;
- source-linked Grafana dashboards and safe-subset Prometheus alert candidates;
- source-linked, credential-free Alloy and OpenTelemetry baselines;
- explicit semantic changes and no fake executable placeholders;
- static self-contained HTML report and transparent exit-readiness score;
- embedded inventory, plan, validation, and bundle JSON Schemas;
- stale-plan, provenance, candidate-safety, report-link, path, symlink, and secret validation;
- transactional directory export with `manifest.json` and `SHA256SUMS`;
- no AI conversion, source mutation, automatic deployment, or cutover.

GitHub Enterprise, identity, edge, AI-provider, and legacy project workflows remain experimental and are not release claims for the primary v0.1 product.

## Checklist

- [ ] `git status --short --branch` contains only intended release changes.
- [ ] `make verify VERSION=0.1.0` passes formatting, static analysis, all tests, the primary Datadog smoke test, and experimental compatibility smoke tests.
- [ ] `make example VERSION=0.1.0` produces `examples/datadog-to-grafana/migration/index.html`, generated candidates, `manifest.json`, and `SHA256SUMS`.
- [ ] A live least-privilege Datadog account completes every expected endpoint, or any unavailable endpoint is understood and visible in the report.
- [ ] A permission-denied endpoint fails `scan` without `--allow-partial` and remains visible in inventory endpoint coverage.
- [ ] Repeating fixture scan/plan with a fixed timestamp produces byte-identical inventory, plan, candidates, validation, and HTML.
- [ ] Every inventory resource has exactly one plan decision and resolvable evidence.
- [ ] Every non-baseline generated file has one or more source references.
- [ ] Complex monitors emit no Prometheus rule and no generated file contains `vector(0)`.
- [ ] Grafana and Prometheus candidates remain marked non-production-ready.
- [ ] Tampered evidence or generated output blocks export.
- [ ] Export rejects stale plans, symlinks, unsafe paths, failed critical validation, and an existing destination without `--force`.
- [ ] Exported report links resolve offline and `SHA256SUMS` matches every listed file.
- [ ] `openexit doctor` compiles every embedded schema.
- [ ] `make release-dist VERSION=0.1.0` produces platform binaries, installer, completions, SBOM, `RELEASE_MANIFEST.json`, and `SHA256SUMS`.
- [ ] `openexit verify-release dist/RELEASE_MANIFEST.json --dist dist --require-checksums` passes and detects tampering.
- [ ] The installer selects and verifies the current platform binary.
- [ ] README, Datadog details, CLI, security, schemas, changelog, and example instructions match current behavior.
- [ ] No credentials, private keys, tokens, or customer data are present in repository fixtures or generated release assets.

## Release Commands

Run `make verify` locally with Go, GNU Make, and a POSIX shell (Git Bash on Windows). Race tests run only when cgo is enabled; otherwise the gate reports the skip. The single push/manual CI workflow uses this same target. GitHub Actions availability is not required for local verification. The shell installer and `make install-smoke` support Linux and macOS; they do not run on native Windows.

```bash
make verify VERSION=0.1.0
make release-check VERSION=0.1.0
openexit verify-release dist/RELEASE_MANIFEST.json --dist dist --require-checksums
```

## Local Builds

Prepare release artifacts locally without publishing anything:

```bash
make verify
make release-dist VERSION=0.1.0-dev
./bin/openexit verify-release dist/RELEASE_MANIFEST.json --dist dist --require-checksums
(cd dist && sha256sum -c SHA256SUMS)
```

On Windows with Git Bash, the verifier binary is `./bin/openexit.exe`. `release-dist` replaces `dist/` and builds Linux, macOS, and Windows binaries plus installer, completions, SBOM, manifest, and checksums. Set the existing `VERSION`, `COMMIT`, and `DATE` Make variables to fixed values when comparing repeat builds. Run `make release-check` on Linux or macOS to include the installer smoke test.

For a single Windows amd64 binary from native PowerShell:

```powershell
$releaseVersion = '0.1.0-dev'
$releaseCommit = git rev-parse --short HEAD
$releaseDate = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$versionPackage = 'github.com/RamazanKara/openexit/internal/version'
$stamp = "-s -w -X $versionPackage.Version=$releaseVersion -X $versionPackage.Commit=$releaseCommit -X $versionPackage.Date=$releaseDate"
$artifact = "openexit_${releaseVersion}_windows_amd64.exe"
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags $stamp -o "dist/$artifact" ./cmd/openexit
& "./dist/$artifact" release-manifest --dist dist --platform windows/amd64 --out dist/RELEASE_MANIFEST.json
$hash = (Get-FileHash "dist/$artifact" -Algorithm SHA256).Hash.ToLowerInvariant()
"$hash  $artifact" | Set-Content dist/SHA256SUMS -Encoding ascii
& "./dist/$artifact" verify-release dist/RELEASE_MANIFEST.json --dist dist --require-checksums
```

These commands create local files only. They do not tag, commit, push, or publish a release.

## Draft Release Summary

OpenExit v0.1 generates a deterministic, read-only Datadog-to-Grafana-LGTM migration plan. It inventories the Datadog observability control plane, emits only the safely recognized Grafana and Prometheus subset, creates review baselines for Alloy and OpenTelemetry, and packages a source-linked static report. Anything uncertain is explicit manual work. OpenExit does not use AI conversion, mutate Datadog, or deploy target configuration.
