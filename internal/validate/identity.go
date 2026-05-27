package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

type identityRealmClientCandidate struct {
	APIVersion             string                        `yaml:"apiVersion"`
	Kind                   string                        `yaml:"kind"`
	Metadata               identityCandidateMetadata     `yaml:"metadata"`
	Realm                  identityRealmCandidate        `yaml:"realm"`
	Clients                []identityClientCandidate     `yaml:"clients"`
	Groups                 []identityGroupCandidate      `yaml:"groups"`
	Policies               []identityPolicyCandidate     `yaml:"policies"`
	MFASettings            []identityMFACandidate        `yaml:"mfaSettings"`
	BreakGlassAccounts     []identityBreakGlassCandidate `yaml:"breakGlassAccounts"`
	HumanReviewRequired    bool                          `yaml:"humanReviewRequired"`
	CredentialsIncluded    bool                          `yaml:"credentialsIncluded"`
	ProductionReady        bool                          `yaml:"productionReady"`
	CandidateGeneratedFrom string                        `yaml:"candidateGeneratedFrom"`
	rawText                string                        `yaml:"-"`
}

type identityCandidateMetadata struct {
	Project string `yaml:"project"`
	Source  string `yaml:"source"`
	Target  string `yaml:"target"`
}

type identityRealmCandidate struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
}

type identityClientCandidate struct {
	ID                       string   `yaml:"id"`
	Name                     string   `yaml:"name"`
	Protocol                 string   `yaml:"protocol"`
	ClientID                 string   `yaml:"clientId"`
	Owners                   []string `yaml:"owners"`
	Groups                   []string `yaml:"groups"`
	RedirectURIs             []string `yaml:"redirectUris"`
	GrantTypes               []string `yaml:"grantTypes"`
	SAMLSigningCertPresent   bool     `yaml:"samlSigningCertPresent"`
	SAMLSigningCertExpiresAt string   `yaml:"samlSigningCertExpiresAt"`
	EvidenceRef              string   `yaml:"evidenceRef"`
}

type identityGroupCandidate struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Owners      []string `yaml:"owners"`
	Members     int      `yaml:"members"`
	EvidenceRef string   `yaml:"evidenceRef"`
}

type identityPolicyCandidate struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"`
	Groups       []string `yaml:"groups"`
	Applications []string `yaml:"applications"`
	EnforcesMFA  bool     `yaml:"enforcesMfa"`
	Priority     int      `yaml:"priority"`
	EvidenceRef  string   `yaml:"evidenceRef"`
}

type identityMFACandidate struct {
	Name        string   `yaml:"name"`
	Required    bool     `yaml:"required"`
	Factors     []string `yaml:"factors"`
	EvidenceRef string   `yaml:"evidenceRef"`
}

type identityBreakGlassCandidate struct {
	Username    string `yaml:"username"`
	MFAEnabled  bool   `yaml:"mfaEnabled"`
	ManagedBy   string `yaml:"managedBy"`
	EvidenceRef string `yaml:"evidenceRef"`
}

func addIdentityRealmClientCandidateChecks(projectDir string, inv *inventory.Inventory, a *assessment.Assessment, add func(string, string, string, bool)) {
	if inv == nil || inv.Source.Type != "identity" {
		return
	}
	path := filepath.Join(projectDir, "generated-config", "identity", "realm-client-candidate.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		add("identity-realm-client-candidate", "failed", err.Error(), true)
		return
	}
	candidate, err := readIdentityRealmClientCandidate(path)
	if err != nil {
		add("identity-realm-client-candidate", "failed", err.Error(), true)
		return
	}
	var problems []string
	problems = append(problems, validateIdentityCandidateShape(candidate, inv, a)...)
	problems = append(problems, validateIdentityClientCandidates(candidate.Clients, inv.Assets.IdentityApps)...)
	problems = append(problems, validateIdentityGroupCandidates(candidate.Groups, inv.Assets.IdentityGroups)...)
	problems = append(problems, validateIdentityPolicyCandidates(candidate.Policies, inv.Assets.IdentityPolicies)...)
	problems = append(problems, validateIdentityMFACandidates(candidate.MFASettings, inv.Assets.MFASettings)...)
	problems = append(problems, validateIdentityBreakGlassCandidates(candidate.BreakGlassAccounts, inv.Assets.BreakGlassAccounts)...)
	problems = append(problems, validateIdentityCandidateREADME(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("identity-realm-client-candidate", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("identity-realm-client-candidate", "passed", "", true)
}

func readIdentityRealmClientCandidate(path string) (*identityRealmClientCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var candidate identityRealmClientCandidate
	if err := yaml.Unmarshal(data, &candidate); err != nil {
		return nil, err
	}
	candidate.rawText = string(data)
	return &candidate, nil
}

func validateIdentityCandidateShape(candidate *identityRealmClientCandidate, inv *inventory.Inventory, a *assessment.Assessment) []string {
	if candidate == nil {
		return []string{"identity realm/client candidate is empty"}
	}
	var problems []string
	if inventory.ContainsSecret(candidate.rawText) {
		problems = append(problems, "realm-client-candidate.yaml contains secret-like content")
	}
	if candidate.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q, got %q", APIVersion, candidate.APIVersion))
	}
	if candidate.Kind != "IdentityRealmClientCandidate" {
		problems = append(problems, fmt.Sprintf("kind must be IdentityRealmClientCandidate, got %q", candidate.Kind))
	}
	if candidate.Metadata.Project != inv.Metadata.Project {
		problems = append(problems, fmt.Sprintf("metadata.project must be %q, got %q", inv.Metadata.Project, candidate.Metadata.Project))
	}
	if candidate.Metadata.Source != inv.Source.Type {
		problems = append(problems, fmt.Sprintf("metadata.source must be %q, got %q", inv.Source.Type, candidate.Metadata.Source))
	}
	if a != nil && candidate.Metadata.Target != a.Target.Type {
		problems = append(problems, fmt.Sprintf("metadata.target must be %q, got %q", a.Target.Type, candidate.Metadata.Target))
	}
	if candidate.Realm.Name != inventory.Slug(inv.Metadata.Project) {
		problems = append(problems, fmt.Sprintf("realm.name must be %q, got %q", inventory.Slug(inv.Metadata.Project), candidate.Realm.Name))
	}
	if candidate.Realm.Provider != inv.Source.Site {
		problems = append(problems, fmt.Sprintf("realm.provider must be %q, got %q", inv.Source.Site, candidate.Realm.Provider))
	}
	for _, field := range []string{"humanReviewRequired:", "credentialsIncluded:", "productionReady:", "candidateGeneratedFrom:"} {
		if !strings.Contains(candidate.rawText, field) {
			problems = append(problems, "missing safety field "+strings.TrimSuffix(field, ":"))
		}
	}
	if !candidate.HumanReviewRequired {
		problems = append(problems, "humanReviewRequired must be true")
	}
	if candidate.CredentialsIncluded {
		problems = append(problems, "credentialsIncluded must be false")
	}
	if candidate.ProductionReady {
		problems = append(problems, "productionReady must be false")
	}
	if candidate.CandidateGeneratedFrom != "redacted-openexit-inventory" {
		problems = append(problems, fmt.Sprintf("candidateGeneratedFrom must be redacted-openexit-inventory, got %q", candidate.CandidateGeneratedFrom))
	}
	return problems
}

func validateIdentityClientCandidates(candidates []identityClientCandidate, apps []inventory.IdentityApp) []string {
	var problems []string
	byID := map[string]identityClientCandidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	if len(candidates) != len(apps) {
		problems = append(problems, fmt.Sprintf("clients count %d does not match inventory count %d", len(candidates), len(apps)))
	}
	for _, app := range apps {
		candidate, ok := byID[app.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf("clients: missing candidate for %s", app.ID))
			continue
		}
		if candidate.Name != app.Name || candidate.Protocol != app.Protocol || candidate.ClientID != app.ClientID || candidate.EvidenceRef != app.EvidenceRef {
			problems = append(problems, fmt.Sprintf("clients.%s must preserve name, protocol, clientId, and evidenceRef", app.ID))
		}
		if candidate.SAMLSigningCertPresent != app.SAMLSigningCertPresent || candidate.SAMLSigningCertExpiresAt != app.SAMLSigningCertExpiresAt {
			problems = append(problems, fmt.Sprintf("clients.%s must preserve SAML signing metadata", app.ID))
		}
		if !sameStringSet(candidate.Owners, app.Owners) {
			problems = append(problems, fmt.Sprintf("clients.%s.owners must match inventory", app.ID))
		}
		if !sameStringSet(candidate.Groups, app.Groups) {
			problems = append(problems, fmt.Sprintf("clients.%s.groups must match inventory", app.ID))
		}
		if !sameStringSet(candidate.RedirectURIs, app.RedirectURIs) {
			problems = append(problems, fmt.Sprintf("clients.%s.redirectUris must match inventory", app.ID))
		}
		if !sameStringSet(candidate.GrantTypes, app.GrantTypes) {
			problems = append(problems, fmt.Sprintf("clients.%s.grantTypes must match inventory", app.ID))
		}
	}
	return problems
}

func validateIdentityGroupCandidates(candidates []identityGroupCandidate, groups []inventory.IdentityGroup) []string {
	var problems []string
	byID := map[string]identityGroupCandidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	if len(candidates) != len(groups) {
		problems = append(problems, fmt.Sprintf("groups count %d does not match inventory count %d", len(candidates), len(groups)))
	}
	for _, group := range groups {
		candidate, ok := byID[group.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf("groups: missing candidate for %s", group.ID))
			continue
		}
		if candidate.Name != group.Name || candidate.Members != group.Members || candidate.EvidenceRef != group.EvidenceRef {
			problems = append(problems, fmt.Sprintf("groups.%s must preserve name, member count, and evidenceRef", group.ID))
		}
		if !sameStringSet(candidate.Owners, group.Owners) {
			problems = append(problems, fmt.Sprintf("groups.%s.owners must match inventory", group.ID))
		}
	}
	return problems
}

func validateIdentityPolicyCandidates(candidates []identityPolicyCandidate, policies []inventory.IdentityPolicy) []string {
	var problems []string
	byID := map[string]identityPolicyCandidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	if len(candidates) != len(policies) {
		problems = append(problems, fmt.Sprintf("policies count %d does not match inventory count %d", len(candidates), len(policies)))
	}
	for _, policy := range policies {
		candidate, ok := byID[policy.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf("policies: missing candidate for %s", policy.ID))
			continue
		}
		if candidate.Name != policy.Name || candidate.Type != policy.Type || candidate.EnforcesMFA != policy.EnforcesMFA || candidate.Priority != policy.Priority || candidate.EvidenceRef != policy.EvidenceRef {
			problems = append(problems, fmt.Sprintf("policies.%s must preserve name, type, MFA, priority, and evidenceRef", policy.ID))
		}
		if !sameStringSet(candidate.Groups, policy.Groups) {
			problems = append(problems, fmt.Sprintf("policies.%s.groups must match inventory", policy.ID))
		}
		if !sameStringSet(candidate.Applications, policy.Applications) {
			problems = append(problems, fmt.Sprintf("policies.%s.applications must match inventory", policy.ID))
		}
	}
	return problems
}

func validateIdentityMFACandidates(candidates []identityMFACandidate, settings []inventory.MFASetting) []string {
	var problems []string
	byName := map[string]identityMFACandidate{}
	for _, candidate := range candidates {
		byName[candidate.Name] = candidate
	}
	if len(candidates) != len(settings) {
		problems = append(problems, fmt.Sprintf("mfaSettings count %d does not match inventory count %d", len(candidates), len(settings)))
	}
	for _, setting := range settings {
		candidate, ok := byName[setting.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("mfaSettings: missing candidate for %s", setting.Name))
			continue
		}
		if candidate.Required != setting.Required || candidate.EvidenceRef != setting.EvidenceRef {
			problems = append(problems, fmt.Sprintf("mfaSettings.%s must preserve required state and evidenceRef", setting.Name))
		}
		if !sameStringSet(candidate.Factors, setting.Factors) {
			problems = append(problems, fmt.Sprintf("mfaSettings.%s.factors must match inventory", setting.Name))
		}
	}
	return problems
}

func validateIdentityBreakGlassCandidates(candidates []identityBreakGlassCandidate, accounts []inventory.BreakGlassAccount) []string {
	var problems []string
	byUsername := map[string]identityBreakGlassCandidate{}
	for _, candidate := range candidates {
		byUsername[candidate.Username] = candidate
	}
	if len(candidates) != len(accounts) {
		problems = append(problems, fmt.Sprintf("breakGlassAccounts count %d does not match inventory count %d", len(candidates), len(accounts)))
	}
	for _, account := range accounts {
		candidate, ok := byUsername[account.Username]
		if !ok {
			problems = append(problems, fmt.Sprintf("breakGlassAccounts: missing candidate for %s", account.Username))
			continue
		}
		if candidate.MFAEnabled != account.MFAEnabled || candidate.ManagedBy != account.ManagedBy || candidate.EvidenceRef != account.EvidenceRef {
			problems = append(problems, fmt.Sprintf("breakGlassAccounts.%s must preserve MFA state, manager, and evidenceRef", account.Username))
		}
	}
	return problems
}

func validateIdentityCandidateREADME(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "identity", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/identity/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"deterministic Keycloak/Zitadel candidate", "no client secrets", "not production-ready", "Review every client", "break-glass account"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/identity/README.md: missing %q", required)}
		}
	}
	return nil
}
