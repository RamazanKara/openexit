package assessment

import (
	"context"
	"fmt"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type IdentityAnalyzer struct{}

func (IdentityAnalyzer) Name() string { return "identity-keycloak-zitadel" }

func (IdentityAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	if inv.Source.Type != "identity" && len(inv.Assets.IdentityApps) == 0 {
		return nil, nil
	}
	var findings []Finding
	for _, app := range inv.Assets.IdentityApps {
		if len(app.Owners) == 0 {
			findings = append(findings, identityFinding(app.EvidenceRef, "identity.application.owner-missing.001", "medium", "Application owner is missing", fmt.Sprintf("%s has no captured owner.", app.Name), []string{"identity-application:" + app.ID}, "Assign an accountable owner before realm/client migration sign-off."))
		}
		if len(app.Groups) == 0 {
			findings = append(findings, identityFinding(app.EvidenceRef, "identity.application.group-mapping-missing.001", "medium", "Application group mapping is missing", fmt.Sprintf("%s has no captured group assignments.", app.Name), []string{"identity-application:" + app.ID}, "Map source assignments to target groups or document why the client is public."))
		}
		for _, redirectURI := range app.RedirectURIs {
			if insecureRedirectURI(redirectURI) {
				findings = append(findings, identityFinding(app.EvidenceRef, "identity.redirect-uri.insecure.001", "high", "Redirect URI needs security review", fmt.Sprintf("%s uses redirect URI %s.", app.Name, redirectURI), []string{"identity-application:" + app.ID}, "Replace wildcard, localhost, or plain HTTP redirect URIs before production cutover."))
				break
			}
		}
		if isOIDC(app.Protocol) && containsFold(app.GrantTypes, "implicit") {
			findings = append(findings, identityFinding(app.EvidenceRef, "identity.oidc.implicit-grant.001", "high", "OIDC client uses implicit grant", fmt.Sprintf("%s allows the implicit grant.", app.Name), []string{"identity-application:" + app.ID}, "Migrate to authorization code with PKCE or document a temporary exception."))
		}
		if isSAML(app.Protocol) && !app.SAMLSigningCertPresent {
			findings = append(findings, identityFinding(app.EvidenceRef, "identity.saml.signing-cert-missing.001", "high", "SAML signing certificate metadata is missing", fmt.Sprintf("%s has no captured SAML signing certificate metadata.", app.Name), []string{"identity-application:" + app.ID}, "Collect SAML certificate metadata before generating the target client configuration."))
		}
		if isSAML(app.Protocol) && app.SAMLSigningCertPresent && strings.TrimSpace(app.SAMLSigningCertExpiresAt) == "" {
			findings = append(findings, identityFinding(app.EvidenceRef, "identity.saml.signing-cert-expiry-unknown.001", "medium", "SAML signing certificate expiry is unknown", fmt.Sprintf("%s has signing metadata but no expiry date.", app.Name), []string{"identity-application:" + app.ID}, "Record the certificate expiry and schedule rotation in the cutover plan."))
		}
	}
	for _, group := range inv.Assets.IdentityGroups {
		if len(group.Owners) == 0 {
			findings = append(findings, identityFinding(group.EvidenceRef, "identity.group.owner-missing.001", "medium", "Group owner is missing", fmt.Sprintf("%s has no captured owner.", group.Name), []string{"identity-group:" + group.ID}, "Assign a group owner before migration approval."))
		}
	}
	for _, policy := range inv.Assets.IdentityPolicies {
		if len(policy.Groups) == 0 {
			findings = append(findings, identityFinding(policy.EvidenceRef, "identity.policy.group-mapping-missing.001", "medium", "Policy has no group scope", fmt.Sprintf("%s has no captured group scope.", policy.Name), []string{"identity-policy:" + policy.ID}, "Review policy ordering and map it to explicit target groups."))
		}
		if !policy.EnforcesMFA && strings.Contains(strings.ToLower(policy.Type), "sign") {
			findings = append(findings, identityFinding(policy.EvidenceRef, "identity.policy.mfa-not-enforced.001", "high", "Sign-on policy does not enforce MFA", fmt.Sprintf("%s does not enforce MFA.", policy.Name), []string{"identity-policy:" + policy.ID}, "Require MFA in the equivalent Keycloak/Zitadel policy or record an approved exception."))
		}
	}
	if len(inv.Assets.MFASettings) == 0 {
		findings = append(findings, identityFinding("evidence://inventory/summary", "identity.mfa.settings-missing.001", "high", "MFA settings were not captured", "No tenant-level MFA settings are present in the inventory.", []string{"identity-mfa:tenant"}, "Collect MFA configuration before cutover planning."))
	}
	for _, setting := range inv.Assets.MFASettings {
		if !setting.Required {
			findings = append(findings, identityFinding(setting.EvidenceRef, "identity.mfa.not-required.001", "high", "MFA is not globally required", fmt.Sprintf("MFA setting %s is not required.", setting.Name), []string{"identity-mfa:" + setting.Name}, "Require MFA for privileged and production-facing applications before cutover."))
		}
		if hasWeakFactor(setting.Factors) {
			findings = append(findings, identityFinding(setting.EvidenceRef, "identity.mfa.weak-factor.001", "medium", "MFA policy allows weak factors", fmt.Sprintf("MFA setting %s allows weak factors.", setting.Name), []string{"identity-mfa:" + setting.Name}, "Prefer phishing-resistant or TOTP/WebAuthn factors and document any SMS/email exceptions."))
		}
	}
	if len(inv.Assets.BreakGlassAccounts) == 0 {
		findings = append(findings, identityFinding("evidence://inventory/summary", "identity.break-glass.missing.001", "high", "Break-glass account was not captured", "No break-glass account metadata is present in the inventory.", []string{"break-glass:tenant"}, "Create and verify a controlled break-glass procedure before migration."))
	}
	for _, account := range inv.Assets.BreakGlassAccounts {
		if !account.MFAEnabled {
			findings = append(findings, identityFinding(account.EvidenceRef, "identity.break-glass.mfa-missing.001", "high", "Break-glass account is missing MFA", fmt.Sprintf("Break-glass account %s does not have MFA enabled.", account.Username), []string{"break-glass:" + account.Username}, "Enable MFA, record custody, and test emergency access before cutover."))
		}
	}
	return findings, nil
}

func identityFinding(evidenceRef, id, severity, title, description string, affected []string, recommendation string) Finding {
	return Finding{
		ID:             id,
		Severity:       severity,
		Title:          title,
		Description:    description,
		AffectedAssets: affected,
		EvidenceRefs:   []string{evidenceRef},
		Recommendation: recommendation,
	}
}

func insecureRedirectURI(value string) bool {
	uri := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(uri, "http://") || strings.Contains(uri, "localhost") || strings.Contains(uri, "*")
}

func isOIDC(protocol string) bool {
	return strings.EqualFold(strings.TrimSpace(protocol), "oidc")
}

func isSAML(protocol string) bool {
	return strings.EqualFold(strings.TrimSpace(protocol), "saml")
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}

func hasWeakFactor(factors []string) bool {
	for _, factor := range factors {
		switch strings.ToLower(strings.TrimSpace(factor)) {
		case "sms", "email", "voice", "security_question":
			return true
		}
	}
	return false
}
