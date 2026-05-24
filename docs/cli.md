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
