# Cloudflare/Akamai To Varnish/HAProxy/Coraza

This path supports local fixture assessment plus read-only live Cloudflare and Akamai inventory collection.

Collected metadata:

- DNS records
- WAF rules
- cache rules
- redirects
- origin config
- TLS settings
- bot rules
- page rules

The live Cloudflare collector gathers DNS records, WAF/custom/managed ruleset metadata, cache rules, dynamic redirects, page rules, zone TLS settings, inferred origin entries from DNS records, and bot-related rules. It does not write to Cloudflare and it does not store API tokens.

The live Akamai collector gathers Edge DNS recordsets, Property Manager hostnames and rule-tree metadata, origins, cache behaviors, redirects, TLS/HSTS metadata, Bot Manager behavior metadata, and optional AppSec custom-rule metadata. It uses EdgeGrid credentials from environment variables or `~/.edgerc`; credential values are not written to project files or evidence.

Live Cloudflare collection:

```bash
export CLOUDFLARE_API_TOKEN=<read-only-token>
openexit init ./cloudflare-live --source edge --target varnish-haproxy-coraza
openexit collect cloudflare --project ./cloudflare-live --zone-id <zone-id> --token-env CLOUDFLARE_API_TOKEN
```

The collector treats plan-specific or unavailable Cloudflare products as warnings. Missing ruleset phases do not block DNS, TLS, page-rule, or available ruleset evidence collection.

Live Akamai collection:

```bash
openexit init ./akamai-live --source edge --target varnish-haproxy-coraza
openexit collect akamai \
  --project ./akamai-live \
  --zone example.com \
  --property-id prp_12345 \
  --contract-id ctr_1-ABCDEF \
  --group-id grp_12345 \
  --security-config-id 12345:7
```

Use repeatable `--zone`, `--property-id`, and `--security-config-id` flags to scope Akamai collection. `--discover-properties` can list accessible Property Manager properties when `--contract-id` and `--group-id` are supplied. AppSec collection is optional because many Akamai API clients have separate security permissions.

Generated artifacts:

- VCL candidates
- HAProxy candidates
- Coraza rule candidates
- `cache-parity-report.md`
- `waf-enforcement-risk-report.md`

Validation checks edge candidates for OpenExit review markers, absence of secret-like content, Varnish backend/cache/page-rule coverage, HAProxy redirect/origin/TLS/health-check coverage, Coraza WAF/bot-rule coverage, and preserved evidence refs.

The assessment flags wildcard and high-TTL DNS records, observe-only or managed WAF rules, challenge actions, cache-key parity issues, redirect pattern translation, origin TLS gaps, low TLS versions, disabled HSTS, and coupled page-rule behavior.
