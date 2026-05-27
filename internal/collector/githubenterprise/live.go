package githubenterprise

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type LiveCollector struct{}

func (LiveCollector) Name() string { return "github" }

func (LiveCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	owner := strings.TrimSpace(req.Options["owner"])
	if owner == "" {
		return nil, fmt.Errorf("--owner is required")
	}
	ownerType := option(req.Options, "owner-type", "org")
	if ownerType != "org" && ownerType != "user" {
		return nil, fmt.Errorf("--owner-type must be org or user")
	}
	tokenEnv := option(req.Options, "token-env", "GITHUB_TOKEN")
	token, err := envSecret(tokenEnv)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(req.Options["base-url"], token)
	if err != nil {
		return nil, err
	}

	repos, err := collectGitHubRepositories(ctx, client, owner, ownerType)
	if err != nil {
		return nil, err
	}
	filters := repoFilterSet(req.Options["repos"], owner)
	fixture := Fixture{Host: client.baseURL}
	var warnings []string

	for _, repo := range repos {
		if !repoAllowed(repo, filters, owner) {
			continue
		}
		repoName := repoInventoryName(repo, owner)
		var repoTeams []githubTeam
		if ownerType == "org" {
			repoTeams, err = collectRepoTeams(ctx, client, owner, repo.Name)
			if err != nil {
				warnings = append(warnings, optionalWarning("collect repository teams for "+repoName, err))
			}
		}
		workflows, workflowSecrets, err := collectRepoWorkflows(ctx, client, owner, repo.Name, repoName, repo.DefaultBranch)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect workflows for "+repoName, err))
		}
		codeowners, err := hasCODEOWNERS(ctx, client, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect CODEOWNERS metadata for "+repoName, err))
		}
		if protection, err := collectBranchProtection(ctx, client, owner, repo.Name, repoName, repo.DefaultBranch); err == nil && protection != nil {
			fixture.BranchProtections = append(fixture.BranchProtections, *protection)
		} else if err != nil && apiStatus(err) != http.StatusNotFound {
			warnings = append(warnings, optionalWarning("collect branch protection for "+repoName, err))
		}
		secrets, err := collectRepoSecrets(ctx, client, owner, repo.Name, repoName, workflowSecrets)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect repository secrets for "+repoName, err))
		}
		deployKeys, err := collectDeployKeys(ctx, client, owner, repo.Name, repoName)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect deploy keys for "+repoName, err))
		}

		fixture.Repositories = append(fixture.Repositories, FixtureRepository{
			Name:                  repoName,
			Visibility:            repoVisibility(repo),
			DefaultBranch:         repo.DefaultBranch,
			Archived:              repo.Archived,
			Topics:                uniqueSortedStrings(repo.Topics),
			Teams:                 uniqueSortedStrings(teamSlugs(repoTeams)),
			ActionsEnabled:        len(workflows) > 0,
			HasCODEOWNERS:         codeowners,
			UsesGitHubPages:       repo.HasPages,
			UsesGitHubPackages:    repo.HasPackages,
			UsesGitHubDiscussions: repo.HasDiscussions,
		})
		fixture.ActionsWorkflows = append(fixture.ActionsWorkflows, workflows...)
		fixture.Secrets = append(fixture.Secrets, secrets...)
		fixture.DeployKeys = append(fixture.DeployKeys, deployKeys...)
	}

	if ownerType == "org" {
		selectedRepos := selectedRepoNames(fixture.Repositories)
		teams, err := collectOrgTeams(ctx, client, owner, selectedRepos, len(filters) > 0)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect organization teams", err))
		}
		runners, err := collectOrgRunners(ctx, client, owner)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect organization runners", err))
		}
		secrets, err := collectOrgSecrets(ctx, client, owner, fixture.ActionsWorkflows)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect organization secrets", err))
		}
		apps, err := collectOrgGitHubApps(ctx, client, owner)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect organization GitHub Apps", err))
		}
		fixture.Teams = append(fixture.Teams, teams...)
		fixture.Runners = append(fixture.Runners, runners...)
		fixture.Secrets = append(fixture.Secrets, secrets...)
		fixture.GitHubApps = append(fixture.GitHubApps, apps...)
	} else {
		warnings = append(warnings, "organization-only metadata was skipped because --owner-type=user")
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "github-enterprise", client.baseURL+"/"+owner, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "github-enterprise/raw-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type githubRepository struct {
	Name           string   `json:"name"`
	FullName       string   `json:"full_name"`
	Visibility     string   `json:"visibility"`
	Private        bool     `json:"private"`
	DefaultBranch  string   `json:"default_branch"`
	Archived       bool     `json:"archived"`
	Topics         []string `json:"topics"`
	HasPages       bool     `json:"has_pages"`
	HasPackages    bool     `json:"has_packages"`
	HasDiscussions bool     `json:"has_discussions"`
}

type githubTeam struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type githubWorkflow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	State string `json:"state"`
}

type githubSecret struct {
	Name string `json:"name"`
}

type githubRunner struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

type githubDeployKey struct {
	Title    string `json:"title"`
	ReadOnly bool   `json:"read_only"`
}

type githubAppInstallation struct {
	ID                  int64             `json:"id"`
	AppID               int64             `json:"app_id"`
	AppSlug             string            `json:"app_slug"`
	RepositorySelection string            `json:"repository_selection"`
	Permissions         map[string]string `json:"permissions"`
	Events              []string          `json:"events"`
}

func collectGitHubRepositories(ctx context.Context, client *Client, owner, ownerType string) ([]githubRepository, error) {
	path := "/orgs/" + url.PathEscape(owner) + "/repos"
	query := url.Values{"type": {"all"}, "sort": {"full_name"}}
	if ownerType == "user" {
		path = "/users/" + url.PathEscape(owner) + "/repos"
		query = url.Values{"type": {"owner"}, "sort": {"full_name"}}
	}
	return getPagedArray[githubRepository](ctx, client, path, query)
}

func collectRepoTeams(ctx context.Context, client *Client, owner, repo string) ([]githubTeam, error) {
	return getPagedArray[githubTeam](ctx, client, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/teams", nil)
}

func collectRepoWorkflows(ctx context.Context, client *Client, owner, repo, repoName, defaultBranch string) ([]FixtureActionsWorkflow, map[string][]string, error) {
	var out []FixtureActionsWorkflow
	consumers := map[string][]string{}
	workflows, err := getPagedObjectArray[githubWorkflow](ctx, client, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/actions/workflows", nil, "workflows")
	if err != nil {
		return nil, consumers, err
	}
	for _, workflow := range workflows {
		item := FixtureActionsWorkflow{Repository: repoName, Path: workflow.Path, Name: workflow.Name}
		if workflow.Path != "" {
			content, err := getContentFile(ctx, client, owner, repo, workflow.Path, defaultBranch)
			if err == nil {
				item.UsesGitHubHosted, item.UsesSelfHosted, item.Actions, item.Secrets = workflowSignals(content)
				for _, secret := range item.Secrets {
					consumers[secret] = append(consumers[secret], workflow.Path)
				}
			}
		}
		out = append(out, item)
	}
	return out, consumers, nil
}

func hasCODEOWNERS(ctx context.Context, client *Client, owner, repo, defaultBranch string) (bool, error) {
	for _, candidate := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		_, err := getContentFile(ctx, client, owner, repo, candidate, defaultBranch)
		if err == nil {
			return true, nil
		}
		if apiStatus(err) == http.StatusNotFound {
			continue
		}
		return false, err
	}
	return false, nil
}

func collectBranchProtection(ctx context.Context, client *Client, owner, repo, repoName, branch string) (*FixtureBranchProtection, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, nil
	}
	var raw struct {
		RequiredStatusChecks *struct {
			Contexts []string `json:"contexts"`
			Checks   []struct {
				Context string `json:"context"`
			} `json:"checks"`
		} `json:"required_status_checks"`
		RequiredPullRequestReviews *struct {
			RequiredApprovingReviewCount int  `json:"required_approving_review_count"`
			RequireCodeOwnerReviews      bool `json:"require_code_owner_reviews"`
		} `json:"required_pull_request_reviews"`
		Restrictions     any `json:"restrictions"`
		AllowForcePushes *struct {
			Enabled bool `json:"enabled"`
		} `json:"allow_force_pushes"`
	}
	_, err := client.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/branches/"+url.PathEscape(branch)+"/protection", nil, &raw)
	if err != nil {
		return nil, err
	}
	protection := FixtureBranchProtection{Repository: repoName, Branch: branch}
	if raw.RequiredStatusChecks != nil {
		protection.RequiredStatusChecks = append(protection.RequiredStatusChecks, raw.RequiredStatusChecks.Contexts...)
		for _, check := range raw.RequiredStatusChecks.Checks {
			if check.Context != "" {
				protection.RequiredStatusChecks = append(protection.RequiredStatusChecks, check.Context)
			}
		}
		protection.RequiredStatusChecks = uniqueSortedStrings(protection.RequiredStatusChecks)
	}
	if raw.RequiredPullRequestReviews != nil {
		protection.RequiredReviews = raw.RequiredPullRequestReviews.RequiredApprovingReviewCount
		protection.RequireCodeOwnerReview = raw.RequiredPullRequestReviews.RequireCodeOwnerReviews
	}
	protection.RestrictPushes = raw.Restrictions != nil
	if raw.AllowForcePushes != nil {
		protection.AllowsForcePushes = raw.AllowForcePushes.Enabled
	}
	return &protection, nil
}

func collectRepoSecrets(ctx context.Context, client *Client, owner, repo, repoName string, consumers map[string][]string) ([]FixtureSecretMetadata, error) {
	secrets, err := getPagedObjectArray[githubSecret](ctx, client, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/actions/secrets", nil, "secrets")
	if err != nil {
		return nil, err
	}
	out := make([]FixtureSecretMetadata, 0, len(secrets))
	for _, secret := range secrets {
		if secret.Name == "" {
			continue
		}
		out = append(out, FixtureSecretMetadata{
			Name:       secret.Name,
			Scope:      "repository",
			Repository: repoName,
			Consumers:  uniqueSortedStrings(consumers[secret.Name]),
		})
	}
	return out, nil
}

func collectDeployKeys(ctx context.Context, client *Client, owner, repo, repoName string) ([]FixtureDeployKey, error) {
	keys, err := getPagedArray[githubDeployKey](ctx, client, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/keys", nil)
	if err != nil {
		return nil, err
	}
	out := make([]FixtureDeployKey, 0, len(keys))
	for _, key := range keys {
		if key.Title == "" {
			continue
		}
		out = append(out, FixtureDeployKey{Repository: repoName, Title: key.Title, ReadOnly: key.ReadOnly})
	}
	return out, nil
}

func collectOrgTeams(ctx context.Context, client *Client, owner string, selectedRepos map[string]struct{}, filterRepos bool) ([]FixtureTeam, error) {
	teams, err := getPagedArray[githubTeam](ctx, client, "/orgs/"+url.PathEscape(owner)+"/teams", nil)
	if err != nil {
		return nil, err
	}
	out := make([]FixtureTeam, 0, len(teams))
	for _, team := range teams {
		repos, err := getPagedArray[githubRepository](ctx, client, "/orgs/"+url.PathEscape(owner)+"/teams/"+url.PathEscape(team.Slug)+"/repos", nil)
		if err != nil {
			return nil, err
		}
		members, err := getPagedArray[struct {
			Login string `json:"login"`
		}](ctx, client, "/orgs/"+url.PathEscape(owner)+"/teams/"+url.PathEscape(team.Slug)+"/members", nil)
		if err != nil {
			return nil, err
		}
		maintainers, err := getPagedArray[struct {
			Login string `json:"login"`
		}](ctx, client, "/orgs/"+url.PathEscape(owner)+"/teams/"+url.PathEscape(team.Slug)+"/members", url.Values{"role": {"maintainer"}})
		if err != nil {
			return nil, err
		}
		repoNames := make([]string, 0, len(repos))
		for _, repo := range repos {
			name := repoInventoryName(repo, owner)
			if filterRepos {
				if _, ok := selectedRepos[name]; !ok {
					continue
				}
			}
			repoNames = append(repoNames, name)
		}
		out = append(out, FixtureTeam{
			Slug:        team.Slug,
			Name:        team.Name,
			Members:     len(members),
			Maintainers: len(maintainers),
			Repos:       uniqueSortedStrings(repoNames),
		})
	}
	return out, nil
}

func collectOrgRunners(ctx context.Context, client *Client, owner string) ([]FixtureRunner, error) {
	runners, err := getPagedObjectArray[githubRunner](ctx, client, "/orgs/"+url.PathEscape(owner)+"/actions/runners", nil, "runners")
	if err != nil {
		return nil, err
	}
	out := make([]FixtureRunner, 0, len(runners))
	for _, runner := range runners {
		var labels []string
		for _, label := range runner.Labels {
			if label.Name != "" {
				labels = append(labels, label.Name)
			}
		}
		out = append(out, FixtureRunner{
			Name:   runner.Name,
			Scope:  "organization",
			Labels: uniqueSortedStrings(labels),
			Online: strings.EqualFold(runner.Status, "online"),
		})
	}
	return out, nil
}

func collectOrgSecrets(ctx context.Context, client *Client, owner string, workflows []FixtureActionsWorkflow) ([]FixtureSecretMetadata, error) {
	secrets, err := getPagedObjectArray[githubSecret](ctx, client, "/orgs/"+url.PathEscape(owner)+"/actions/secrets", nil, "secrets")
	if err != nil {
		return nil, err
	}
	consumers := map[string][]string{}
	for _, workflow := range workflows {
		for _, secret := range workflow.Secrets {
			consumers[secret] = append(consumers[secret], workflow.Repository+":"+workflow.Path)
		}
	}
	out := make([]FixtureSecretMetadata, 0, len(secrets))
	for _, secret := range secrets {
		if secret.Name == "" {
			continue
		}
		out = append(out, FixtureSecretMetadata{
			Name:      secret.Name,
			Scope:     "organization",
			Consumers: uniqueSortedStrings(consumers[secret.Name]),
		})
	}
	return out, nil
}

func collectOrgGitHubApps(ctx context.Context, client *Client, owner string) ([]FixtureGitHubApp, error) {
	installations, err := getPagedObjectArray[githubAppInstallation](ctx, client, "/orgs/"+url.PathEscape(owner)+"/installations", nil, "installations")
	if err != nil {
		return nil, err
	}
	out := make([]FixtureGitHubApp, 0, len(installations))
	for _, installation := range installations {
		name := strings.TrimSpace(installation.AppSlug)
		if name == "" {
			if installation.AppID > 0 {
				name = "github-app-" + strconv.FormatInt(installation.AppID, 10)
			} else {
				name = "github-app-installation-" + strconv.FormatInt(installation.ID, 10)
			}
		}
		out = append(out, FixtureGitHubApp{
			Name:                name,
			RepositorySelection: strings.TrimSpace(installation.RepositorySelection),
			Permissions:         githubAppPermissions(installation.Permissions),
			Events:              uniqueSortedStrings(installation.Events),
			WebhookEnabled:      len(installation.Events) > 0,
		})
	}
	return out, nil
}

func githubAppPermissions(permissions map[string]string) []string {
	out := make([]string, 0, len(permissions))
	for name, level := range permissions {
		name = strings.TrimSpace(name)
		level = strings.TrimSpace(level)
		if name == "" || level == "" {
			continue
		}
		out = append(out, name+":"+level)
	}
	sort.Strings(out)
	return out
}

func getContentFile(ctx context.Context, client *Client, owner, repo, filePath, ref string) (string, error) {
	var raw struct {
		Type     string `json:"type"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	query := url.Values{}
	if strings.TrimSpace(ref) != "" {
		query.Set("ref", ref)
	}
	_, err := client.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/contents/"+escapeGitHubPath(filePath), query, &raw)
	if err != nil {
		return "", err
	}
	if raw.Content == "" {
		return "", nil
	}
	if raw.Encoding != "base64" {
		return raw.Content, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(raw.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func getPagedArray[T any](ctx context.Context, client *Client, path string, query url.Values) ([]T, error) {
	const pageSize = 100
	var out []T
	for page := 1; ; page++ {
		q := cloneValues(query)
		q.Set("per_page", strconv.Itoa(pageSize))
		q.Set("page", strconv.Itoa(page))
		var batch []T
		if _, err := client.get(ctx, path, q, &batch); err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < pageSize {
			break
		}
	}
	return out, nil
}

func getPagedObjectArray[T any](ctx context.Context, client *Client, path string, query url.Values, field string) ([]T, error) {
	const pageSize = 100
	var out []T
	for page := 1; ; page++ {
		q := cloneValues(query)
		q.Set("per_page", strconv.Itoa(pageSize))
		q.Set("page", strconv.Itoa(page))
		var raw map[string]json.RawMessage
		if _, err := client.get(ctx, path, q, &raw); err != nil {
			return nil, err
		}
		var batch []T
		if err := json.Unmarshal(raw[field], &batch); err != nil {
			return nil, fmt.Errorf("parse github response field %q: %w", field, err)
		}
		out = append(out, batch...)
		if len(batch) < pageSize {
			break
		}
	}
	return out, nil
}

func cloneValues(input url.Values) url.Values {
	out := url.Values{}
	for key, values := range input {
		out[key] = append([]string{}, values...)
	}
	return out
}

var (
	actionUsePattern = regexp.MustCompile(`(?mi)^\s*-?\s*uses:\s*["']?([^"'\s#]+)`)
	secretPattern    = regexp.MustCompile(`secrets\.([A-Za-z_][A-Za-z0-9_]*)`)
)

func workflowSignals(content string) (bool, bool, []string, []string) {
	lower := strings.ToLower(content)
	usesGitHubHosted := strings.Contains(lower, "ubuntu-latest") ||
		strings.Contains(lower, "windows-latest") ||
		strings.Contains(lower, "macos-latest") ||
		regexp.MustCompile(`(?m)(ubuntu|windows|macos)-[0-9]`).MatchString(lower)
	usesSelfHosted := strings.Contains(lower, "self-hosted")

	var actions []string
	for _, match := range actionUsePattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			actions = append(actions, strings.Trim(match[1], `"'`))
		}
	}
	var secrets []string
	for _, match := range secretPattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			secrets = append(secrets, match[1])
		}
	}
	return usesGitHubHosted, usesSelfHosted, uniqueSortedStrings(actions), uniqueSortedStrings(secrets)
}

func repoInventoryName(repo githubRepository, owner string) string {
	if strings.TrimSpace(repo.FullName) != "" {
		return repo.FullName
	}
	if strings.TrimSpace(repo.Name) == "" {
		return owner + "/unnamed"
	}
	return owner + "/" + repo.Name
}

func repoVisibility(repo githubRepository) string {
	if strings.TrimSpace(repo.Visibility) != "" {
		return repo.Visibility
	}
	if repo.Private {
		return "private"
	}
	return "public"
}

func repoFilterSet(raw, owner string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[string]struct{}{}
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' }) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out[line] = struct{}{}
		if !strings.Contains(line, "/") {
			out[owner+"/"+line] = struct{}{}
		}
	}
	return out
}

func repoAllowed(repo githubRepository, filters map[string]struct{}, owner string) bool {
	if len(filters) == 0 {
		return true
	}
	_, byFullName := filters[repoInventoryName(repo, owner)]
	_, byName := filters[repo.Name]
	return byFullName || byName
}

func selectedRepoNames(repos []FixtureRepository) map[string]struct{} {
	out := map[string]struct{}{}
	for _, repo := range repos {
		out[repo.Name] = struct{}{}
	}
	return out
}

func teamSlugs(teams []githubTeam) []string {
	out := make([]string, 0, len(teams))
	for _, team := range teams {
		if team.Slug != "" {
			out = append(out, team.Slug)
		}
	}
	return out
}

func optionalWarning(label string, err error) string {
	return label + ": " + err.Error()
}

func compactWarnings(warnings []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		out = append(out, warning)
	}
	sort.Strings(out)
	return out
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func escapeGitHubPath(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func option(options map[string]string, key, fallback string) string {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	return value
}

func prettyJSON(value any) []byte {
	data, _ := json.MarshalIndent(value, "", "  ")
	return data
}
