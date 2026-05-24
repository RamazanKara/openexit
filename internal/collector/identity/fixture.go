package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
	"gopkg.in/yaml.v3"
)

type FixtureCollector struct{}

func (FixtureCollector) Name() string { return "identity-fixture" }

func (FixtureCollector) Collect(_ context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	input := req.Options["input"]
	if input == "" {
		return nil, fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	data = inventory.RedactBytes(data)
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	inv := inventory.New(req.Project, "identity", fixture.Provider, version.Version, time.Now().UTC())
	if fixture.Tenant != "" {
		inv.Source.Site = fixture.Provider + "/" + fixture.Tenant
	}
	if err := normalizeFixture(req.ProjectDir, &fixture, inv, data); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func normalizeFixture(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte) error {
	if err := writeEvidence(projectDir, "identity/raw-fixture.json", raw); err != nil {
		return err
	}
	for _, app := range fixture.Applications {
		evidenceID := safeID(app.ID)
		if err := writeJSONEvidence(projectDir, "identity/applications/"+evidenceID+".json", app); err != nil {
			return err
		}
		inv.Assets.IdentityApps = append(inv.Assets.IdentityApps, inventory.IdentityApp{
			ID:                       app.ID,
			Name:                     app.Name,
			Protocol:                 app.Protocol,
			ClientID:                 app.ClientID,
			Owners:                   append([]string{}, app.Owners...),
			Groups:                   append([]string{}, app.Groups...),
			RedirectURIs:             append([]string{}, app.RedirectURIs...),
			GrantTypes:               append([]string{}, app.GrantTypes...),
			SAMLSigningCertPresent:   app.SAMLSigningCertPresent,
			SAMLSigningCertExpiresAt: app.SAMLSigningCertExpiresAt,
			EvidenceRef:              "evidence://identity/application/" + evidenceID,
		})
	}
	for _, group := range fixture.Groups {
		evidenceID := safeID(group.ID)
		if err := writeJSONEvidence(projectDir, "identity/groups/"+evidenceID+".json", group); err != nil {
			return err
		}
		inv.Assets.IdentityGroups = append(inv.Assets.IdentityGroups, inventory.IdentityGroup{
			ID:          group.ID,
			Name:        group.Name,
			Owners:      append([]string{}, group.Owners...),
			Members:     group.Members,
			EvidenceRef: "evidence://identity/group/" + evidenceID,
		})
	}
	for _, policy := range fixture.Policies {
		evidenceID := safeID(policy.ID)
		if err := writeJSONEvidence(projectDir, "identity/policies/"+evidenceID+".json", policy); err != nil {
			return err
		}
		inv.Assets.IdentityPolicies = append(inv.Assets.IdentityPolicies, inventory.IdentityPolicy{
			ID:           policy.ID,
			Name:         policy.Name,
			Type:         policy.Type,
			Groups:       append([]string{}, policy.Groups...),
			Applications: append([]string{}, policy.Applications...),
			EnforcesMFA:  policy.EnforcesMFA,
			Priority:     policy.Priority,
			EvidenceRef:  "evidence://identity/policy/" + evidenceID,
		})
	}
	for _, setting := range fixture.MFASettings {
		evidenceID := safeID(setting.Name)
		if err := writeJSONEvidence(projectDir, "identity/mfa-settings/"+evidenceID+".json", setting); err != nil {
			return err
		}
		inv.Assets.MFASettings = append(inv.Assets.MFASettings, inventory.MFASetting{
			Name:        setting.Name,
			Required:    setting.Required,
			Factors:     append([]string{}, setting.Factors...),
			EvidenceRef: "evidence://identity/mfa-setting/" + evidenceID,
		})
	}
	for _, account := range fixture.BreakGlassAccounts {
		evidenceID := safeID(account.Username)
		if err := writeJSONEvidence(projectDir, "identity/break-glass-accounts/"+evidenceID+".json", account); err != nil {
			return err
		}
		inv.Assets.BreakGlassAccounts = append(inv.Assets.BreakGlassAccounts, inventory.BreakGlassAccount{
			Username:    account.Username,
			MFAEnabled:  account.MFAEnabled,
			ManagedBy:   account.ManagedBy,
			EvidenceRef: "evidence://identity/break-glass-account/" + evidenceID,
		})
	}
	sortInventory(inv)
	inv.RecomputeSummary()
	return inventory.Validate(inv)
}

func writeInventory(projectDir string, inv *inventory.Inventory) error {
	invDir := filepath.Join(projectDir, "inventory")
	if err := os.MkdirAll(invDir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(inv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(invDir, "openexit.inventory.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(invDir, "openexit.inventory.json"), jsonData, 0o644)
}

func writeJSONEvidence(projectDir, rel string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeEvidence(projectDir, rel, inventory.RedactBytes(data))
}

func writeEvidence(projectDir, rel string, data []byte) error {
	path := filepath.Join(projectDir, "evidence", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func sortInventory(inv *inventory.Inventory) {
	sort.Slice(inv.Assets.IdentityApps, func(i, j int) bool { return inv.Assets.IdentityApps[i].ID < inv.Assets.IdentityApps[j].ID })
	sort.Slice(inv.Assets.IdentityGroups, func(i, j int) bool { return inv.Assets.IdentityGroups[i].ID < inv.Assets.IdentityGroups[j].ID })
	sort.Slice(inv.Assets.IdentityPolicies, func(i, j int) bool { return inv.Assets.IdentityPolicies[i].ID < inv.Assets.IdentityPolicies[j].ID })
	sort.Slice(inv.Assets.MFASettings, func(i, j int) bool { return inv.Assets.MFASettings[i].Name < inv.Assets.MFASettings[j].Name })
	sort.Slice(inv.Assets.BreakGlassAccounts, func(i, j int) bool {
		return inv.Assets.BreakGlassAccounts[i].Username < inv.Assets.BreakGlassAccounts[j].Username
	})
}

func safeID(value string) string {
	return inventory.Slug(value)
}
