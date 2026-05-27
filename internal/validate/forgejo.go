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

type forgejoMigrationCandidate struct {
	APIVersion          string                         `yaml:"apiVersion"`
	Kind                string                         `yaml:"kind"`
	Metadata            forgejoCandidateMetadata       `yaml:"metadata"`
	Repositories        []forgejoRepositoryCandidate   `yaml:"repositories"`
	Teams               []forgejoTeamCandidate         `yaml:"teams"`
	BranchProtections   []forgejoBranchCandidate       `yaml:"branchProtections"`
	CIWorkflows         []forgejoWorkflowCandidate     `yaml:"ciWorkflows"`
	SecretMetadata      []forgejoSecretCandidate       `yaml:"secretMetadata"`
	RunnerMigration     []forgejoRunnerCandidate       `yaml:"runnerMigration"`
	DeployKeyReview     []forgejoDeployKeyCandidate    `yaml:"deployKeyReview"`
	IntegrationReview   []forgejoAppCandidate          `yaml:"integrationReview"`
	ManualReview        []forgejoManualReviewCandidate `yaml:"manualReview"`
	CredentialsIncluded bool                           `yaml:"credentialsIncluded"`
	ProductionReady     bool                           `yaml:"productionReady"`
	rawText             string                         `yaml:"-"`
}

type forgejoCandidateMetadata struct {
	Project    string `yaml:"project"`
	Source     string `yaml:"source"`
	SourceHost string `yaml:"sourceHost"`
	Target     string `yaml:"target"`
}

type forgejoRepositoryCandidate struct {
	Name                  string                     `yaml:"name"`
	TargetOwner           string                     `yaml:"targetOwner"`
	TargetName            string                     `yaml:"targetName"`
	Visibility            string                     `yaml:"visibility"`
	DefaultBranch         string                     `yaml:"defaultBranch"`
	Archived              bool                       `yaml:"archived"`
	Topics                []string                   `yaml:"topics"`
	Teams                 []string                   `yaml:"teams"`
	ActionsEnabled        bool                       `yaml:"actionsEnabled"`
	HasCODEOWNERS         bool                       `yaml:"hasCodeowners"`
	UsesGitHubPages       bool                       `yaml:"usesGitHubPages"`
	UsesGitHubPackages    bool                       `yaml:"usesGitHubPackages"`
	UsesGitHubDiscussions bool                       `yaml:"usesGitHubDiscussions"`
	SourceCloneRef        string                     `yaml:"sourceCloneRef"`
	TargetRepository      string                     `yaml:"targetRepository"`
	BranchProtections     []forgejoBranchCandidate   `yaml:"branchProtections"`
	CIWorkflows           []forgejoWorkflowCandidate `yaml:"ciWorkflows"`
	SecretMetadataCount   int                        `yaml:"secretMetadataCount"`
	ReviewRequired        bool                       `yaml:"reviewRequired"`
	EvidenceRef           string                     `yaml:"evidenceRef"`
}

type forgejoTeamCandidate struct {
	Slug         string   `yaml:"slug"`
	Name         string   `yaml:"name"`
	Members      int      `yaml:"members"`
	Maintainers  int      `yaml:"maintainers"`
	Repositories []string `yaml:"repositories"`
	EvidenceRef  string   `yaml:"evidenceRef"`
}

type forgejoBranchCandidate struct {
	Repository             string   `yaml:"repository,omitempty"`
	Branch                 string   `yaml:"branch"`
	RequiredStatusChecks   []string `yaml:"requiredStatusChecks"`
	RequiredReviews        int      `yaml:"requiredReviews"`
	RequireCodeOwnerReview bool     `yaml:"requireCodeOwnerReview"`
	RestrictPushes         bool     `yaml:"restrictPushes"`
	AllowsForcePushes      bool     `yaml:"allowsForcePushes"`
	TargetReview           bool     `yaml:"targetReview,omitempty"`
	EvidenceRef            string   `yaml:"evidenceRef"`
}

type forgejoWorkflowCandidate struct {
	Repository       string   `yaml:"repository,omitempty"`
	Path             string   `yaml:"path"`
	Name             string   `yaml:"name"`
	UsesGitHubHosted bool     `yaml:"usesGitHubHosted"`
	UsesSelfHosted   bool     `yaml:"usesSelfHosted"`
	Actions          []string `yaml:"actions"`
	SecretNames      []string `yaml:"secretNames,omitempty"`
	TargetRunnerPlan string   `yaml:"targetRunnerPlan,omitempty"`
	EvidenceRef      string   `yaml:"evidenceRef"`
}

type forgejoSecretCandidate struct {
	Name        string   `yaml:"name"`
	Scope       string   `yaml:"scope"`
	Repository  string   `yaml:"repository"`
	Consumers   []string `yaml:"consumers"`
	ValueStatus string   `yaml:"valueStatus"`
	EvidenceRef string   `yaml:"evidenceRef"`
}

type forgejoRunnerCandidate struct {
	Name            string   `yaml:"name"`
	Scope           string   `yaml:"scope"`
	Labels          []string `yaml:"labels"`
	Online          bool     `yaml:"online"`
	TargetAction    string   `yaml:"targetAction"`
	RegistrationRef string   `yaml:"registrationRef"`
	EvidenceRef     string   `yaml:"evidenceRef"`
}

type forgejoDeployKeyCandidate struct {
	Repository   string `yaml:"repository"`
	Title        string `yaml:"title"`
	ReadOnly     bool   `yaml:"readOnly"`
	TargetAction string `yaml:"targetAction"`
	EvidenceRef  string `yaml:"evidenceRef"`
}

type forgejoAppCandidate struct {
	Name                string   `yaml:"name"`
	Repositories        []string `yaml:"repositories"`
	RepositorySelection string   `yaml:"repositorySelection"`
	Permissions         []string `yaml:"permissions"`
	Events              []string `yaml:"events"`
	WebhookEnabled      bool     `yaml:"webhookEnabled"`
	TargetAction        string   `yaml:"targetAction"`
	EvidenceRef         string   `yaml:"evidenceRef"`
}

type forgejoManualReviewCandidate struct {
	ID             string   `yaml:"id"`
	Severity       string   `yaml:"severity"`
	Title          string   `yaml:"title"`
	AffectedAssets []string `yaml:"affectedAssets"`
	EvidenceRefs   []string `yaml:"evidenceRefs"`
	Recommendation string   `yaml:"recommendation"`
}

func addForgejoMigrationCandidateChecks(projectDir string, inv *inventory.Inventory, a *assessment.Assessment, add func(string, string, string, bool)) {
	if inv == nil || inv.Source.Type != "github-enterprise" {
		return
	}
	path := filepath.Join(projectDir, "generated-config", "forgejo", "migration-candidate.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		add("forgejo-migration-candidate", "failed", err.Error(), true)
		return
	}
	candidate, err := readForgejoMigrationCandidate(path)
	if err != nil {
		add("forgejo-migration-candidate", "failed", err.Error(), true)
		return
	}
	var problems []string
	problems = append(problems, validateForgejoCandidateShape(candidate, inv, a)...)
	problems = append(problems, validateForgejoRepositoryCandidates(candidate.Repositories, inv)...)
	problems = append(problems, validateForgejoTeamCandidates(candidate.Teams, inv.Assets.Teams)...)
	problems = append(problems, validateForgejoBranchCandidates(candidate.BranchProtections, inv.Assets.BranchProtections, true)...)
	problems = append(problems, validateForgejoWorkflowCandidates(candidate.CIWorkflows, inv.Assets.ActionsWorkflows, true)...)
	problems = append(problems, validateForgejoSecretCandidates(candidate.SecretMetadata, inv.Assets.Secrets)...)
	problems = append(problems, validateForgejoRunnerCandidates(candidate.RunnerMigration, inv.Assets.Runners)...)
	problems = append(problems, validateForgejoDeployKeyCandidates(candidate.DeployKeyReview, inv.Assets.DeployKeys)...)
	problems = append(problems, validateForgejoAppCandidates(candidate.IntegrationReview, inv.Assets.GitHubApps)...)
	problems = append(problems, validateForgejoManualReviewCandidates(candidate.ManualReview, a)...)
	problems = append(problems, validateForgejoCandidateREADME(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("forgejo-migration-candidate", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("forgejo-migration-candidate", "passed", "", true)
}

func readForgejoMigrationCandidate(path string) (*forgejoMigrationCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var candidate forgejoMigrationCandidate
	if err := yaml.Unmarshal(data, &candidate); err != nil {
		return nil, err
	}
	candidate.rawText = string(data)
	return &candidate, nil
}

func validateForgejoCandidateShape(candidate *forgejoMigrationCandidate, inv *inventory.Inventory, a *assessment.Assessment) []string {
	if candidate == nil {
		return []string{"Forgejo migration candidate is empty"}
	}
	var problems []string
	if inventory.ContainsSecret(candidate.rawText) {
		problems = append(problems, "migration-candidate.yaml contains secret-like content")
	}
	if candidate.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q, got %q", APIVersion, candidate.APIVersion))
	}
	if candidate.Kind != "ForgejoMigrationCandidate" {
		problems = append(problems, fmt.Sprintf("kind must be ForgejoMigrationCandidate, got %q", candidate.Kind))
	}
	if candidate.Metadata.Project != inv.Metadata.Project {
		problems = append(problems, fmt.Sprintf("metadata.project must be %q, got %q", inv.Metadata.Project, candidate.Metadata.Project))
	}
	if candidate.Metadata.Source != inv.Source.Type {
		problems = append(problems, fmt.Sprintf("metadata.source must be %q, got %q", inv.Source.Type, candidate.Metadata.Source))
	}
	if candidate.Metadata.SourceHost != inv.Source.Site {
		problems = append(problems, fmt.Sprintf("metadata.sourceHost must be %q, got %q", inv.Source.Site, candidate.Metadata.SourceHost))
	}
	if a != nil && candidate.Metadata.Target != a.Target.Type {
		problems = append(problems, fmt.Sprintf("metadata.target must be %q, got %q", a.Target.Type, candidate.Metadata.Target))
	}
	if candidate.CredentialsIncluded {
		problems = append(problems, "credentialsIncluded must be false")
	}
	if candidate.ProductionReady {
		problems = append(problems, "productionReady must be false")
	}
	return problems
}

func validateForgejoRepositoryCandidates(candidates []forgejoRepositoryCandidate, inv *inventory.Inventory) []string {
	var problems []string
	byName := mapForgejoRepositories(candidates)
	if len(candidates) != len(inv.Assets.Repositories) {
		problems = append(problems, fmt.Sprintf("repositories count %d does not match inventory count %d", len(candidates), len(inv.Assets.Repositories)))
	}
	protections := branchProtectionsByRepo(inv.Assets.BranchProtections)
	workflows := workflowsByRepo(inv.Assets.ActionsWorkflows)
	secretCounts := secretCountsByRepo(inv.Assets.Secrets)
	for _, repo := range inv.Assets.Repositories {
		candidate, ok := byName[repo.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("repositories: missing candidate for %s", repo.Name))
			continue
		}
		owner, name := splitRepositoryName(repo.Name)
		expectedStrings := map[string][2]string{
			"targetOwner":      {owner, candidate.TargetOwner},
			"targetName":       {name, candidate.TargetName},
			"visibility":       {repo.Visibility, candidate.Visibility},
			"defaultBranch":    {repo.DefaultBranch, candidate.DefaultBranch},
			"sourceCloneRef":   {sourceCloneRef(inv.Source.Site, repo.Name), candidate.SourceCloneRef},
			"targetRepository": {repo.Name, candidate.TargetRepository},
			"evidenceRef":      {repo.EvidenceRef, candidate.EvidenceRef},
		}
		for field, values := range expectedStrings {
			if values[0] != values[1] {
				problems = append(problems, fmt.Sprintf("repositories.%s.%s must be %q, got %q", repo.Name, field, values[0], values[1]))
			}
		}
		if candidate.Archived != repo.Archived {
			problems = append(problems, fmt.Sprintf("repositories.%s.archived must be %t", repo.Name, repo.Archived))
		}
		if candidate.ActionsEnabled != repo.ActionsEnabled {
			problems = append(problems, fmt.Sprintf("repositories.%s.actionsEnabled must be %t", repo.Name, repo.ActionsEnabled))
		}
		if candidate.HasCODEOWNERS != repo.HasCODEOWNERS {
			problems = append(problems, fmt.Sprintf("repositories.%s.hasCodeowners must be %t", repo.Name, repo.HasCODEOWNERS))
		}
		if candidate.UsesGitHubPages != repo.UsesGitHubPages || candidate.UsesGitHubPackages != repo.UsesGitHubPackages || candidate.UsesGitHubDiscussions != repo.UsesGitHubDiscussions {
			problems = append(problems, fmt.Sprintf("repositories.%s must preserve GitHub feature flags", repo.Name))
		}
		if !sameStringSet(candidate.Topics, repo.Topics) {
			problems = append(problems, fmt.Sprintf("repositories.%s.topics must match inventory", repo.Name))
		}
		if !sameStringSet(candidate.Teams, repo.Teams) {
			problems = append(problems, fmt.Sprintf("repositories.%s.teams must match inventory", repo.Name))
		}
		if candidate.SecretMetadataCount != secretCounts[repo.Name] {
			problems = append(problems, fmt.Sprintf("repositories.%s.secretMetadataCount must be %d, got %d", repo.Name, secretCounts[repo.Name], candidate.SecretMetadataCount))
		}
		if candidate.ReviewRequired != forgejoRepositoryReviewRequired(repo) {
			problems = append(problems, fmt.Sprintf("repositories.%s.reviewRequired must be %t", repo.Name, forgejoRepositoryReviewRequired(repo)))
		}
		nestedProtections := candidate.BranchProtections
		for i := range nestedProtections {
			if nestedProtections[i].Repository == "" {
				nestedProtections[i].Repository = repo.Name
			}
		}
		nestedWorkflows := candidate.CIWorkflows
		for i := range nestedWorkflows {
			if nestedWorkflows[i].Repository == "" {
				nestedWorkflows[i].Repository = repo.Name
			}
		}
		problems = append(problems, validateForgejoBranchCandidates(nestedProtections, protections[repo.Name], false)...)
		problems = append(problems, validateForgejoWorkflowCandidates(nestedWorkflows, workflows[repo.Name], false)...)
	}
	return problems
}

func validateForgejoTeamCandidates(candidates []forgejoTeamCandidate, teams []inventory.Team) []string {
	var problems []string
	bySlug := map[string]forgejoTeamCandidate{}
	for _, candidate := range candidates {
		bySlug[candidate.Slug] = candidate
	}
	if len(candidates) != len(teams) {
		problems = append(problems, fmt.Sprintf("teams count %d does not match inventory count %d", len(candidates), len(teams)))
	}
	for _, team := range teams {
		candidate, ok := bySlug[team.Slug]
		if !ok {
			problems = append(problems, fmt.Sprintf("teams: missing candidate for %s", team.Slug))
			continue
		}
		if candidate.Name != team.Name || candidate.Members != team.Members || candidate.Maintainers != team.Maintainers || candidate.EvidenceRef != team.EvidenceRef {
			problems = append(problems, fmt.Sprintf("teams.%s must preserve name, member counts, and evidenceRef", team.Slug))
		}
		if !sameStringSet(candidate.Repositories, team.Repos) {
			problems = append(problems, fmt.Sprintf("teams.%s.repositories must match inventory", team.Slug))
		}
	}
	return problems
}

func validateForgejoBranchCandidates(candidates []forgejoBranchCandidate, protections []inventory.BranchProtection, requireRepository bool) []string {
	var problems []string
	byKey := mapForgejoBranches(candidates)
	if len(candidates) != len(protections) {
		problems = append(problems, fmt.Sprintf("branchProtections count %d does not match inventory count %d", len(candidates), len(protections)))
	}
	for _, protection := range protections {
		candidate, ok := byKey[branchKey(protection.Repository, protection.Branch)]
		if !ok {
			problems = append(problems, fmt.Sprintf("branchProtections: missing candidate for %s/%s", protection.Repository, protection.Branch))
			continue
		}
		if requireRepository && candidate.Repository != protection.Repository {
			problems = append(problems, fmt.Sprintf("branchProtections.%s/%s.repository must be %q", protection.Repository, protection.Branch, protection.Repository))
		}
		if candidate.RequiredReviews != protection.RequiredReviews || candidate.RequireCodeOwnerReview != protection.RequireCodeOwnerReview || candidate.RestrictPushes != protection.RestrictPushes || candidate.AllowsForcePushes != protection.AllowsForcePushes {
			problems = append(problems, fmt.Sprintf("branchProtections.%s/%s must preserve branch protection settings", protection.Repository, protection.Branch))
		}
		if !sameStringSet(candidate.RequiredStatusChecks, protection.RequiredStatusChecks) {
			problems = append(problems, fmt.Sprintf("branchProtections.%s/%s.requiredStatusChecks must match inventory", protection.Repository, protection.Branch))
		}
		if requireRepository && candidate.TargetReview != (!protection.RequireCodeOwnerReview || protection.AllowsForcePushes || protection.RequiredReviews == 0) {
			problems = append(problems, fmt.Sprintf("branchProtections.%s/%s.targetReview has wrong value", protection.Repository, protection.Branch))
		}
		if candidate.EvidenceRef != protection.EvidenceRef {
			problems = append(problems, fmt.Sprintf("branchProtections.%s/%s.evidenceRef must be %q", protection.Repository, protection.Branch, protection.EvidenceRef))
		}
	}
	return problems
}

func validateForgejoWorkflowCandidates(candidates []forgejoWorkflowCandidate, workflows []inventory.ActionsWorkflow, requireRepository bool) []string {
	var problems []string
	byKey := mapForgejoWorkflows(candidates)
	if len(candidates) != len(workflows) {
		problems = append(problems, fmt.Sprintf("ciWorkflows count %d does not match inventory count %d", len(candidates), len(workflows)))
	}
	for _, workflow := range workflows {
		candidate, ok := byKey[workflowKey(workflow.Repository, workflow.Path)]
		if !ok {
			problems = append(problems, fmt.Sprintf("ciWorkflows: missing candidate for %s:%s", workflow.Repository, workflow.Path))
			continue
		}
		if requireRepository && candidate.Repository != workflow.Repository {
			problems = append(problems, fmt.Sprintf("ciWorkflows.%s:%s.repository must be %q", workflow.Repository, workflow.Path, workflow.Repository))
		}
		if candidate.Name != workflow.Name || candidate.UsesGitHubHosted != workflow.UsesGitHubHosted || candidate.UsesSelfHosted != workflow.UsesSelfHosted || candidate.EvidenceRef != workflow.EvidenceRef {
			problems = append(problems, fmt.Sprintf("ciWorkflows.%s:%s must preserve workflow metadata", workflow.Repository, workflow.Path))
		}
		if !sameStringSet(candidate.Actions, workflow.Actions) {
			problems = append(problems, fmt.Sprintf("ciWorkflows.%s:%s.actions must match inventory", workflow.Repository, workflow.Path))
		}
		if requireRepository && !sameStringSet(candidate.SecretNames, workflow.Secrets) {
			problems = append(problems, fmt.Sprintf("ciWorkflows.%s:%s.secretNames must match inventory metadata", workflow.Repository, workflow.Path))
		}
		if requireRepository && candidate.TargetRunnerPlan != forgejoRunnerPlan(workflow) {
			problems = append(problems, fmt.Sprintf("ciWorkflows.%s:%s.targetRunnerPlan must be %q", workflow.Repository, workflow.Path, forgejoRunnerPlan(workflow)))
		}
	}
	return problems
}

func validateForgejoSecretCandidates(candidates []forgejoSecretCandidate, secrets []inventory.SecretMetadata) []string {
	var problems []string
	byKey := map[string]forgejoSecretCandidate{}
	for _, candidate := range candidates {
		byKey[secretKey(candidate.Scope, candidate.Repository, candidate.Name)] = candidate
	}
	if len(candidates) != len(secrets) {
		problems = append(problems, fmt.Sprintf("secretMetadata count %d does not match inventory count %d", len(candidates), len(secrets)))
	}
	for _, secret := range secrets {
		candidate, ok := byKey[secretKey(secret.Scope, secret.Repository, secret.Name)]
		if !ok {
			problems = append(problems, fmt.Sprintf("secretMetadata: missing candidate for %s", secret.Name))
			continue
		}
		if candidate.ValueStatus != "not-collected" {
			problems = append(problems, fmt.Sprintf("secretMetadata.%s.valueStatus must be not-collected", secret.Name))
		}
		if !sameStringSet(candidate.Consumers, secret.Consumers) {
			problems = append(problems, fmt.Sprintf("secretMetadata.%s.consumers must match inventory metadata", secret.Name))
		}
		if candidate.EvidenceRef != secret.EvidenceRef {
			problems = append(problems, fmt.Sprintf("secretMetadata.%s.evidenceRef must be %q", secret.Name, secret.EvidenceRef))
		}
	}
	return problems
}

func validateForgejoRunnerCandidates(candidates []forgejoRunnerCandidate, runners []inventory.Runner) []string {
	var problems []string
	byKey := map[string]forgejoRunnerCandidate{}
	for _, candidate := range candidates {
		byKey[runnerKey(candidate.Scope, candidate.Name)] = candidate
	}
	if len(candidates) != len(runners) {
		problems = append(problems, fmt.Sprintf("runnerMigration count %d does not match inventory count %d", len(candidates), len(runners)))
	}
	for _, runner := range runners {
		candidate, ok := byKey[runnerKey(runner.Scope, runner.Name)]
		if !ok {
			problems = append(problems, fmt.Sprintf("runnerMigration: missing candidate for %s", runner.Name))
			continue
		}
		if candidate.Online != runner.Online || candidate.EvidenceRef != runner.EvidenceRef {
			problems = append(problems, fmt.Sprintf("runnerMigration.%s must preserve online state and evidenceRef", runner.Name))
		}
		if !sameStringSet(candidate.Labels, runner.Labels) {
			problems = append(problems, fmt.Sprintf("runnerMigration.%s.labels must match inventory", runner.Name))
		}
		if candidate.TargetAction != forgejoRunnerAction(runner) {
			problems = append(problems, fmt.Sprintf("runnerMigration.%s.targetAction must be %q", runner.Name, forgejoRunnerAction(runner)))
		}
		if candidate.RegistrationRef != "create-runner-registration-during-cutover" {
			problems = append(problems, fmt.Sprintf("runnerMigration.%s.registrationRef must remain the cutover placeholder", runner.Name))
		}
	}
	return problems
}

func validateForgejoDeployKeyCandidates(candidates []forgejoDeployKeyCandidate, keys []inventory.DeployKey) []string {
	var problems []string
	byKey := map[string]forgejoDeployKeyCandidate{}
	for _, candidate := range candidates {
		byKey[deployKeyKey(candidate.Repository, candidate.Title)] = candidate
	}
	if len(candidates) != len(keys) {
		problems = append(problems, fmt.Sprintf("deployKeyReview count %d does not match inventory count %d", len(candidates), len(keys)))
	}
	for _, key := range keys {
		candidate, ok := byKey[deployKeyKey(key.Repository, key.Title)]
		if !ok {
			problems = append(problems, fmt.Sprintf("deployKeyReview: missing candidate for %s:%s", key.Repository, key.Title))
			continue
		}
		if candidate.ReadOnly != key.ReadOnly || candidate.EvidenceRef != key.EvidenceRef {
			problems = append(problems, fmt.Sprintf("deployKeyReview.%s:%s must preserve readOnly and evidenceRef", key.Repository, key.Title))
		}
		if candidate.TargetAction != forgejoDeployKeyAction(key) {
			problems = append(problems, fmt.Sprintf("deployKeyReview.%s:%s.targetAction must be %q", key.Repository, key.Title, forgejoDeployKeyAction(key)))
		}
	}
	return problems
}

func validateForgejoAppCandidates(candidates []forgejoAppCandidate, apps []inventory.GitHubApp) []string {
	var problems []string
	byName := map[string]forgejoAppCandidate{}
	for _, candidate := range candidates {
		byName[candidate.Name] = candidate
	}
	if len(candidates) != len(apps) {
		problems = append(problems, fmt.Sprintf("integrationReview count %d does not match inventory count %d", len(candidates), len(apps)))
	}
	for _, app := range apps {
		candidate, ok := byName[app.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("integrationReview: missing candidate for %s", app.Name))
			continue
		}
		if candidate.RepositorySelection != app.RepositorySelection || candidate.WebhookEnabled != app.WebhookEnabled || candidate.EvidenceRef != app.EvidenceRef {
			problems = append(problems, fmt.Sprintf("integrationReview.%s must preserve selection, webhook state, and evidenceRef", app.Name))
		}
		if !sameStringSet(candidate.Repositories, app.Repositories) || !sameStringSet(candidate.Permissions, app.Permissions) || !sameStringSet(candidate.Events, app.Events) {
			problems = append(problems, fmt.Sprintf("integrationReview.%s must preserve repositories, permissions, and events", app.Name))
		}
		if candidate.TargetAction != "map-to-forgejo-integration-or-replace" {
			problems = append(problems, fmt.Sprintf("integrationReview.%s.targetAction must be map-to-forgejo-integration-or-replace", app.Name))
		}
	}
	return problems
}

func validateForgejoManualReviewCandidates(candidates []forgejoManualReviewCandidate, a *assessment.Assessment) []string {
	if a == nil {
		return nil
	}
	var problems []string
	byID := map[string][]forgejoManualReviewCandidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = append(byID[candidate.ID], candidate)
	}
	expected := 0
	for _, finding := range a.Findings {
		if !strings.HasPrefix(finding.ID, "ghe.") || finding.Severity == "low" {
			continue
		}
		expected++
		matches := byID[finding.ID]
		found := false
		for _, candidate := range matches {
			if candidate.Title == finding.Title &&
				candidate.Severity == finding.Severity &&
				candidate.Recommendation == finding.Recommendation &&
				sameStringSet(candidate.AffectedAssets, finding.AffectedAssets) &&
				sameStringSet(candidate.EvidenceRefs, finding.EvidenceRefs) {
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("manualReview missing finding %s with preserved assets/evidence/recommendation", finding.ID))
		}
	}
	if len(candidates) != expected {
		problems = append(problems, fmt.Sprintf("manualReview count %d does not match non-low GitHub Enterprise finding count %d", len(candidates), expected))
	}
	return problems
}

func validateForgejoCandidateREADME(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "forgejo", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/forgejo/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"deterministic Forgejo migration candidate", "no repository contents", "secret values", "Review repository ownership", "CI compatibility"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/forgejo/README.md: missing %q", required)}
		}
	}
	return nil
}

func mapForgejoRepositories(candidates []forgejoRepositoryCandidate) map[string]forgejoRepositoryCandidate {
	out := map[string]forgejoRepositoryCandidate{}
	for _, candidate := range candidates {
		out[candidate.Name] = candidate
	}
	return out
}

func mapForgejoBranches(candidates []forgejoBranchCandidate) map[string]forgejoBranchCandidate {
	out := map[string]forgejoBranchCandidate{}
	for _, candidate := range candidates {
		out[branchKey(candidate.Repository, candidate.Branch)] = candidate
	}
	return out
}

func mapForgejoWorkflows(candidates []forgejoWorkflowCandidate) map[string]forgejoWorkflowCandidate {
	out := map[string]forgejoWorkflowCandidate{}
	for _, candidate := range candidates {
		out[workflowKey(candidate.Repository, candidate.Path)] = candidate
	}
	return out
}

func branchProtectionsByRepo(protections []inventory.BranchProtection) map[string][]inventory.BranchProtection {
	out := map[string][]inventory.BranchProtection{}
	for _, protection := range protections {
		out[protection.Repository] = append(out[protection.Repository], protection)
	}
	return out
}

func workflowsByRepo(workflows []inventory.ActionsWorkflow) map[string][]inventory.ActionsWorkflow {
	out := map[string][]inventory.ActionsWorkflow{}
	for _, workflow := range workflows {
		out[workflow.Repository] = append(out[workflow.Repository], workflow)
	}
	return out
}

func secretCountsByRepo(secrets []inventory.SecretMetadata) map[string]int {
	out := map[string]int{}
	for _, secret := range secrets {
		out[secret.Repository]++
	}
	return out
}

func splitRepositoryName(name string) (string, string) {
	owner, repo, ok := strings.Cut(name, "/")
	if !ok {
		return "default", name
	}
	return owner, repo
}

func sourceCloneRef(host, repo string) string {
	if strings.TrimSpace(host) == "" {
		return repo
	}
	return strings.TrimSuffix(host, "/") + "/" + repo
}

func forgejoRepositoryReviewRequired(repo inventory.Repository) bool {
	return !repo.HasCODEOWNERS || repo.UsesGitHubPages || repo.UsesGitHubPackages || repo.UsesGitHubDiscussions
}

func forgejoRunnerPlan(workflow inventory.ActionsWorkflow) string {
	if workflow.UsesSelfHosted {
		return "map-existing-self-hosted-runner-labels"
	}
	if workflow.UsesGitHubHosted {
		return "provision-forgejo-runner"
	}
	return "review-workflow-runner-requirements"
}

func forgejoRunnerAction(runner inventory.Runner) string {
	if !runner.Online {
		return "replace-offline-runner"
	}
	return "register-forgejo-runner"
}

func forgejoDeployKeyAction(key inventory.DeployKey) string {
	if key.ReadOnly {
		return "recreate-read-only-key-after-approval"
	}
	return "replace-with-scoped-deploy-token-or-reviewed-write-key"
}

func branchKey(repo, branch string) string {
	return repo + "\x00" + branch
}

func workflowKey(repo, path string) string {
	return repo + "\x00" + path
}

func secretKey(scope, repo, name string) string {
	return scope + "\x00" + repo + "\x00" + name
}

func runnerKey(scope, name string) string {
	return scope + "\x00" + name
}

func deployKeyKey(repo, title string) string {
	return repo + "\x00" + title
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
