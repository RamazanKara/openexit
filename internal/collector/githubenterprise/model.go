package githubenterprise

type Fixture struct {
	Host              string                    `json:"host"`
	Repositories      []FixtureRepository       `json:"repositories"`
	Teams             []FixtureTeam             `json:"teams"`
	BranchProtections []FixtureBranchProtection `json:"branchProtections"`
	ActionsWorkflows  []FixtureActionsWorkflow  `json:"actionsWorkflows"`
	Secrets           []FixtureSecretMetadata   `json:"secrets"`
	Runners           []FixtureRunner           `json:"runners"`
	DeployKeys        []FixtureDeployKey        `json:"deployKeys"`
	GitHubApps        []FixtureGitHubApp        `json:"githubApps"`
}

type FixtureRepository struct {
	Name                  string   `json:"name"`
	Visibility            string   `json:"visibility"`
	DefaultBranch         string   `json:"defaultBranch"`
	Archived              bool     `json:"archived"`
	Topics                []string `json:"topics"`
	Teams                 []string `json:"teams"`
	ActionsEnabled        bool     `json:"actionsEnabled"`
	HasCODEOWNERS         bool     `json:"hasCodeowners"`
	UsesGitHubPages       bool     `json:"usesGitHubPages"`
	UsesGitHubPackages    bool     `json:"usesGitHubPackages"`
	UsesGitHubDiscussions bool     `json:"usesGitHubDiscussions"`
}

type FixtureTeam struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Members     int      `json:"members"`
	Maintainers int      `json:"maintainers"`
	Repos       []string `json:"repos"`
}

type FixtureBranchProtection struct {
	Repository             string   `json:"repository"`
	Branch                 string   `json:"branch"`
	RequiredStatusChecks   []string `json:"requiredStatusChecks"`
	RequiredReviews        int      `json:"requiredReviews"`
	RequireCodeOwnerReview bool     `json:"requireCodeOwnerReview"`
	RestrictPushes         bool     `json:"restrictPushes"`
	AllowsForcePushes      bool     `json:"allowsForcePushes"`
}

type FixtureActionsWorkflow struct {
	Repository       string   `json:"repository"`
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	UsesGitHubHosted bool     `json:"usesGitHubHosted"`
	UsesSelfHosted   bool     `json:"usesSelfHosted"`
	Actions          []string `json:"actions"`
	Secrets          []string `json:"secrets"`
}

type FixtureSecretMetadata struct {
	Name       string   `json:"name"`
	Scope      string   `json:"scope"`
	Repository string   `json:"repository"`
	Consumers  []string `json:"consumers"`
}

type FixtureRunner struct {
	Name   string   `json:"name"`
	Scope  string   `json:"scope"`
	Labels []string `json:"labels"`
	Online bool     `json:"online"`
}

type FixtureDeployKey struct {
	Repository string `json:"repository"`
	Title      string `json:"title"`
	ReadOnly   bool   `json:"readOnly"`
}

type FixtureGitHubApp struct {
	Name           string   `json:"name"`
	Repositories   []string `json:"repositories"`
	Permissions    []string `json:"permissions"`
	WebhookEnabled bool     `json:"webhookEnabled"`
}
