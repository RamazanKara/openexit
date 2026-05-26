# Cloudflare/Akamai To Varnish/HAProxy/Coraza

This path supports local fixture assessment and read-only live Cloudflare inventory collection. Akamai is currently fixture-only.

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

Live Cloudflare collection:

```bash
export CLOUDFLARE_API_TOKEN=<read-only-token>
openexit init ./cloudflare-live --source edge --target varnish-haproxy-coraza
openexit collect cloudflare --project ./cloudflare-live --zone-id <zone-id> --token-env CLOUDFLARE_API_TOKEN
```

The collector treats plan-specific or unavailable Cloudflare products as warnings. Missing ruleset phases do not block DNS, TLS, page-rule, or available ruleset evidence collection.

Generated artifacts:

- VCL candidates
- HAProxy candidates
- Coraza rule candidates
- `cache-parity-report.md`
- `waf-enforcement-risk-report.md`

The assessment flags wildcard and high-TTL DNS records, observe-only or managed WAF rules, challenge actions, cache-key parity issues, redirect pattern translation, origin TLS gaps, low TLS versions, disabled HSTS, and coupled page-rule behavior.
