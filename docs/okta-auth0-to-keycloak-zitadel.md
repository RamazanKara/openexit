# Okta/Auth0 To Keycloak/Zitadel

This path supports local fixture assessment and read-only live Okta and Auth0 inventory collection.

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

The live Auth0 collector gathers clients, role metadata and member counts, action/rule metadata, Guardian MFA factors, and explicitly named break-glass users. It requests metadata fields only for clients and does not persist client secrets, action code, rule scripts, user passwords, MFA secrets, or token values. It does not write to Auth0.

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

Live Auth0 collection:

```bash
export AUTH0_MANAGEMENT_TOKEN=<read-only-management-token>
openexit init ./auth0-live --source identity --target keycloak-zitadel
openexit collect auth0 --project ./auth0-live --domain https://example.us.auth0.com --token-env AUTH0_MANAGEMENT_TOKEN
```

To capture emergency account readiness, pass one or more break-glass users by email, username, or Auth0 user ID:

```bash
openexit collect auth0 --project ./auth0-live --domain https://example.us.auth0.com --break-glass-user breakglass-admin@example.com
```

Auth0 API surface used:

- Management API clients: `GET /api/v2/clients`.
- Management API roles and role users: `GET /api/v2/roles` and `GET /api/v2/roles/{id}/users`.
- Management API actions and rules: `GET /api/v2/actions/actions` and `GET /api/v2/rules`.
- Management API Guardian factors: `GET /api/v2/guardian/factors`.
- Management API users and authentication methods for explicit break-glass users: `GET /api/v2/users` and `GET /api/v2/users/{id}/authentication-methods`.

Generated artifacts:

- `identity-migration-risk-register.md`
- `realm-client-candidate.yaml`
- `break-glass-checklist.md`
- `identity-cutover-plan.md`
- `identity-rollback-plan.md`

Validation checks the identity realm/client candidate for source/target metadata, non-production safety flags, absence of credentials, realm provider metadata, application/client coverage, group coverage, policy coverage, MFA settings, break-glass account metadata, and preserved evidence refs.

The assessment flags missing owners, missing group mappings, insecure redirect URIs, OIDC implicit grant usage, SAML signing metadata gaps, MFA policy gaps, weak MFA factors, and break-glass account issues.
