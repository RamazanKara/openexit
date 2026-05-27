# CLI

See `openexit --help` for the command tree.

`openexit init` accepts `--source` and `--target`. If `--target` is omitted, the CLI uses the standard target for the selected source.

Release builds stamp version metadata:

```bash
make build VERSION=0.1.0
./bin/openexit version
```

`openexit doctor` checks the local CLI runtime before a project run. It verifies version metadata, embedded schema compilation, and optional validator availability for `promtool` and `kubeconform`. Missing optional validators are warnings by default; pass `--strict` to make warnings fail, or `--json` for automation.

The minimum local demo is:

```bash
openexit demo ./demo
```

`openexit demo` uses built-in redacted fixture data, so release binaries can create a complete sample project without access to this repository's `testdata/` directory. It initializes the project, collects fixture inventory, runs assessment, mapping, full artifact generation, validation, final status reporting, and evidence bundle export. Pass `--source github-enterprise`, `--source identity`, `--source edge`, or `--source ai-provider` to try another built-in path.

The CI test suite also runs this definition-of-done pipeline against fixture inventory and checks that the generated project layout and export bundle contain the expected artifacts. Bundle checksums are verified against the archived file bytes.

The checked-in Datadog example can be refreshed with `make example VERSION=0.1.0-dev`. CI also runs `make example-smoke` as part of `make verify` to ensure the example fixture still completes the full pipeline.

`openexit run --project <project-dir>` is the ergonomic path after collection. It runs assessment, mapping, full artifact generation, validation, and final status reporting. Add `--export --out <file>` to write the evidence bundle after validation passes, or `--strict` to treat validation warnings as failures.

`openexit status --project <project-dir>` summarizes the current pipeline state: project layout, source/target pair, inventory counts, assessment finding severity, mapping counts, generated candidate artifacts, validation check totals, export readiness, and the next recommended command. Use `--json` to feed the same readiness data into automation or release gates.

`openexit validate` performs typed consistency checks, embedded JSON Schema validation, Grafana dashboard candidate validation, Prometheus alert-rule candidate validation, OpenTelemetry collector candidate validation, ArgoCD candidate validation, Forgejo migration candidate validation, identity realm/client candidate validation, edge VCL/HAProxy/Coraza candidate validation, LiteLLM/vLLM candidate validation, YAML/JSON parse checks, evidence reference checks, secret scanning, and optional external tool checks when `promtool` or `kubeconform` are installed.

`openexit export` refuses to package symlinks from exported project sections, even with `--force`, so evidence bundles cannot accidentally include files from outside the project tree. Exported zips include `openexit-evidence/manifest.json` with build metadata, project source/target, validation totals, and per-file SHA-256 digests, plus `checksums.txt` for archive-level verification. The manifest shape is published as `schemas/openexit.evidence-bundle.schema.json`.

`openexit verify-bundle <bundle.zip>` verifies an exported bundle without requiring the original project directory. It checks archive path safety, required bundle files, manifest schema validity, manifest file size/digest metadata, and `checksums.txt`. Use `--json` to feed the verification report into a handover gate.

`openexit release-manifest --dist dist --out dist/RELEASE_MANIFEST.json` writes a machine-readable manifest for release artifacts. It records the stamped release version, commit, build date, generation time, each expected OS/architecture binary, and auxiliary assets such as `install.sh` and shell completions with file name, type, relative path, size, and SHA-256 digest. The manifest shape is published as `schemas/openexit.release-manifest.schema.json`.

`openexit verify-release dist/RELEASE_MANIFEST.json --dist dist --require-checksums` verifies release artifacts after download or before publishing. It validates the manifest schema, rejects unsafe artifact paths, recomputes file sizes and SHA-256 digests, and verifies `SHA256SUMS` when required. Use repeatable `--artifact <name>` to verify only the current platform artifact after download, or `--json` to feed the verification report into a release gate.

`openexit completion bash|zsh|fish|powershell` prints shell completion scripts. Release artifacts also include `openexit.bash`, `_openexit`, `openexit.fish`, and `openexit.ps1` so package managers and manual installs can wire completion without rebuilding from source.

`openexit sbom --out SBOM.cdx.json` writes a CycloneDX JSON SBOM for the current OpenExit binary. It records the stamped OpenExit version, commit, build date, Go toolchain version, and Go module dependencies reported by the binary build info. Release builds include `SBOM.cdx.json` and cover it with both `RELEASE_MANIFEST.json` and `SHA256SUMS`.

Generate individual artifacts with `openexit generate --artifact <name>`. The primary Datadog path supports `mapping`, `assessment`, `risk-register`, `manual-review`, `cost-drivers`, `target-architecture`, `acceptance-criteria`, `rollback-plan`, `runbook`, `restore-drill-checklist`, `alert-shadowing-plan`, `migration-plan`, `grafana-dashboards`, `prometheus-rules`, `opentelemetry`, and `argocd`. The GitHub Enterprise path also supports `forgejo-migration-candidate`; the identity path also supports `realm-client-candidate`; the edge path also supports `vcl-candidates`, `haproxy-candidates`, and `coraza-rule-candidates`; the AI provider path also supports `litellm-config-candidate`.

The GitHub Enterprise to Forgejo fixture path uses local JSON metadata:

```bash
openexit init ./ghe-demo --source github-enterprise --target forgejo
openexit collect github-fixture --project ./ghe-demo --input ./testdata/github-enterprise/small.json
openexit run --project ./ghe-demo
```

The same path can collect read-only live GitHub or GitHub Enterprise metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files. Organization GitHub App installation metadata is collected when the token can read organization administration metadata; otherwise OpenExit records a warning and continues collecting repository-scoped metadata.

```bash
export GITHUB_TOKEN=<read-only-token>
openexit init ./ghe-live --source github-enterprise --target forgejo
openexit collect github --project ./ghe-live --owner acme --token-env GITHUB_TOKEN
openexit run --project ./ghe-live
```

For GitHub Enterprise Server, pass the API root:

```bash
openexit collect github --project ./ghe-live --owner acme --base-url https://github.example.com/api/v3
```

Use repeatable `--repo` flags to restrict collection to selected repositories:

```bash
openexit collect github --project ./ghe-live --owner acme --repo acme/platform-api --repo docs-site
```

The Okta/Auth0 to Keycloak/Zitadel fixture path uses local JSON metadata:

```bash
openexit init ./identity-demo --source identity --target keycloak-zitadel
openexit collect identity-fixture --project ./identity-demo --input ./testdata/identity/small.json
openexit run --project ./identity-demo
```

The same path can collect read-only live Okta metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export OKTA_API_TOKEN=<read-only-token>
openexit init ./okta-live --source identity --target keycloak-zitadel
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --token-env OKTA_API_TOKEN
openexit run --project ./okta-live
```

Use repeatable `--break-glass-user` flags to verify named emergency accounts and capture whether they have active factors:

```bash
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --break-glass-user breakglass-admin@example.com
```

The same identity path can collect read-only live Auth0 metadata. Set an Auth0 Management API token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export AUTH0_MANAGEMENT_TOKEN=<read-only-management-token>
openexit init ./auth0-live --source identity --target keycloak-zitadel
openexit collect auth0 --project ./auth0-live --domain https://example.us.auth0.com --token-env AUTH0_MANAGEMENT_TOKEN
openexit run --project ./auth0-live
```

Use repeatable `--break-glass-user` flags with an email, username, or Auth0 user ID to verify emergency accounts:

```bash
openexit collect auth0 --project ./auth0-live --domain https://example.us.auth0.com --break-glass-user breakglass-admin@example.com
```

The Cloudflare/Akamai to Varnish/HAProxy/Coraza fixture path uses local JSON metadata:

```bash
openexit init ./edge-demo --source edge --target varnish-haproxy-coraza
openexit collect edge-fixture --project ./edge-demo --input ./testdata/edge/small.json
openexit run --project ./edge-demo
```

The same path can collect read-only live Cloudflare metadata. Set the API token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export CLOUDFLARE_API_TOKEN=<read-only-token>
openexit init ./cloudflare-live --source edge --target varnish-haproxy-coraza
openexit collect cloudflare --project ./cloudflare-live --zone-id <zone-id> --token-env CLOUDFLARE_API_TOKEN
openexit run --project ./cloudflare-live
```

The same path can collect read-only live Akamai metadata. The collector reads EdgeGrid credentials from `~/.edgerc` or the `AKAMAI_HOST`, `AKAMAI_CLIENT_TOKEN`, `AKAMAI_ACCESS_TOKEN`, and `AKAMAI_CLIENT_SECRET` environment variables. It records Edge DNS recordsets, Property Manager hostnames and rules, and optional AppSec custom-rule metadata without storing credential values.

```bash
openexit init ./akamai-live --source edge --target varnish-haproxy-coraza
openexit collect akamai \
  --project ./akamai-live \
  --zone example.com \
  --property-id prp_12345 \
  --contract-id ctr_1-ABCDEF \
  --group-id grp_12345 \
  --security-config-id 12345:7
openexit run --project ./akamai-live
```

Use repeatable `--zone`, `--property-id`, and `--security-config-id` flags to scope collection. Use `--discover-properties` with `--contract-id` and `--group-id` to list accessible Property Manager properties before collection, and `--account-switch-key` when the API client needs to act against another account.

The OpenAI/Anthropic to vLLM/LiteLLM fixture path uses local JSON metadata:

```bash
openexit init ./ai-demo --source ai-provider --target vllm-litellm
openexit collect ai-fixture --project ./ai-demo --input ./testdata/ai-provider/small.json
openexit run --project ./ai-demo
```

The same path can collect read-only aggregate OpenAI usage. Set an OpenAI admin key in an environment variable; OpenExit reads it at runtime and does not write it into project files or evidence. The live collector records model-grouped token usage and available model metadata, not raw prompts or responses.

```bash
export OPENAI_ADMIN_KEY=<admin-key>
openexit init ./openai-live --source ai-provider --target vllm-litellm
openexit collect openai \
  --project ./openai-live \
  --admin-key-env OPENAI_ADMIN_KEY \
  --workspace acme \
  --owner platform-ai \
  --fallback-strategy manual-queue \
  --fallback-manual-queue
openexit run --project ./openai-live
```

Use `--days` to change the aggregate usage window and `--peak-days` to change the hourly peak-estimate window. Use `--organization-id` or `--project-id` only when your OpenAI account requires those headers.

The same path can collect read-only aggregate Anthropic Messages API usage. Set an Anthropic Admin API key in an environment variable; OpenExit reads it at runtime and does not write it into project files or evidence. The live collector records model-grouped token usage and server tool metadata, not raw prompts or responses.

```bash
export ANTHROPIC_ADMIN_KEY=<admin-key>
openexit init ./anthropic-live --source ai-provider --target vllm-litellm
openexit collect anthropic \
  --project ./anthropic-live \
  --admin-key-env ANTHROPIC_ADMIN_KEY \
  --workspace platform \
  --workspace-id wrkspc_01JwQvzr7rXLA5AGx3HKfFUJ \
  --owner platform-ai \
  --fallback-strategy manual-queue \
  --fallback-manual-queue
openexit run --project ./anthropic-live
```

Use `--api-key-id`, `--workspace-id`, or `--model` to restrict the Anthropic usage query. Use `--days` to change the daily aggregate usage window and `--peak-days` to change the hourly peak-estimate window.
