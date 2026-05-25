package inventory

import "time"

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "Inventory"
)

type Inventory struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Source     Source   `json:"source" yaml:"source"`
	Assets     Assets   `json:"assets" yaml:"assets"`
	Summary    Summary  `json:"summary" yaml:"summary"`
	Volumes    Volumes  `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Warnings   []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type Metadata struct {
	Project          string    `json:"project" yaml:"project"`
	Source           string    `json:"source" yaml:"source"`
	CollectedAt      time.Time `json:"collectedAt" yaml:"collectedAt"`
	CollectorVersion string    `json:"collectorVersion" yaml:"collectorVersion"`
}

type Source struct {
	Type string `json:"type" yaml:"type"`
	Site string `json:"site,omitempty" yaml:"site,omitempty"`
}

type Assets struct {
	Dashboards                  []Dashboard                 `json:"dashboards,omitempty" yaml:"dashboards,omitempty"`
	Monitors                    []Monitor                   `json:"monitors,omitempty" yaml:"monitors,omitempty"`
	SLOs                        []SLO                       `json:"slos,omitempty" yaml:"slos,omitempty"`
	Integrations                []Integration               `json:"integrations,omitempty" yaml:"integrations,omitempty"`
	Metrics                     []MetricRef                 `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Repositories                []Repository                `json:"repositories,omitempty" yaml:"repositories,omitempty"`
	Teams                       []Team                      `json:"teams,omitempty" yaml:"teams,omitempty"`
	BranchProtections           []BranchProtection          `json:"branchProtections,omitempty" yaml:"branchProtections,omitempty"`
	ActionsWorkflows            []ActionsWorkflow           `json:"actionsWorkflows,omitempty" yaml:"actionsWorkflows,omitempty"`
	Secrets                     []SecretMetadata            `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Runners                     []Runner                    `json:"runners,omitempty" yaml:"runners,omitempty"`
	DeployKeys                  []DeployKey                 `json:"deployKeys,omitempty" yaml:"deployKeys,omitempty"`
	GitHubApps                  []GitHubApp                 `json:"githubApps,omitempty" yaml:"githubApps,omitempty"`
	IdentityApps                []IdentityApp               `json:"identityApplications,omitempty" yaml:"identityApplications,omitempty"`
	IdentityGroups              []IdentityGroup             `json:"identityGroups,omitempty" yaml:"identityGroups,omitempty"`
	IdentityPolicies            []IdentityPolicy            `json:"identityPolicies,omitempty" yaml:"identityPolicies,omitempty"`
	MFASettings                 []MFASetting                `json:"mfaSettings,omitempty" yaml:"mfaSettings,omitempty"`
	BreakGlassAccounts          []BreakGlassAccount         `json:"breakGlassAccounts,omitempty" yaml:"breakGlassAccounts,omitempty"`
	DNSRecords                  []DNSRecord                 `json:"dnsRecords,omitempty" yaml:"dnsRecords,omitempty"`
	WAFRules                    []WAFRule                   `json:"wafRules,omitempty" yaml:"wafRules,omitempty"`
	CacheRules                  []CacheRule                 `json:"cacheRules,omitempty" yaml:"cacheRules,omitempty"`
	Redirects                   []RedirectRule              `json:"redirects,omitempty" yaml:"redirects,omitempty"`
	Origins                     []OriginConfig              `json:"origins,omitempty" yaml:"origins,omitempty"`
	TLSSettings                 []TLSSetting                `json:"tlsSettings,omitempty" yaml:"tlsSettings,omitempty"`
	BotRules                    []BotRule                   `json:"botRules,omitempty" yaml:"botRules,omitempty"`
	PageRules                   []PageRule                  `json:"pageRules,omitempty" yaml:"pageRules,omitempty"`
	AIModelUsageClasses         []AIModelUsageClass         `json:"aiModelUsageClasses,omitempty" yaml:"aiModelUsageClasses,omitempty"`
	AITokenVolumes              []AITokenVolume             `json:"aiTokenVolumes,omitempty" yaml:"aiTokenVolumes,omitempty"`
	AILatencyExpectations       []AILatencyExpectation      `json:"aiLatencyExpectations,omitempty" yaml:"aiLatencyExpectations,omitempty"`
	AISensitivePromptCategories []AISensitivePromptCategory `json:"aiSensitivePromptCategories,omitempty" yaml:"aiSensitivePromptCategories,omitempty"`
	AIToolUsages                []AIToolUsage               `json:"aiToolUsages,omitempty" yaml:"aiToolUsages,omitempty"`
	AIFallbackBehaviors         []AIFallbackBehavior        `json:"aiFallbackBehaviors,omitempty" yaml:"aiFallbackBehaviors,omitempty"`
}

type Dashboard struct {
	ID                string             `json:"id" yaml:"id"`
	Title             string             `json:"title" yaml:"title"`
	URL               string             `json:"url,omitempty" yaml:"url,omitempty"`
	Tags              []string           `json:"tags,omitempty" yaml:"tags,omitempty"`
	Widgets           WidgetSummary      `json:"widgets" yaml:"widgets"`
	Queries           []Query            `json:"queries,omitempty" yaml:"queries,omitempty"`
	DataSources       []string           `json:"dataSources,omitempty" yaml:"dataSources,omitempty"`
	TemplateVariables []TemplateVariable `json:"templateVariables,omitempty" yaml:"templateVariables,omitempty"`
	EvidenceRef       string             `json:"evidenceRef" yaml:"evidenceRef"`
}

type WidgetSummary struct {
	Total       int      `json:"total" yaml:"total"`
	Unsupported int      `json:"unsupported" yaml:"unsupported"`
	Types       []string `json:"types,omitempty" yaml:"types,omitempty"`
}

type Query struct {
	ID       string `json:"id" yaml:"id"`
	Language string `json:"language" yaml:"language"`
	Raw      string `json:"raw" yaml:"raw"`
}

type TemplateVariable struct {
	Name  string `json:"name" yaml:"name"`
	Query string `json:"query,omitempty" yaml:"query,omitempty"`
}

type Monitor struct {
	ID                  string   `json:"id" yaml:"id"`
	Name                string   `json:"name" yaml:"name"`
	Type                string   `json:"type" yaml:"type"`
	Query               string   `json:"query" yaml:"query"`
	NotificationTargets []string `json:"notificationTargets,omitempty" yaml:"notificationTargets,omitempty"`
	Tags                []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	RunbookURL          string   `json:"runbookUrl,omitempty" yaml:"runbookUrl,omitempty"`
	EvidenceRef         string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type SLO struct {
	ID                 string   `json:"id" yaml:"id"`
	Name               string   `json:"name" yaml:"name"`
	Target             float64  `json:"target" yaml:"target"`
	Timeframe          string   `json:"timeframe" yaml:"timeframe"`
	SLI                string   `json:"sli,omitempty" yaml:"sli,omitempty"`
	BurnRateMonitorIDs []string `json:"burnRateMonitorIds,omitempty" yaml:"burnRateMonitorIds,omitempty"`
	DashboardRefs      []string `json:"dashboardRefs,omitempty" yaml:"dashboardRefs,omitempty"`
	EvidenceRef        string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type Integration struct {
	Name        string   `json:"name" yaml:"name"`
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	EvidenceRef string   `json:"evidenceRef,omitempty" yaml:"evidenceRef,omitempty"`
}

type MetricRef struct {
	Name string   `json:"name" yaml:"name"`
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Summary struct {
	Dashboards                  int `json:"dashboards" yaml:"dashboards"`
	Monitors                    int `json:"monitors" yaml:"monitors"`
	SLOs                        int `json:"slos" yaml:"slos"`
	Integrations                int `json:"integrations" yaml:"integrations"`
	UniqueMetrics               int `json:"uniqueMetrics" yaml:"uniqueMetrics"`
	NotificationTargets         int `json:"notificationTargets" yaml:"notificationTargets"`
	Repositories                int `json:"repositories,omitempty" yaml:"repositories,omitempty"`
	Teams                       int `json:"teams,omitempty" yaml:"teams,omitempty"`
	BranchProtections           int `json:"branchProtections,omitempty" yaml:"branchProtections,omitempty"`
	ActionsWorkflows            int `json:"actionsWorkflows,omitempty" yaml:"actionsWorkflows,omitempty"`
	Secrets                     int `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Runners                     int `json:"runners,omitempty" yaml:"runners,omitempty"`
	DeployKeys                  int `json:"deployKeys,omitempty" yaml:"deployKeys,omitempty"`
	GitHubApps                  int `json:"githubApps,omitempty" yaml:"githubApps,omitempty"`
	IdentityApps                int `json:"identityApplications,omitempty" yaml:"identityApplications,omitempty"`
	IdentityGroups              int `json:"identityGroups,omitempty" yaml:"identityGroups,omitempty"`
	IdentityPolicies            int `json:"identityPolicies,omitempty" yaml:"identityPolicies,omitempty"`
	MFASettings                 int `json:"mfaSettings,omitempty" yaml:"mfaSettings,omitempty"`
	BreakGlassAccounts          int `json:"breakGlassAccounts,omitempty" yaml:"breakGlassAccounts,omitempty"`
	DNSRecords                  int `json:"dnsRecords,omitempty" yaml:"dnsRecords,omitempty"`
	WAFRules                    int `json:"wafRules,omitempty" yaml:"wafRules,omitempty"`
	CacheRules                  int `json:"cacheRules,omitempty" yaml:"cacheRules,omitempty"`
	Redirects                   int `json:"redirects,omitempty" yaml:"redirects,omitempty"`
	Origins                     int `json:"origins,omitempty" yaml:"origins,omitempty"`
	TLSSettings                 int `json:"tlsSettings,omitempty" yaml:"tlsSettings,omitempty"`
	BotRules                    int `json:"botRules,omitempty" yaml:"botRules,omitempty"`
	PageRules                   int `json:"pageRules,omitempty" yaml:"pageRules,omitempty"`
	AIModelUsageClasses         int `json:"aiModelUsageClasses,omitempty" yaml:"aiModelUsageClasses,omitempty"`
	AITokenVolumes              int `json:"aiTokenVolumes,omitempty" yaml:"aiTokenVolumes,omitempty"`
	AILatencyExpectations       int `json:"aiLatencyExpectations,omitempty" yaml:"aiLatencyExpectations,omitempty"`
	AISensitivePromptCategories int `json:"aiSensitivePromptCategories,omitempty" yaml:"aiSensitivePromptCategories,omitempty"`
	AIToolUsages                int `json:"aiToolUsages,omitempty" yaml:"aiToolUsages,omitempty"`
	AIFallbackBehaviors         int `json:"aiFallbackBehaviors,omitempty" yaml:"aiFallbackBehaviors,omitempty"`
}

type Volumes struct {
	LogVolumeKnown   bool `json:"logVolumeKnown,omitempty" yaml:"logVolumeKnown,omitempty"`
	TraceVolumeKnown bool `json:"traceVolumeKnown,omitempty" yaml:"traceVolumeKnown,omitempty"`
}

type Repository struct {
	Name                  string   `json:"name" yaml:"name"`
	Visibility            string   `json:"visibility" yaml:"visibility"`
	DefaultBranch         string   `json:"defaultBranch" yaml:"defaultBranch"`
	Archived              bool     `json:"archived" yaml:"archived"`
	Topics                []string `json:"topics,omitempty" yaml:"topics,omitempty"`
	Teams                 []string `json:"teams,omitempty" yaml:"teams,omitempty"`
	ActionsEnabled        bool     `json:"actionsEnabled" yaml:"actionsEnabled"`
	HasCODEOWNERS         bool     `json:"hasCodeowners" yaml:"hasCodeowners"`
	UsesGitHubPages       bool     `json:"usesGitHubPages,omitempty" yaml:"usesGitHubPages,omitempty"`
	UsesGitHubPackages    bool     `json:"usesGitHubPackages,omitempty" yaml:"usesGitHubPackages,omitempty"`
	UsesGitHubDiscussions bool     `json:"usesGitHubDiscussions,omitempty" yaml:"usesGitHubDiscussions,omitempty"`
	EvidenceRef           string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type Team struct {
	Slug        string   `json:"slug" yaml:"slug"`
	Name        string   `json:"name" yaml:"name"`
	Members     int      `json:"members" yaml:"members"`
	Maintainers int      `json:"maintainers" yaml:"maintainers"`
	Repos       []string `json:"repos,omitempty" yaml:"repos,omitempty"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type BranchProtection struct {
	Repository             string   `json:"repository" yaml:"repository"`
	Branch                 string   `json:"branch" yaml:"branch"`
	RequiredStatusChecks   []string `json:"requiredStatusChecks,omitempty" yaml:"requiredStatusChecks,omitempty"`
	RequiredReviews        int      `json:"requiredReviews" yaml:"requiredReviews"`
	RequireCodeOwnerReview bool     `json:"requireCodeOwnerReview" yaml:"requireCodeOwnerReview"`
	RestrictPushes         bool     `json:"restrictPushes" yaml:"restrictPushes"`
	AllowsForcePushes      bool     `json:"allowsForcePushes" yaml:"allowsForcePushes"`
	EvidenceRef            string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type ActionsWorkflow struct {
	Repository       string   `json:"repository" yaml:"repository"`
	Path             string   `json:"path" yaml:"path"`
	Name             string   `json:"name" yaml:"name"`
	UsesGitHubHosted bool     `json:"usesGitHubHosted" yaml:"usesGitHubHosted"`
	UsesSelfHosted   bool     `json:"usesSelfHosted" yaml:"usesSelfHosted"`
	Actions          []string `json:"actions,omitempty" yaml:"actions,omitempty"`
	Secrets          []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	EvidenceRef      string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type SecretMetadata struct {
	Name        string   `json:"name" yaml:"name"`
	Scope       string   `json:"scope" yaml:"scope"`
	Repository  string   `json:"repository,omitempty" yaml:"repository,omitempty"`
	Consumers   []string `json:"consumers,omitempty" yaml:"consumers,omitempty"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type Runner struct {
	Name        string   `json:"name" yaml:"name"`
	Scope       string   `json:"scope" yaml:"scope"`
	Labels      []string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Online      bool     `json:"online" yaml:"online"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type DeployKey struct {
	Repository  string `json:"repository" yaml:"repository"`
	Title       string `json:"title" yaml:"title"`
	ReadOnly    bool   `json:"readOnly" yaml:"readOnly"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type GitHubApp struct {
	Name           string   `json:"name" yaml:"name"`
	Repositories   []string `json:"repositories,omitempty" yaml:"repositories,omitempty"`
	Permissions    []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	WebhookEnabled bool     `json:"webhookEnabled" yaml:"webhookEnabled"`
	EvidenceRef    string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type IdentityApp struct {
	ID                       string   `json:"id" yaml:"id"`
	Name                     string   `json:"name" yaml:"name"`
	Protocol                 string   `json:"protocol" yaml:"protocol"`
	ClientID                 string   `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	Owners                   []string `json:"owners,omitempty" yaml:"owners,omitempty"`
	Groups                   []string `json:"groups,omitempty" yaml:"groups,omitempty"`
	RedirectURIs             []string `json:"redirectUris,omitempty" yaml:"redirectUris,omitempty"`
	GrantTypes               []string `json:"grantTypes,omitempty" yaml:"grantTypes,omitempty"`
	SAMLSigningCertPresent   bool     `json:"samlSigningCertPresent,omitempty" yaml:"samlSigningCertPresent,omitempty"`
	SAMLSigningCertExpiresAt string   `json:"samlSigningCertExpiresAt,omitempty" yaml:"samlSigningCertExpiresAt,omitempty"`
	EvidenceRef              string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type IdentityGroup struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Owners      []string `json:"owners,omitempty" yaml:"owners,omitempty"`
	Members     int      `json:"members" yaml:"members"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type IdentityPolicy struct {
	ID           string   `json:"id" yaml:"id"`
	Name         string   `json:"name" yaml:"name"`
	Type         string   `json:"type" yaml:"type"`
	Groups       []string `json:"groups,omitempty" yaml:"groups,omitempty"`
	Applications []string `json:"applications,omitempty" yaml:"applications,omitempty"`
	EnforcesMFA  bool     `json:"enforcesMfa" yaml:"enforcesMfa"`
	Priority     int      `json:"priority,omitempty" yaml:"priority,omitempty"`
	EvidenceRef  string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type MFASetting struct {
	Name        string   `json:"name" yaml:"name"`
	Required    bool     `json:"required" yaml:"required"`
	Factors     []string `json:"factors,omitempty" yaml:"factors,omitempty"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type BreakGlassAccount struct {
	Username    string `json:"username" yaml:"username"`
	MFAEnabled  bool   `json:"mfaEnabled" yaml:"mfaEnabled"`
	ManagedBy   string `json:"managedBy,omitempty" yaml:"managedBy,omitempty"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type DNSRecord struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Value       string `json:"value" yaml:"value"`
	TTL         int    `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Proxied     bool   `json:"proxied,omitempty" yaml:"proxied,omitempty"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type WAFRule struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Expression  string `json:"expression" yaml:"expression"`
	Action      string `json:"action" yaml:"action"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Managed     bool   `json:"managed,omitempty" yaml:"managed,omitempty"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type CacheRule struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Pattern     string   `json:"pattern" yaml:"pattern"`
	Action      string   `json:"action" yaml:"action"`
	EdgeTTL     int      `json:"edgeTtl,omitempty" yaml:"edgeTtl,omitempty"`
	BrowserTTL  int      `json:"browserTtl,omitempty" yaml:"browserTtl,omitempty"`
	CacheKey    []string `json:"cacheKey,omitempty" yaml:"cacheKey,omitempty"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type RedirectRule struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	Source        string `json:"source" yaml:"source"`
	Target        string `json:"target" yaml:"target"`
	StatusCode    int    `json:"statusCode" yaml:"statusCode"`
	PreserveQuery bool   `json:"preserveQuery,omitempty" yaml:"preserveQuery,omitempty"`
	EvidenceRef   string `json:"evidenceRef" yaml:"evidenceRef"`
}

type OriginConfig struct {
	ID          string `json:"id" yaml:"id"`
	Hostname    string `json:"hostname" yaml:"hostname"`
	Scheme      string `json:"scheme" yaml:"scheme"`
	Port        int    `json:"port" yaml:"port"`
	HostHeader  string `json:"hostHeader,omitempty" yaml:"hostHeader,omitempty"`
	TLSVerify   bool   `json:"tlsVerify" yaml:"tlsVerify"`
	HealthCheck bool   `json:"healthCheck" yaml:"healthCheck"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type TLSSetting struct {
	ID          string `json:"id" yaml:"id"`
	Hostname    string `json:"hostname" yaml:"hostname"`
	Mode        string `json:"mode" yaml:"mode"`
	MinVersion  string `json:"minVersion" yaml:"minVersion"`
	HSTSEnabled bool   `json:"hstsEnabled" yaml:"hstsEnabled"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type BotRule struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Expression  string `json:"expression" yaml:"expression"`
	Action      string `json:"action" yaml:"action"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	EvidenceRef string `json:"evidenceRef" yaml:"evidenceRef"`
}

type PageRule struct {
	ID          string   `json:"id" yaml:"id"`
	Target      string   `json:"target" yaml:"target"`
	Actions     []string `json:"actions,omitempty" yaml:"actions,omitempty"`
	Priority    int      `json:"priority,omitempty" yaml:"priority,omitempty"`
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	EvidenceRef string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type AIModelUsageClass struct {
	ID                        string   `json:"id" yaml:"id"`
	Name                      string   `json:"name" yaml:"name"`
	Provider                  string   `json:"provider" yaml:"provider"`
	Models                    []string `json:"models,omitempty" yaml:"models,omitempty"`
	Purpose                   string   `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Owners                    []string `json:"owners,omitempty" yaml:"owners,omitempty"`
	SensitivePromptCategories []string `json:"sensitivePromptCategories,omitempty" yaml:"sensitivePromptCategories,omitempty"`
	ToolRefs                  []string `json:"toolRefs,omitempty" yaml:"toolRefs,omitempty"`
	FallbackBehavior          string   `json:"fallbackBehavior,omitempty" yaml:"fallbackBehavior,omitempty"`
	EvidenceRef               string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type AITokenVolume struct {
	ID                  string `json:"id" yaml:"id"`
	UsageClassID        string `json:"usageClassId" yaml:"usageClassId"`
	MonthlyInputTokens  int64  `json:"monthlyInputTokens" yaml:"monthlyInputTokens"`
	MonthlyOutputTokens int64  `json:"monthlyOutputTokens" yaml:"monthlyOutputTokens"`
	PeakTokensPerMinute int    `json:"peakTokensPerMinute" yaml:"peakTokensPerMinute"`
	BurstWindow         string `json:"burstWindow,omitempty" yaml:"burstWindow,omitempty"`
	EvidenceRef         string `json:"evidenceRef" yaml:"evidenceRef"`
}

type AILatencyExpectation struct {
	ID                string `json:"id" yaml:"id"`
	UsageClassID      string `json:"usageClassId" yaml:"usageClassId"`
	P50Ms             int    `json:"p50Ms,omitempty" yaml:"p50Ms,omitempty"`
	P95Ms             int    `json:"p95Ms" yaml:"p95Ms"`
	TimeoutMs         int    `json:"timeoutMs,omitempty" yaml:"timeoutMs,omitempty"`
	StreamingRequired bool   `json:"streamingRequired,omitempty" yaml:"streamingRequired,omitempty"`
	EvidenceRef       string `json:"evidenceRef" yaml:"evidenceRef"`
}

type AISensitivePromptCategory struct {
	ID                    string `json:"id" yaml:"id"`
	UsageClassID          string `json:"usageClassId" yaml:"usageClassId"`
	Category              string `json:"category" yaml:"category"`
	DataClass             string `json:"dataClass,omitempty" yaml:"dataClass,omitempty"`
	ContainsPII           bool   `json:"containsPii,omitempty" yaml:"containsPii,omitempty"`
	RequiresDataResidency bool   `json:"requiresDataResidency,omitempty" yaml:"requiresDataResidency,omitempty"`
	Retention             string `json:"retention,omitempty" yaml:"retention,omitempty"`
	EvidenceRef           string `json:"evidenceRef" yaml:"evidenceRef"`
}

type AIToolUsage struct {
	ID                    string `json:"id" yaml:"id"`
	UsageClassID          string `json:"usageClassId" yaml:"usageClassId"`
	Name                  string `json:"name" yaml:"name"`
	Type                  string `json:"type" yaml:"type"`
	NetworkAccess         bool   `json:"networkAccess,omitempty" yaml:"networkAccess,omitempty"`
	WritesExternalState   bool   `json:"writesExternalState,omitempty" yaml:"writesExternalState,omitempty"`
	HumanApprovalRequired bool   `json:"humanApprovalRequired,omitempty" yaml:"humanApprovalRequired,omitempty"`
	EvidenceRef           string `json:"evidenceRef" yaml:"evidenceRef"`
}

type AIFallbackBehavior struct {
	ID           string   `json:"id" yaml:"id"`
	UsageClassID string   `json:"usageClassId" yaml:"usageClassId"`
	Strategy     string   `json:"strategy" yaml:"strategy"`
	Models       []string `json:"models,omitempty" yaml:"models,omitempty"`
	ManualQueue  bool     `json:"manualQueue,omitempty" yaml:"manualQueue,omitempty"`
	MaxRetries   int      `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	EvidenceRef  string   `json:"evidenceRef" yaml:"evidenceRef"`
}

func New(project, source, site, collectorVersion string, collectedAt time.Time) *Inventory {
	return &Inventory{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Project:          project,
			Source:           source,
			CollectedAt:      collectedAt.UTC(),
			CollectorVersion: collectorVersion,
		},
		Source: Source{
			Type: source,
			Site: site,
		},
	}
}

func (inv *Inventory) RecomputeSummary() {
	metricNames := map[string]struct{}{}
	targets := map[string]struct{}{}
	for _, metric := range inv.Assets.Metrics {
		metricNames[metric.Name] = struct{}{}
	}
	for _, dashboard := range inv.Assets.Dashboards {
		for _, query := range dashboard.Queries {
			for _, metric := range ExtractMetricNames(query.Raw) {
				metricNames[metric] = struct{}{}
			}
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		for _, metric := range ExtractMetricNames(monitor.Query) {
			metricNames[metric] = struct{}{}
		}
		for _, target := range monitor.NotificationTargets {
			targets[target] = struct{}{}
		}
	}
	inv.Summary = Summary{
		Dashboards:                  len(inv.Assets.Dashboards),
		Monitors:                    len(inv.Assets.Monitors),
		SLOs:                        len(inv.Assets.SLOs),
		Integrations:                len(inv.Assets.Integrations),
		UniqueMetrics:               len(metricNames),
		NotificationTargets:         len(targets),
		Repositories:                len(inv.Assets.Repositories),
		Teams:                       len(inv.Assets.Teams),
		BranchProtections:           len(inv.Assets.BranchProtections),
		ActionsWorkflows:            len(inv.Assets.ActionsWorkflows),
		Secrets:                     len(inv.Assets.Secrets),
		Runners:                     len(inv.Assets.Runners),
		DeployKeys:                  len(inv.Assets.DeployKeys),
		GitHubApps:                  len(inv.Assets.GitHubApps),
		IdentityApps:                len(inv.Assets.IdentityApps),
		IdentityGroups:              len(inv.Assets.IdentityGroups),
		IdentityPolicies:            len(inv.Assets.IdentityPolicies),
		MFASettings:                 len(inv.Assets.MFASettings),
		BreakGlassAccounts:          len(inv.Assets.BreakGlassAccounts),
		DNSRecords:                  len(inv.Assets.DNSRecords),
		WAFRules:                    len(inv.Assets.WAFRules),
		CacheRules:                  len(inv.Assets.CacheRules),
		Redirects:                   len(inv.Assets.Redirects),
		Origins:                     len(inv.Assets.Origins),
		TLSSettings:                 len(inv.Assets.TLSSettings),
		BotRules:                    len(inv.Assets.BotRules),
		PageRules:                   len(inv.Assets.PageRules),
		AIModelUsageClasses:         len(inv.Assets.AIModelUsageClasses),
		AITokenVolumes:              len(inv.Assets.AITokenVolumes),
		AILatencyExpectations:       len(inv.Assets.AILatencyExpectations),
		AISensitivePromptCategories: len(inv.Assets.AISensitivePromptCategories),
		AIToolUsages:                len(inv.Assets.AIToolUsages),
		AIFallbackBehaviors:         len(inv.Assets.AIFallbackBehaviors),
	}
}
