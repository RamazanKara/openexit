# Okta/Auth0 To Keycloak/Zitadel

This path supports local fixture assessment and read-only live Okta inventory collection. Auth0 is currently fixture-only.

Collected metadata:

- applications
- SAML/OIDC clients
- groups
- policies
- MFA settings
- redirect URIs
- owners
- break-glass account metadata

The live Okta collector gathers applications, SAML/OIDC client settings, app group assignments, groups, group member counts, policy/rule metadata, org MFA factors, and explicitly named break-glass users. It does not collect client secrets, user passwords, factor secrets, or token values, and it does not write to Okta.

Live Okta collection:

```bash
export OKTA_API_TOKEN=<read-only-token>
openexit init ./okta-live --source identity --target keycloak-zitadel
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --token-env OKTA_API_TOKEN
```

To capture emergency account readiness, pass one or more break-glass users:

```bash
openexit collect okta --project ./okta-live --org-url https://dev-123456.okta.com --break-glass-user breakglass-admin@example.com
```

Generated artifacts:

- `identity-migration-risk-register.md`
- `realm-client-candidate.yaml`
- `break-glass-checklist.md`
- `identity-cutover-plan.md`
- `identity-rollback-plan.md`

The assessment flags missing owners, missing group mappings, insecure redirect URIs, OIDC implicit grant usage, SAML signing metadata gaps, MFA policy gaps, weak MFA factors, and break-glass account issues.
