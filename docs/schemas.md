# Schemas

OpenExit schemas live under `schemas/` and mirror the typed Go manifests. The CLI performs typed validation plus YAML/JSON parse checks in v0.1.

Inventory dashboards can include optional `dataSources` and `templateVariables` fields so assessment can flag Grafana mapping risk. SLOs can include optional `sli`, `burnRateMonitorIds`, and `dashboardRefs` fields. The top-level inventory `volumes` section records whether log and trace volume assumptions are known.

For the GitHub Enterprise to Forgejo preview, inventory can include `repositories`, `teams`, `branchProtections`, `actionsWorkflows`, `secrets`, `runners`, `deployKeys`, and `githubApps`. Secret entries are metadata only; OpenExit should never collect or store secret values.

For the Okta/Auth0 to Keycloak/Zitadel preview, inventory can include `identityApplications`, `identityGroups`, `identityPolicies`, `mfaSettings`, and `breakGlassAccounts`. Client entries include redirect URIs, grant types, owners, group assignments, and SAML signing metadata; they do not include client secrets.

For the Cloudflare/Akamai to Varnish/HAProxy/Coraza preview, inventory can include `dnsRecords`, `wafRules`, `cacheRules`, `redirects`, `origins`, `tlsSettings`, `botRules`, and `pageRules`. WAF entries contain rule metadata and expressions only; they should not contain provider credentials.
