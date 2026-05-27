# CLI

See `openexit --help` for the command tree.

`openexit init` accepts `--source` and `--target`. If `--target` is omitted, the CLI uses the standard target for the selected source.

Release builds stamp version metadata:

```bash
make build VERSION=0.1.0
./bin/openexit version
```

The minimum local demo is:

```bash
openexit init ./demo --source datadog --target grafana-lgtm
openexit collect fixture --project ./demo --input ./testdata/datadog/small.json
openexit assess --project ./demo --target grafana-lgtm
openexit map --project ./demo
openexit generate --project ./demo --all
openexit validate --project ./demo
openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

The CI test suite also runs this definition-of-done pipeline against the fixture inventory and checks that the generated project layout and export bundle contain the expected artifacts. Bundle checksums are verified against the archived file bytes.

`openexit validate` performs typed consistency checks, embedded JSON Schema validation, Grafana dashboard candidate validation, Prometheus alert-rule candidate validation, OpenTelemetry collector candidate validation, ArgoCD candidate validation, YAML/JSON parse checks, evidence reference checks, secret scanning, and optional external tool checks when `promtool` or `kubeconform` are installed.

`openexit export` refuses to package symlinks from exported project sections, even with `--force`, so evidence bundles cannot accidentally include files from outside the project tree.

Generate individual artifacts with `openexit generate --artifact <name>`. The primary Datadog path supports `mapping`, `assessment`, `risk-register`, `manual-review`, `cost-drivers`, `target-architecture`, `acceptance-criteria`, `rollback-plan`, `runbook`, `restore-drill-checklist`, `alert-shadowing-plan`, `migration-plan`, `grafana-dashboards`, `prometheus-rules`, `opentelemetry`, and `argocd`. The GitHub Enterprise path also supports `forgejo-migration-candidate`.

The GitHub Enterprise to Forgejo fixture path uses local JSON metadata:

```bash
openexit init ./ghe-demo --source github-enterprise --target forgejo
openexit collect github-fixture --project ./ghe-demo --input ./testdata/github-enterprise/small.json
openexit assess --project ./ghe-demo --target forgejo
openexit map --project ./ghe-demo
openexit generate --project ./ghe-demo --all
openexit validate --project ./ghe-demo
```

The same path can collect read-only live GitHub or GitHub Enterprise metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files. Organization GitHub App installation metadata is collected when the token can read organization administration metadata; otherwise OpenExit records a warning and continues collecting repository-scoped metadata.

```bash
export GITHUB_TOKEN=<read-only-token>
openexit init ./ghe-live --source github-enterprise --target forgejo
openexit collect github --project ./ghe-live --owner acme --token-env GITHUB_TOKEN
openexit assess --project ./ghe-live --target forgejo
openexit map --project ./ghe-live
openexit generate --project ./ghe-live --all
openexit validate --project ./ghe-live
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
openexit assess --project ./identity-demo --target keycloak-zitadel
openexit map --project ./identity-demo
openexit generate --project ./identity-demo --all
openexit validate --project ./identity-demo
```

The same path can collect read-only live Okta metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export OKTA_API_TOKEN=<read-only-token>
openexit init ./okta-live --source identity --target keycloak-zitadel
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --token-env OKTA_API_TOKEN
openexit assess --project ./okta-live --target keycloak-zitadel
openexit map --project ./okta-live
openexit generate --project ./okta-live --all
openexit validate --project ./okta-live
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
openexit assess --project ./auth0-live --target keycloak-zitadel
openexit map --project ./auth0-live
openexit generate --project ./auth0-live --all
openexit validate --project ./auth0-live
```

Use repeatable `--break-glass-user` flags with an email, username, or Auth0 user ID to verify emergency accounts:

```bash
openexit collect auth0 --project ./auth0-live --domain https://example.us.auth0.com --break-glass-user breakglass-admin@example.com
```

The Cloudflare/Akamai to Varnish/HAProxy/Coraza fixture path uses local JSON metadata:

```bash
openexit init ./edge-demo --source edge --target varnish-haproxy-coraza
openexit collect edge-fixture --project ./edge-demo --input ./testdata/edge/small.json
openexit assess --project ./edge-demo --target varnish-haproxy-coraza
openexit map --project ./edge-demo
openexit generate --project ./edge-demo --all
openexit validate --project ./edge-demo
```

The same path can collect read-only live Cloudflare metadata. Set the API token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export CLOUDFLARE_API_TOKEN=<read-only-token>
openexit init ./cloudflare-live --source edge --target varnish-haproxy-coraza
openexit collect cloudflare --project ./cloudflare-live --zone-id <zone-id> --token-env CLOUDFLARE_API_TOKEN
openexit assess --project ./cloudflare-live --target varnish-haproxy-coraza
openexit map --project ./cloudflare-live
openexit generate --project ./cloudflare-live --all
openexit validate --project ./cloudflare-live
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
openexit assess --project ./akamai-live --target varnish-haproxy-coraza
openexit map --project ./akamai-live
openexit generate --project ./akamai-live --all
openexit validate --project ./akamai-live
```

Use repeatable `--zone`, `--property-id`, and `--security-config-id` flags to scope collection. Use `--discover-properties` with `--contract-id` and `--group-id` to list accessible Property Manager properties before collection, and `--account-switch-key` when the API client needs to act against another account.

The OpenAI/Anthropic to vLLM/LiteLLM fixture path uses local JSON metadata:

```bash
openexit init ./ai-demo --source ai-provider --target vllm-litellm
openexit collect ai-fixture --project ./ai-demo --input ./testdata/ai-provider/small.json
openexit assess --project ./ai-demo --target vllm-litellm
openexit map --project ./ai-demo
openexit generate --project ./ai-demo --all
openexit validate --project ./ai-demo
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
openexit assess --project ./openai-live --target vllm-litellm
openexit map --project ./openai-live
openexit generate --project ./openai-live --all
openexit validate --project ./openai-live
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
openexit assess --project ./anthropic-live --target vllm-litellm
openexit map --project ./anthropic-live
openexit generate --project ./anthropic-live --all
openexit validate --project ./anthropic-live
```

Use `--api-key-id`, `--workspace-id`, or `--model` to restrict the Anthropic usage query. Use `--days` to change the daily aggregate usage window and `--peak-days` to change the hourly peak-estimate window.
