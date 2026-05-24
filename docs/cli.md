# CLI

See `openexit --help` for the command tree.

The minimum local demo is:

```bash
openexit init ./demo
openexit collect fixture --project ./demo --input ./testdata/datadog/small.json
openexit assess --project ./demo --target grafana-lgtm
openexit generate --project ./demo --all
openexit validate --project ./demo
openexit export --project ./demo --format zip --out ./openexit-demo.zip
```

The CI test suite also runs this definition-of-done pipeline against the fixture inventory and checks that the generated project layout and export bundle contain the expected artifacts. Bundle checksums are verified against the archived file bytes.

The GitHub Enterprise to Forgejo preview uses a fixture-first path:

```bash
openexit init ./ghe-demo
openexit collect github-fixture --project ./ghe-demo --input ./testdata/github-enterprise/small.json
openexit assess --project ./ghe-demo --target forgejo
openexit generate --project ./ghe-demo --all
openexit validate --project ./ghe-demo
```

The Okta/Auth0 to Keycloak/Zitadel preview also uses a fixture-first path:

```bash
openexit init ./identity-demo
openexit collect identity-fixture --project ./identity-demo --input ./testdata/identity/small.json
openexit assess --project ./identity-demo --target keycloak-zitadel
openexit generate --project ./identity-demo --all
openexit validate --project ./identity-demo
```

The Cloudflare/Akamai to Varnish/HAProxy/Coraza preview uses fixture metadata too:

```bash
openexit init ./edge-demo
openexit collect edge-fixture --project ./edge-demo --input ./testdata/edge/small.json
openexit assess --project ./edge-demo --target varnish-haproxy-coraza
openexit generate --project ./edge-demo --all
openexit validate --project ./edge-demo
```
