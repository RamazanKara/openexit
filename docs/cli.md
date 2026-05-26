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
openexit generate --project ./demo --all
openexit validate --project ./demo
openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

The CI test suite also runs this definition-of-done pipeline against the fixture inventory and checks that the generated project layout and export bundle contain the expected artifacts. Bundle checksums are verified against the archived file bytes.

Generate individual artifacts with `openexit generate --artifact <name>`. The primary Datadog path supports `assessment`, `risk-register`, `manual-review`, `cost-drivers`, `target-architecture`, `acceptance-criteria`, `rollback-plan`, `runbook`, `restore-drill-checklist`, `alert-shadowing-plan`, `grafana-dashboards`, `prometheus-rules`, `opentelemetry`, and `argocd`. The GitHub Enterprise path also supports `forgejo-migration-candidate`.

The GitHub Enterprise to Forgejo fixture path uses local JSON metadata:

```bash
openexit init ./ghe-demo --source github-enterprise --target forgejo
openexit collect github-fixture --project ./ghe-demo --input ./testdata/github-enterprise/small.json
openexit assess --project ./ghe-demo --target forgejo
openexit generate --project ./ghe-demo --all
openexit validate --project ./ghe-demo
```

The same path can collect read-only live GitHub or GitHub Enterprise metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export GITHUB_TOKEN=<read-only-token>
openexit init ./ghe-live --source github-enterprise --target forgejo
openexit collect github --project ./ghe-live --owner acme --token-env GITHUB_TOKEN
openexit assess --project ./ghe-live --target forgejo
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
openexit generate --project ./identity-demo --all
openexit validate --project ./identity-demo
```

The same path can collect read-only live Okta metadata. Set the token in an environment variable; OpenExit reads it at runtime and does not write it into project files.

```bash
export OKTA_API_TOKEN=<read-only-token>
openexit init ./okta-live --source identity --target keycloak-zitadel
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --token-env OKTA_API_TOKEN
openexit assess --project ./okta-live --target keycloak-zitadel
openexit generate --project ./okta-live --all
openexit validate --project ./okta-live
```

Use repeatable `--break-glass-user` flags to verify named emergency accounts and capture whether they have active factors:

```bash
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --break-glass-user breakglass-admin@example.com
```

The Cloudflare/Akamai to Varnish/HAProxy/Coraza fixture path uses local JSON metadata:

```bash
openexit init ./edge-demo --source edge --target varnish-haproxy-coraza
openexit collect edge-fixture --project ./edge-demo --input ./testdata/edge/small.json
openexit assess --project ./edge-demo --target varnish-haproxy-coraza
openexit generate --project ./edge-demo --all
openexit validate --project ./edge-demo
```

The OpenAI/Anthropic to vLLM/LiteLLM fixture path uses local JSON metadata:

```bash
openexit init ./ai-demo --source ai-provider --target vllm-litellm
openexit collect ai-fixture --project ./ai-demo --input ./testdata/ai-provider/small.json
openexit assess --project ./ai-demo --target vllm-litellm
openexit generate --project ./ai-demo --all
openexit validate --project ./ai-demo
```
