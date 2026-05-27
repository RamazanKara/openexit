package githubenterprise

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

func (FixtureCollector) Name() string { return "github-enterprise-fixture" }

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
	inv := inventory.New(req.Project, "github-enterprise", fixture.Host, version.Version, time.Now().UTC())
	if err := normalizeFixture(req.ProjectDir, &fixture, inv, data); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func normalizeFixture(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte) error {
	return normalizeFixtureWithRawPath(projectDir, fixture, inv, raw, "github-enterprise/raw-fixture.json")
}

func normalizeFixtureWithRawPath(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte, rawPath string) error {
	if rawPath != "" && len(raw) > 0 {
		if err := writeEvidence(projectDir, rawPath, raw); err != nil {
			return err
		}
	}
	if rawPath == "" && len(raw) > 0 {
		return fmt.Errorf("raw evidence path is required when raw evidence is provided")
	}
	for _, repo := range fixture.Repositories {
		evidenceID := safeID(repo.Name)
		if err := writeJSONEvidence(projectDir, "github-enterprise/repositories/"+evidenceID+".json", repo); err != nil {
			return err
		}
		inv.Assets.Repositories = append(inv.Assets.Repositories, inventory.Repository{
			Name:                  repo.Name,
			Visibility:            repo.Visibility,
			DefaultBranch:         repo.DefaultBranch,
			Archived:              repo.Archived,
			Topics:                append([]string{}, repo.Topics...),
			Teams:                 append([]string{}, repo.Teams...),
			ActionsEnabled:        repo.ActionsEnabled,
			HasCODEOWNERS:         repo.HasCODEOWNERS,
			UsesGitHubPages:       repo.UsesGitHubPages,
			UsesGitHubPackages:    repo.UsesGitHubPackages,
			UsesGitHubDiscussions: repo.UsesGitHubDiscussions,
			EvidenceRef:           "evidence://github-enterprise/repository/" + evidenceID,
		})
	}
	for _, team := range fixture.Teams {
		evidenceID := safeID(team.Slug)
		if err := writeJSONEvidence(projectDir, "github-enterprise/teams/"+evidenceID+".json", team); err != nil {
			return err
		}
		inv.Assets.Teams = append(inv.Assets.Teams, inventory.Team{
			Slug:        team.Slug,
			Name:        team.Name,
			Members:     team.Members,
			Maintainers: team.Maintainers,
			Repos:       append([]string{}, team.Repos...),
			EvidenceRef: "evidence://github-enterprise/team/" + evidenceID,
		})
	}
	for _, protection := range fixture.BranchProtections {
		evidenceID := safeID(protection.Repository + "-" + protection.Branch)
		if err := writeJSONEvidence(projectDir, "github-enterprise/branch-protections/"+evidenceID+".json", protection); err != nil {
			return err
		}
		inv.Assets.BranchProtections = append(inv.Assets.BranchProtections, inventory.BranchProtection{
			Repository:             protection.Repository,
			Branch:                 protection.Branch,
			RequiredStatusChecks:   append([]string{}, protection.RequiredStatusChecks...),
			RequiredReviews:        protection.RequiredReviews,
			RequireCodeOwnerReview: protection.RequireCodeOwnerReview,
			RestrictPushes:         protection.RestrictPushes,
			AllowsForcePushes:      protection.AllowsForcePushes,
			EvidenceRef:            "evidence://github-enterprise/branch-protection/" + evidenceID,
		})
	}
	for _, workflow := range fixture.ActionsWorkflows {
		evidenceID := safeID(workflow.Repository + "-" + workflow.Path)
		if err := writeJSONEvidence(projectDir, "github-enterprise/actions-workflows/"+evidenceID+".json", workflow); err != nil {
			return err
		}
		inv.Assets.ActionsWorkflows = append(inv.Assets.ActionsWorkflows, inventory.ActionsWorkflow{
			Repository:       workflow.Repository,
			Path:             workflow.Path,
			Name:             workflow.Name,
			UsesGitHubHosted: workflow.UsesGitHubHosted,
			UsesSelfHosted:   workflow.UsesSelfHosted,
			Actions:          append([]string{}, workflow.Actions...),
			Secrets:          append([]string{}, workflow.Secrets...),
			EvidenceRef:      "evidence://github-enterprise/actions-workflow/" + evidenceID,
		})
	}
	for _, secret := range fixture.Secrets {
		evidenceID := safeID(secret.Scope + "-" + secret.Repository + "-" + secret.Name)
		if err := writeJSONEvidence(projectDir, "github-enterprise/secrets/"+evidenceID+".json", secret); err != nil {
			return err
		}
		inv.Assets.Secrets = append(inv.Assets.Secrets, inventory.SecretMetadata{
			Name:        secret.Name,
			Scope:       secret.Scope,
			Repository:  secret.Repository,
			Consumers:   append([]string{}, secret.Consumers...),
			EvidenceRef: "evidence://github-enterprise/secret/" + evidenceID,
		})
	}
	for _, runner := range fixture.Runners {
		evidenceID := safeID(runner.Scope + "-" + runner.Name)
		if err := writeJSONEvidence(projectDir, "github-enterprise/runners/"+evidenceID+".json", runner); err != nil {
			return err
		}
		inv.Assets.Runners = append(inv.Assets.Runners, inventory.Runner{
			Name:        runner.Name,
			Scope:       runner.Scope,
			Labels:      append([]string{}, runner.Labels...),
			Online:      runner.Online,
			EvidenceRef: "evidence://github-enterprise/runner/" + evidenceID,
		})
	}
	for _, key := range fixture.DeployKeys {
		evidenceID := safeID(key.Repository + "-" + key.Title)
		if err := writeJSONEvidence(projectDir, "github-enterprise/deploy-keys/"+evidenceID+".json", key); err != nil {
			return err
		}
		inv.Assets.DeployKeys = append(inv.Assets.DeployKeys, inventory.DeployKey{
			Repository:  key.Repository,
			Title:       key.Title,
			ReadOnly:    key.ReadOnly,
			EvidenceRef: "evidence://github-enterprise/deploy-key/" + evidenceID,
		})
	}
	for _, app := range fixture.GitHubApps {
		evidenceID := safeID(app.Name)
		if err := writeJSONEvidence(projectDir, "github-enterprise/github-apps/"+evidenceID+".json", app); err != nil {
			return err
		}
		inv.Assets.GitHubApps = append(inv.Assets.GitHubApps, inventory.GitHubApp{
			Name:                app.Name,
			Repositories:        append([]string{}, app.Repositories...),
			RepositorySelection: app.RepositorySelection,
			Permissions:         append([]string{}, app.Permissions...),
			Events:              append([]string{}, app.Events...),
			WebhookEnabled:      app.WebhookEnabled,
			EvidenceRef:         "evidence://github-enterprise/github-app/" + evidenceID,
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
	sort.Slice(inv.Assets.Repositories, func(i, j int) bool { return inv.Assets.Repositories[i].Name < inv.Assets.Repositories[j].Name })
	sort.Slice(inv.Assets.Teams, func(i, j int) bool { return inv.Assets.Teams[i].Slug < inv.Assets.Teams[j].Slug })
	sort.Slice(inv.Assets.BranchProtections, func(i, j int) bool {
		left := inv.Assets.BranchProtections[i].Repository + inv.Assets.BranchProtections[i].Branch
		right := inv.Assets.BranchProtections[j].Repository + inv.Assets.BranchProtections[j].Branch
		return left < right
	})
	sort.Slice(inv.Assets.ActionsWorkflows, func(i, j int) bool {
		left := inv.Assets.ActionsWorkflows[i].Repository + inv.Assets.ActionsWorkflows[i].Path
		right := inv.Assets.ActionsWorkflows[j].Repository + inv.Assets.ActionsWorkflows[j].Path
		return left < right
	})
	sort.Slice(inv.Assets.Secrets, func(i, j int) bool {
		left := inv.Assets.Secrets[i].Scope + inv.Assets.Secrets[i].Repository + inv.Assets.Secrets[i].Name
		right := inv.Assets.Secrets[j].Scope + inv.Assets.Secrets[j].Repository + inv.Assets.Secrets[j].Name
		return left < right
	})
	sort.Slice(inv.Assets.Runners, func(i, j int) bool {
		left := inv.Assets.Runners[i].Scope + inv.Assets.Runners[i].Name
		right := inv.Assets.Runners[j].Scope + inv.Assets.Runners[j].Name
		return left < right
	})
	sort.Slice(inv.Assets.DeployKeys, func(i, j int) bool {
		left := inv.Assets.DeployKeys[i].Repository + inv.Assets.DeployKeys[i].Title
		right := inv.Assets.DeployKeys[j].Repository + inv.Assets.DeployKeys[j].Title
		return left < right
	})
	sort.Slice(inv.Assets.GitHubApps, func(i, j int) bool { return inv.Assets.GitHubApps[i].Name < inv.Assets.GitHubApps[j].Name })
}

func safeID(value string) string {
	return inventory.Slug(value)
}
