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

Generate individual artifacts with `openexit generate --artifact <name>`. The primary Datadog path supports `assessment`, `risk-register`, `manual-review`, `cost-drivers`, `target-architecture`, `acceptance-criteria`, `rollback-plan`, `runbook`, `restore-drill-checklist`, `alert-shadowing-plan`, `grafana-dashboards`, `prometheus-rules`, `opentelemetry`, and `argocd`.

The GitHub Enterprise to Forgejo fixture path uses local JSON metadata:

```bash
openexit init ./ghe-demo --source github-enterprise --target forgejo
openexit collect github-fixture --project ./ghe-demo --input ./testdata/github-enterprise/small.json
openexit assess --project ./ghe-demo --target forgejo
openexit generate --project ./ghe-demo --all
openexit validate --project ./ghe-demo
```

The Okta/Auth0 to Keycloak/Zitadel fixture path uses local JSON metadata:

```bash
openexit init ./identity-demo --source identity --target keycloak-zitadel
openexit collect identity-fixture --project ./identity-demo --input ./testdata/identity/small.json
openexit assess --project ./identity-demo --target keycloak-zitadel
openexit generate --project ./identity-demo --all
openexit validate --project ./identity-demo
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
