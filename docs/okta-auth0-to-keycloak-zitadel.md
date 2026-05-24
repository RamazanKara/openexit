# Okta/Auth0 To Keycloak/Zitadel

This is the v0.3 preview path. It is fixture-first and makes no live API calls.

Collected fixture metadata:

- applications
- SAML/OIDC clients
- groups
- policies
- MFA settings
- redirect URIs
- owners
- break-glass account metadata

Generated artifacts:

- `identity-migration-risk-register.md`
- `realm-client-candidate.yaml`
- `break-glass-checklist.md`
- `identity-cutover-plan.md`
- `identity-rollback-plan.md`

The assessment flags missing owners, missing group mappings, insecure redirect URIs, OIDC implicit grant usage, SAML signing metadata gaps, MFA policy gaps, weak MFA factors, and break-glass account issues.
