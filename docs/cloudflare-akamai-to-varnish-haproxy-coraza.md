# Cloudflare/Akamai To Varnish/HAProxy/Coraza

This is a fixture-complete assessment path. It makes no live API calls.

Collected fixture metadata:

- DNS records
- WAF rules
- cache rules
- redirects
- origin config
- TLS settings
- bot rules
- page rules

Generated artifacts:

- VCL candidates
- HAProxy candidates
- Coraza rule candidates
- `cache-parity-report.md`
- `waf-enforcement-risk-report.md`

The assessment flags wildcard and high-TTL DNS records, observe-only or managed WAF rules, challenge actions, cache-key parity issues, redirect pattern translation, origin TLS gaps, low TLS versions, disabled HSTS, and coupled page-rule behavior.
