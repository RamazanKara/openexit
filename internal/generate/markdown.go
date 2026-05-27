package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	openmapping "github.com/RamazanKara/openexit/internal/mapping"
	migrationplan "github.com/RamazanKara/openexit/internal/plan"
	"gopkg.in/yaml.v3"
)

type Context struct {
	ProjectDir string
	Inventory  *inventory.Inventory
	Assessment *assessment.Assessment
}

func Generate(projectDir string, artifacts []string) error {
	ctx, err := loadContext(projectDir)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		artifacts = markdownArtifacts
	}
	for _, artifact := range artifacts {
		switch artifact {
		case "all":
			if err := GenerateAll(projectDir); err != nil {
				return err
			}
		case "mapping":
			if err := writeMappingArtifact(ctx); err != nil {
				return err
			}
		case "assessment", "risk-register", "manual-review", "cost-drivers", "target-architecture", "acceptance-criteria", "rollback-plan", "runbook", "restore-drill-checklist", "alert-shadowing-plan", "forgejo-migration-assessment", "ci-compatibility-report", "branch-protection-mapping", "runner-migration-plan", "repository-ownership-report", "identity-migration-risk-register", "break-glass-checklist", "identity-cutover-plan", "identity-rollback-plan", "cache-parity-report", "waf-enforcement-risk-report", "self-hosted-llm-readiness-report", "vllm-sizing-assumptions", "evaluation-plan", "data-sensitivity-report":
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		case "migration-plan":
			if err := writeMigrationPlan(ctx); err != nil {
				return err
			}
		case "forgejo-migration-candidate":
			if err := writeForgejoMigrationCandidate(ctx); err != nil {
				return err
			}
		case "realm-client-candidate":
			if err := writeIdentityRealmClientCandidate(ctx); err != nil {
				return err
			}
		case "vcl-candidates":
			if err := writeEdgeVCLCandidates(ctx); err != nil {
				return err
			}
		case "haproxy-candidates":
			if err := writeEdgeHAProxyCandidates(ctx); err != nil {
				return err
			}
		case "coraza-rule-candidates":
			if err := writeEdgeCorazaCandidates(ctx); err != nil {
				return err
			}
		case "litellm-config-candidate":
			if err := writeAILiteLLMConfigCandidate(ctx); err != nil {
				return err
			}
		case "prometheus-rules":
			if err := Prometheus(ctx); err != nil {
				return err
			}
		case "grafana-dashboards":
			if err := Grafana(ctx); err != nil {
				return err
			}
		case "opentelemetry":
			if err := OpenTelemetry(ctx); err != nil {
				return err
			}
		case "argocd":
			if err := ArgoCD(ctx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown artifact %q", artifact)
		}
	}
	return nil
}

func GenerateAll(projectDir string) error {
	ctx, err := loadContext(projectDir)
	if err != nil {
		return err
	}
	if err := writeMappingArtifact(ctx); err != nil {
		return err
	}
	artifacts := markdownArtifacts
	if ctx.Inventory.Source.Type == "github-enterprise" {
		artifacts = []string{"assessment", "risk-register", "manual-review"}
	}
	if ctx.Inventory.Source.Type == "identity" {
		artifacts = []string{"assessment", "risk-register", "manual-review"}
	}
	if ctx.Inventory.Source.Type == "edge" {
		artifacts = []string{"assessment", "risk-register", "manual-review"}
	}
	if ctx.Inventory.Source.Type == "ai-provider" {
		artifacts = []string{"assessment", "risk-register", "manual-review"}
	}
	for _, artifact := range artifacts {
		if err := writeMarkdown(ctx, artifact); err != nil {
			return err
		}
	}
	if ctx.Inventory.Source.Type == "github-enterprise" {
		for _, artifact := range githubMarkdownArtifacts {
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		}
		if err := writeForgejoMigrationCandidate(ctx); err != nil {
			return err
		}
		return writeMigrationPlan(ctx)
	}
	if ctx.Inventory.Source.Type == "identity" {
		for _, artifact := range identityMarkdownArtifacts {
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		}
		if err := writeIdentityRealmClientCandidate(ctx); err != nil {
			return err
		}
		return writeMigrationPlan(ctx)
	}
	if ctx.Inventory.Source.Type == "edge" {
		for _, artifact := range edgeMarkdownArtifacts {
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		}
		if err := writeEdgeCandidateConfigs(ctx); err != nil {
			return err
		}
		return writeMigrationPlan(ctx)
	}
	if ctx.Inventory.Source.Type == "ai-provider" {
		for _, artifact := range aiMarkdownArtifacts {
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		}
		if err := writeAILiteLLMConfigCandidate(ctx); err != nil {
			return err
		}
		return writeMigrationPlan(ctx)
	}
	if err := Grafana(ctx); err != nil {
		return err
	}
	if err := Prometheus(ctx); err != nil {
		return err
	}
	if err := OpenTelemetry(ctx); err != nil {
		return err
	}
	if err := ArgoCD(ctx); err != nil {
		return err
	}
	return writeMigrationPlan(ctx)
}

func loadContext(projectDir string) (*Context, error) {
	invData, err := os.ReadFile(filepath.Join(projectDir, "inventory", "openexit.inventory.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}
	var inv inventory.Inventory
	if err := yaml.Unmarshal(invData, &inv); err != nil {
		return nil, fmt.Errorf("parse inventory: %w", err)
	}
	if err := inventory.Validate(&inv); err != nil {
		return nil, err
	}
	assessmentData, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read assessment: %w", err)
	}
	var a assessment.Assessment
	if err := yaml.Unmarshal(assessmentData, &a); err != nil {
		return nil, fmt.Errorf("parse assessment: %w", err)
	}
	if err := assessment.Validate(&a); err != nil {
		return nil, err
	}
	return &Context{ProjectDir: projectDir, Inventory: &inv, Assessment: &a}, nil
}

func writeMarkdown(ctx *Context, artifact string) error {
	var b bytes.Buffer
	title := titleCaseASCII(strings.ReplaceAll(artifact, "-", " "))
	fmt.Fprintf(&b, "# %s\n\n", title)
	writeCommonHeader(&b, ctx)
	switch artifact {
	case "assessment":
		writeAssessmentBody(&b, ctx)
	case "risk-register":
		writeRiskRegister(&b, ctx, nil)
	case "manual-review":
		writeManualReview(&b, ctx)
	case "cost-drivers":
		writeCostDrivers(&b, ctx)
	case "target-architecture":
		writeTargetArchitecture(&b, ctx)
	case "acceptance-criteria":
		writeAcceptanceCriteria(&b, ctx)
	case "rollback-plan":
		writeRollbackPlan(&b, ctx)
	case "runbook":
		writeRunbook(&b, ctx)
	case "restore-drill-checklist":
		writeRestoreDrill(&b, ctx)
	case "alert-shadowing-plan":
		writeAlertShadowing(&b, ctx)
	case "forgejo-migration-assessment":
		writeForgejoMigrationAssessment(&b, ctx)
	case "ci-compatibility-report":
		writeCICompatibilityReport(&b, ctx)
	case "branch-protection-mapping":
		writeBranchProtectionMapping(&b, ctx)
	case "runner-migration-plan":
		writeRunnerMigrationPlan(&b, ctx)
	case "repository-ownership-report":
		writeRepositoryOwnershipReport(&b, ctx)
	case "identity-migration-risk-register":
		writeIdentityMigrationRiskRegister(&b, ctx)
	case "break-glass-checklist":
		writeBreakGlassChecklist(&b, ctx)
	case "identity-cutover-plan":
		writeIdentityCutoverPlan(&b, ctx)
	case "identity-rollback-plan":
		writeIdentityRollbackPlan(&b, ctx)
	case "cache-parity-report":
		writeCacheParityReport(&b, ctx)
	case "waf-enforcement-risk-report":
		writeWAFEnforcementRiskReport(&b, ctx)
	case "self-hosted-llm-readiness-report":
		writeSelfHostedLLMReadinessReport(&b, ctx)
	case "vllm-sizing-assumptions":
		writeVLLMSizingAssumptions(&b, ctx)
	case "evaluation-plan":
		writeAIEvaluationPlan(&b, ctx)
	case "data-sensitivity-report":
		writeAIDataSensitivityReport(&b, ctx)
	}
	writeGeneratedBy(&b)
	path := filepath.Join(ctx.ProjectDir, "assessment", artifact+".md")
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func writeMigrationPlan(ctx *Context) error {
	p, err := migrationplan.Build(ctx.ProjectDir, ctx.Inventory, ctx.Assessment, ctx.Assessment.Metadata.GeneratedAt)
	if err != nil {
		return err
	}
	return migrationplan.Write(ctx.ProjectDir, p)
}

func writeMappingArtifact(ctx *Context) error {
	result, err := openmapping.Build(ctx.Inventory, ctx.Assessment, ctx.Assessment.Metadata.GeneratedAt)
	if err != nil {
		return err
	}
	return openmapping.Write(ctx.ProjectDir, result)
}

func writeCommonHeader(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintf(b, "- Project: %s\n", ctx.Assessment.Metadata.Project)
	fmt.Fprintf(b, "- Source: %s\n", ctx.Assessment.Source.Type)
	fmt.Fprintf(b, "- Target: %s\n", ctx.Assessment.Target.Type)
	fmt.Fprintf(b, "- Generated: %s\n", ctx.Assessment.Metadata.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(b, "- Complexity: %s (%d)\n\n", ctx.Assessment.Score.Level, ctx.Assessment.Score.Value)
	fmt.Fprintln(b, "## Assumptions")
	fmt.Fprintln(b, "- OpenExit is local-first and performs no production writes.")
	fmt.Fprintln(b, "- Generated target files are candidates and require human review.")
	fmt.Fprintln(b, "- Evidence references point to redacted local source snapshots.")
	fmt.Fprintln(b)
}

func writeAssessmentBody(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Inventory Summary")
	switch ctx.Inventory.Source.Type {
	case "github-enterprise":
		fmt.Fprintf(b, "- Repositories: %d\n", ctx.Inventory.Summary.Repositories)
		fmt.Fprintf(b, "- Teams: %d\n", ctx.Inventory.Summary.Teams)
		fmt.Fprintf(b, "- Branch protections: %d\n", ctx.Inventory.Summary.BranchProtections)
		fmt.Fprintf(b, "- Actions workflows: %d\n", ctx.Inventory.Summary.ActionsWorkflows)
		fmt.Fprintf(b, "- Secrets metadata entries: %d\n", ctx.Inventory.Summary.Secrets)
		fmt.Fprintf(b, "- Runners: %d\n\n", ctx.Inventory.Summary.Runners)
	case "identity":
		fmt.Fprintf(b, "- Identity applications: %d\n", ctx.Inventory.Summary.IdentityApps)
		fmt.Fprintf(b, "- Identity groups: %d\n", ctx.Inventory.Summary.IdentityGroups)
		fmt.Fprintf(b, "- Identity policies: %d\n", ctx.Inventory.Summary.IdentityPolicies)
		fmt.Fprintf(b, "- MFA settings: %d\n", ctx.Inventory.Summary.MFASettings)
		fmt.Fprintf(b, "- Break-glass accounts: %d\n\n", ctx.Inventory.Summary.BreakGlassAccounts)
	case "edge":
		fmt.Fprintf(b, "- DNS records: %d\n", ctx.Inventory.Summary.DNSRecords)
		fmt.Fprintf(b, "- WAF rules: %d\n", ctx.Inventory.Summary.WAFRules)
		fmt.Fprintf(b, "- Cache rules: %d\n", ctx.Inventory.Summary.CacheRules)
		fmt.Fprintf(b, "- Redirects: %d\n", ctx.Inventory.Summary.Redirects)
		fmt.Fprintf(b, "- Origins: %d\n", ctx.Inventory.Summary.Origins)
		fmt.Fprintf(b, "- TLS settings: %d\n", ctx.Inventory.Summary.TLSSettings)
		fmt.Fprintf(b, "- Bot rules: %d\n", ctx.Inventory.Summary.BotRules)
		fmt.Fprintf(b, "- Page rules: %d\n\n", ctx.Inventory.Summary.PageRules)
	case "ai-provider":
		fmt.Fprintf(b, "- Model usage classes: %d\n", ctx.Inventory.Summary.AIModelUsageClasses)
		fmt.Fprintf(b, "- Token volume profiles: %d\n", ctx.Inventory.Summary.AITokenVolumes)
		fmt.Fprintf(b, "- Latency expectations: %d\n", ctx.Inventory.Summary.AILatencyExpectations)
		fmt.Fprintf(b, "- Sensitive prompt categories: %d\n", ctx.Inventory.Summary.AISensitivePromptCategories)
		fmt.Fprintf(b, "- Tool usages: %d\n", ctx.Inventory.Summary.AIToolUsages)
		fmt.Fprintf(b, "- Fallback behaviors: %d\n\n", ctx.Inventory.Summary.AIFallbackBehaviors)
	default:
		fmt.Fprintf(b, "- Dashboards: %d\n", ctx.Inventory.Summary.Dashboards)
		fmt.Fprintf(b, "- Monitors: %d\n", ctx.Inventory.Summary.Monitors)
		fmt.Fprintf(b, "- SLOs: %d\n", ctx.Inventory.Summary.SLOs)
		fmt.Fprintf(b, "- Integrations: %d\n", ctx.Inventory.Summary.Integrations)
		fmt.Fprintf(b, "- Unique metrics: %d\n\n", ctx.Inventory.Summary.UniqueMetrics)
	}
	fmt.Fprintln(b, "## Complexity Drivers")
	for _, driver := range ctx.Assessment.Score.Drivers {
		fmt.Fprintf(b, "- %s\n", driver)
	}
	fmt.Fprintln(b)
	writeRiskRegister(b, ctx, firstN(ctx.Assessment.Findings, 10))
}

func writeRiskRegister(b *bytes.Buffer, ctx *Context, findings []assessment.Finding) {
	if findings == nil {
		findings = ctx.Assessment.Findings
	}
	fmt.Fprintln(b, "## Risks")
	if len(findings) == 0 {
		fmt.Fprintln(b, "- No material risks detected in the provided inventory.")
		fmt.Fprintln(b)
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "### %s\n", finding.ID)
		fmt.Fprintf(b, "- Severity: %s\n", finding.Severity)
		fmt.Fprintf(b, "- Title: %s\n", finding.Title)
		fmt.Fprintf(b, "- Affected assets: %s\n", strings.Join(finding.AffectedAssets, ", "))
		fmt.Fprintf(b, "- Evidence: %s\n", strings.Join(finding.EvidenceRefs, ", "))
		fmt.Fprintf(b, "- Recommendation: %s\n\n", finding.Recommendation)
	}
}

func writeManualReview(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Manual Review Required")
	count := 0
	for _, finding := range ctx.Assessment.Findings {
		if finding.Severity == "high" || strings.Contains(strings.ToLower(finding.ID), "manual") || strings.Contains(strings.ToLower(finding.Title), "manual") {
			count++
			fmt.Fprintf(b, "- %s: %s (%s)\n", finding.ID, finding.Title, strings.Join(finding.AffectedAssets, ", "))
		}
	}
	if count == 0 {
		fmt.Fprintln(b, "- No high-severity manual review findings were detected.")
	}
	fmt.Fprintln(b)
}

func writeCostDrivers(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Cost And Scale Drivers")
	fmt.Fprintf(b, "- Unique metrics: %d\n", ctx.Inventory.Summary.UniqueMetrics)
	fmt.Fprintf(b, "- Notification targets: %d\n", ctx.Inventory.Summary.NotificationTargets)
	for _, finding := range ctx.Assessment.Findings {
		if strings.Contains(finding.ID, "cost") {
			fmt.Fprintf(b, "- %s: %s\n", finding.ID, finding.Title)
		}
	}
	fmt.Fprintln(b)
}

func writeTargetArchitecture(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Target Architecture")
	fmt.Fprintln(b, "- Grafana dashboards are generated as candidate JSON under generated-config/grafana.")
	fmt.Fprintln(b, "- Prometheus-compatible alert rule candidates are generated under generated-config/prometheus.")
	fmt.Fprintln(b, "- OpenTelemetry Collector or Grafana Alloy sketch receives OTLP metrics and traces.")
	fmt.Fprintln(b, "- Mimir, Loki, Tempo, and Grafana sizing must be validated against real retention and volume requirements.")
	fmt.Fprintln(b)
}

func writeAcceptanceCriteria(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Acceptance Criteria")
	fmt.Fprintln(b, "- All candidate dashboards are reviewed by an owner.")
	fmt.Fprintln(b, "- All alert candidates shadow production Datadog alerts before cutover.")
	fmt.Fprintln(b, "- SLO semantics, burn-rate behavior, and notification routes are signed off.")
	fmt.Fprintln(b, "- Validation passes with no critical findings.")
	fmt.Fprintln(b)
}

func writeRollbackPlan(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Rollback Plan")
	fmt.Fprintln(b, "- Keep Datadog monitors active until target alerting has completed a shadow period.")
	fmt.Fprintln(b, "- Disable candidate notification routes before reverting alert traffic.")
	fmt.Fprintln(b, "- Preserve source evidence and generated configs for post-rollback analysis.")
	fmt.Fprintln(b, "- Roll back dashboard links and runbook references to Datadog until parity is restored.")
	fmt.Fprintln(b)
}

func writeRunbook(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Runbook")
	fmt.Fprintln(b, "1. Review the risk register and manual-review list.")
	fmt.Fprintln(b, "2. Import candidate dashboards into a non-production Grafana folder.")
	fmt.Fprintln(b, "3. Load Prometheus rule candidates into a test ruler or promtool check.")
	fmt.Fprintln(b, "4. Run alert shadowing and compare behavior with Datadog.")
	fmt.Fprintln(b, "5. Record owner approvals before any production routing change.")
	fmt.Fprintln(b)
}

func writeRestoreDrill(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Restore Drill Checklist")
	fmt.Fprintln(b, "- Confirm source Datadog access and monitor state.")
	fmt.Fprintln(b, "- Confirm target Grafana, Mimir, Tempo, Loki, and Collector backup procedures.")
	fmt.Fprintln(b, "- Restore candidate configs into an isolated environment.")
	fmt.Fprintln(b, "- Verify dashboards, rules, and evidence refs after restore.")
	fmt.Fprintln(b)
}

func writeAlertShadowing(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Alert Shadowing Plan")
	fmt.Fprintln(b, "- Run target alert candidates without production paging.")
	fmt.Fprintln(b, "- Compare Datadog and target alert state over at least one representative period.")
	fmt.Fprintln(b, "- Investigate every high-severity finding before enabling notifications.")
	fmt.Fprintln(b, "- Document acceptable differences and owner sign-off.")
	fmt.Fprintln(b)
}

func writeForgejoMigrationAssessment(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Forgejo Migration Assessment")
	fmt.Fprintf(b, "- Repositories: %d\n", ctx.Inventory.Summary.Repositories)
	fmt.Fprintf(b, "- Teams: %d\n", ctx.Inventory.Summary.Teams)
	fmt.Fprintf(b, "- Branch protections: %d\n", ctx.Inventory.Summary.BranchProtections)
	fmt.Fprintf(b, "- Actions workflows: %d\n", ctx.Inventory.Summary.ActionsWorkflows)
	fmt.Fprintf(b, "- GitHub Apps: %d\n\n", ctx.Inventory.Summary.GitHubApps)
	writeRiskRegister(b, ctx, firstN(ctx.Assessment.Findings, 12))
}

func writeCICompatibilityReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## CI Compatibility")
	if len(ctx.Inventory.Assets.ActionsWorkflows) == 0 {
		fmt.Fprintln(b, "- No Actions workflow metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, workflow := range ctx.Inventory.Assets.ActionsWorkflows {
		fmt.Fprintf(b, "- %s %s: github-hosted=%t self-hosted=%t actions=%s evidence=%s\n", workflow.Repository, workflow.Path, workflow.UsesGitHubHosted, workflow.UsesSelfHosted, strings.Join(workflow.Actions, ", "), workflow.EvidenceRef)
	}
	fmt.Fprintln(b)
}

func writeBranchProtectionMapping(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Branch Protection Mapping")
	if len(ctx.Inventory.Assets.BranchProtections) == 0 {
		fmt.Fprintln(b, "- No branch protection metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, protection := range ctx.Inventory.Assets.BranchProtections {
		fmt.Fprintf(b, "- %s/%s: reviews=%d code-owner-review=%t force-push=%t checks=%s evidence=%s\n", protection.Repository, protection.Branch, protection.RequiredReviews, protection.RequireCodeOwnerReview, protection.AllowsForcePushes, strings.Join(protection.RequiredStatusChecks, ", "), protection.EvidenceRef)
	}
	fmt.Fprintln(b)
}

func writeRunnerMigrationPlan(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Runner Migration Plan")
	if len(ctx.Inventory.Assets.Runners) == 0 {
		fmt.Fprintln(b, "- No runner metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, runner := range ctx.Inventory.Assets.Runners {
		fmt.Fprintf(b, "- %s (%s): online=%t labels=%s evidence=%s\n", runner.Name, runner.Scope, runner.Online, strings.Join(runner.Labels, ", "), runner.EvidenceRef)
	}
	fmt.Fprintln(b)
}

func writeRepositoryOwnershipReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Repository Ownership")
	if len(ctx.Inventory.Assets.Repositories) == 0 {
		fmt.Fprintln(b, "- No repository metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, repo := range ctx.Inventory.Assets.Repositories {
		fmt.Fprintf(b, "- %s: teams=%s codeowners=%t visibility=%s evidence=%s\n", repo.Name, strings.Join(repo.Teams, ", "), repo.HasCODEOWNERS, repo.Visibility, repo.EvidenceRef)
	}
	fmt.Fprintln(b)
}

func writeForgejoMigrationCandidate(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "forgejo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candidate := map[string]any{
		"apiVersion": "openexit.dev/v1alpha1",
		"kind":       "ForgejoMigrationCandidate",
		"metadata": map[string]any{
			"project":    ctx.Assessment.Metadata.Project,
			"source":     ctx.Inventory.Source.Type,
			"sourceHost": ctx.Inventory.Source.Site,
			"target":     ctx.Assessment.Target.Type,
		},
		"repositories":        forgejoRepositoryCandidates(ctx),
		"teams":               forgejoTeamCandidates(ctx.Inventory.Assets.Teams),
		"branchProtections":   forgejoBranchProtectionCandidates(ctx.Inventory.Assets.BranchProtections),
		"ciWorkflows":         forgejoWorkflowCandidates(ctx.Inventory.Assets.ActionsWorkflows),
		"secretMetadata":      forgejoSecretMetadataCandidates(ctx.Inventory.Assets.Secrets),
		"runnerMigration":     forgejoRunnerCandidates(ctx.Inventory.Assets.Runners),
		"deployKeyReview":     forgejoDeployKeyCandidates(ctx.Inventory.Assets.DeployKeys),
		"integrationReview":   forgejoAppCandidates(ctx.Inventory.Assets.GitHubApps),
		"manualReview":        forgejoManualReviewItems(ctx.Assessment.Findings),
		"credentialsIncluded": false,
		"productionReady":     false,
	}
	data, err := yaml.Marshal(candidate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "migration-candidate.yaml"), data, 0o644); err != nil {
		return err
	}
	readme := "# Forgejo Migration Candidate\n\nThis directory contains a deterministic Forgejo migration candidate derived from redacted GitHub Enterprise inventory. It contains no repository contents or secret values. Review repository ownership, branch protection behavior, CI compatibility, runners, deploy keys, and app integrations before migration.\n"
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}

func forgejoRepositoryCandidates(ctx *Context) []map[string]any {
	workflows := workflowsByRepository(ctx.Inventory.Assets.ActionsWorkflows)
	protections := branchProtectionsByRepository(ctx.Inventory.Assets.BranchProtections)
	secretCounts := secretCountByRepository(ctx.Inventory.Assets.Secrets)
	out := make([]map[string]any, 0, len(ctx.Inventory.Assets.Repositories))
	for _, repo := range ctx.Inventory.Assets.Repositories {
		owner, name := splitRepositoryName(repo.Name)
		item := map[string]any{
			"name":                  repo.Name,
			"targetOwner":           owner,
			"targetName":            name,
			"visibility":            repo.Visibility,
			"defaultBranch":         repo.DefaultBranch,
			"archived":              repo.Archived,
			"topics":                sortedStrings(repo.Topics),
			"teams":                 sortedStrings(repo.Teams),
			"actionsEnabled":        repo.ActionsEnabled,
			"hasCodeowners":         repo.HasCODEOWNERS,
			"usesGitHubPages":       repo.UsesGitHubPages,
			"usesGitHubPackages":    repo.UsesGitHubPackages,
			"usesGitHubDiscussions": repo.UsesGitHubDiscussions,
			"sourceCloneRef":        sourceCloneRef(ctx.Inventory.Source.Site, repo.Name),
			"targetRepository":      repo.Name,
			"branchProtections":     protections[repo.Name],
			"ciWorkflows":           workflows[repo.Name],
			"secretMetadataCount":   secretCounts[repo.Name],
			"reviewRequired":        forgejoRepositoryReviewRequired(repo),
			"evidenceRef":           repo.EvidenceRef,
		}
		out = append(out, item)
	}
	return out
}

func forgejoTeamCandidates(teams []inventory.Team) []map[string]any {
	out := make([]map[string]any, 0, len(teams))
	for _, team := range teams {
		out = append(out, map[string]any{
			"slug":         team.Slug,
			"name":         team.Name,
			"members":      team.Members,
			"maintainers":  team.Maintainers,
			"repositories": sortedStrings(team.Repos),
			"evidenceRef":  team.EvidenceRef,
		})
	}
	return out
}

func forgejoBranchProtectionCandidates(protections []inventory.BranchProtection) []map[string]any {
	out := make([]map[string]any, 0, len(protections))
	for _, protection := range protections {
		out = append(out, map[string]any{
			"repository":             protection.Repository,
			"branch":                 protection.Branch,
			"requiredStatusChecks":   sortedStrings(protection.RequiredStatusChecks),
			"requiredReviews":        protection.RequiredReviews,
			"requireCodeOwnerReview": protection.RequireCodeOwnerReview,
			"restrictPushes":         protection.RestrictPushes,
			"allowsForcePushes":      protection.AllowsForcePushes,
			"targetReview":           !protection.RequireCodeOwnerReview || protection.AllowsForcePushes || protection.RequiredReviews == 0,
			"evidenceRef":            protection.EvidenceRef,
		})
	}
	return out
}

func forgejoWorkflowCandidates(workflows []inventory.ActionsWorkflow) []map[string]any {
	out := make([]map[string]any, 0, len(workflows))
	for _, workflow := range workflows {
		out = append(out, map[string]any{
			"repository":       workflow.Repository,
			"path":             workflow.Path,
			"name":             workflow.Name,
			"usesGitHubHosted": workflow.UsesGitHubHosted,
			"usesSelfHosted":   workflow.UsesSelfHosted,
			"actions":          sortedStrings(workflow.Actions),
			"secretNames":      sortedStrings(workflow.Secrets),
			"targetRunnerPlan": forgejoRunnerPlan(workflow),
			"evidenceRef":      workflow.EvidenceRef,
		})
	}
	return out
}

func forgejoSecretMetadataCandidates(secrets []inventory.SecretMetadata) []map[string]any {
	out := make([]map[string]any, 0, len(secrets))
	for _, secret := range secrets {
		out = append(out, map[string]any{
			"name":        secret.Name,
			"scope":       secret.Scope,
			"repository":  secret.Repository,
			"consumers":   sortedStrings(secret.Consumers),
			"valueStatus": "not-collected",
			"evidenceRef": secret.EvidenceRef,
		})
	}
	return out
}

func forgejoRunnerCandidates(runners []inventory.Runner) []map[string]any {
	out := make([]map[string]any, 0, len(runners))
	for _, runner := range runners {
		out = append(out, map[string]any{
			"name":            runner.Name,
			"scope":           runner.Scope,
			"labels":          sortedStrings(runner.Labels),
			"online":          runner.Online,
			"targetAction":    forgejoRunnerAction(runner),
			"registrationRef": "create-runner-registration-during-cutover",
			"evidenceRef":     runner.EvidenceRef,
		})
	}
	return out
}

func forgejoDeployKeyCandidates(keys []inventory.DeployKey) []map[string]any {
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, map[string]any{
			"repository":   key.Repository,
			"title":        key.Title,
			"readOnly":     key.ReadOnly,
			"targetAction": forgejoDeployKeyAction(key),
			"evidenceRef":  key.EvidenceRef,
		})
	}
	return out
}

func forgejoAppCandidates(apps []inventory.GitHubApp) []map[string]any {
	out := make([]map[string]any, 0, len(apps))
	for _, app := range apps {
		out = append(out, map[string]any{
			"name":                app.Name,
			"repositories":        sortedStrings(app.Repositories),
			"repositorySelection": app.RepositorySelection,
			"permissions":         sortedStrings(app.Permissions),
			"events":              sortedStrings(app.Events),
			"webhookEnabled":      app.WebhookEnabled,
			"targetAction":        "map-to-forgejo-integration-or-replace",
			"evidenceRef":         app.EvidenceRef,
		})
	}
	return out
}

func forgejoManualReviewItems(findings []assessment.Finding) []map[string]any {
	out := []map[string]any{}
	for _, finding := range findings {
		if !strings.HasPrefix(finding.ID, "ghe.") || finding.Severity == "low" {
			continue
		}
		out = append(out, map[string]any{
			"id":             finding.ID,
			"severity":       finding.Severity,
			"title":          finding.Title,
			"affectedAssets": sortedStrings(finding.AffectedAssets),
			"evidenceRefs":   sortedStrings(finding.EvidenceRefs),
			"recommendation": finding.Recommendation,
		})
	}
	return out
}

func workflowsByRepository(workflows []inventory.ActionsWorkflow) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, workflow := range workflows {
		out[workflow.Repository] = append(out[workflow.Repository], map[string]any{
			"path":             workflow.Path,
			"name":             workflow.Name,
			"usesGitHubHosted": workflow.UsesGitHubHosted,
			"usesSelfHosted":   workflow.UsesSelfHosted,
			"actions":          sortedStrings(workflow.Actions),
			"evidenceRef":      workflow.EvidenceRef,
		})
	}
	return out
}

func branchProtectionsByRepository(protections []inventory.BranchProtection) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, protection := range protections {
		out[protection.Repository] = append(out[protection.Repository], map[string]any{
			"branch":                 protection.Branch,
			"requiredStatusChecks":   sortedStrings(protection.RequiredStatusChecks),
			"requiredReviews":        protection.RequiredReviews,
			"requireCodeOwnerReview": protection.RequireCodeOwnerReview,
			"restrictPushes":         protection.RestrictPushes,
			"allowsForcePushes":      protection.AllowsForcePushes,
			"evidenceRef":            protection.EvidenceRef,
		})
	}
	return out
}

func secretCountByRepository(secrets []inventory.SecretMetadata) map[string]int {
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

func writeIdentityMigrationRiskRegister(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Identity Migration Risk Register")
	fmt.Fprintf(b, "- Applications: %d\n", ctx.Inventory.Summary.IdentityApps)
	fmt.Fprintf(b, "- Groups: %d\n", ctx.Inventory.Summary.IdentityGroups)
	fmt.Fprintf(b, "- Policies: %d\n", ctx.Inventory.Summary.IdentityPolicies)
	fmt.Fprintf(b, "- MFA settings: %d\n\n", ctx.Inventory.Summary.MFASettings)
	writeRiskRegister(b, ctx, ctx.Assessment.Findings)
}

func writeBreakGlassChecklist(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Break-Glass Checklist")
	if len(ctx.Inventory.Assets.BreakGlassAccounts) == 0 {
		fmt.Fprintln(b, "- No break-glass account metadata was captured.")
	} else {
		for _, account := range ctx.Inventory.Assets.BreakGlassAccounts {
			fmt.Fprintf(b, "- %s: mfa-enabled=%t managed-by=%s evidence=%s\n", account.Username, account.MFAEnabled, account.ManagedBy, account.EvidenceRef)
		}
	}
	fmt.Fprintln(b, "- Confirm emergency account custody and approval path.")
	fmt.Fprintln(b, "- Test emergency login against the target realm before cutover.")
	fmt.Fprintln(b, "- Store recovery codes according to the organization's access-control policy.")
	fmt.Fprintln(b)
}

func writeIdentityCutoverPlan(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Cutover Plan")
	fmt.Fprintln(b, "1. Freeze source application and policy changes for the approved cutover window.")
	fmt.Fprintln(b, "2. Import candidate groups, clients, redirect URIs, and MFA settings into a non-production target realm.")
	fmt.Fprintln(b, "3. Validate SAML metadata, OIDC grant types, and redirect URI behavior with application owners.")
	fmt.Fprintln(b, "4. Run parallel login tests for high-priority applications and privileged groups.")
	fmt.Fprintln(b, "5. Switch DNS, issuer, or federation configuration only after owner sign-off is recorded.")
	fmt.Fprintln(b)
}

func writeIdentityRollbackPlan(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Rollback Plan")
	fmt.Fprintln(b, "- Preserve source Okta/Auth0 tenant configuration until target sign-in is accepted.")
	fmt.Fprintln(b, "- Keep source SAML metadata and OIDC client settings available for reactivation.")
	fmt.Fprintln(b, "- Revert issuer, callback, and federation changes in reverse cutover order.")
	fmt.Fprintln(b, "- Disable target clients that received production traffic during the failed cutover.")
	fmt.Fprintln(b, "- Use the verified break-glass account if administrative SSO is impaired.")
	fmt.Fprintln(b)
}

func writeIdentityRealmClientCandidate(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "identity")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candidate := map[string]any{
		"apiVersion": "openexit.dev/v1alpha1",
		"kind":       "IdentityRealmClientCandidate",
		"metadata": map[string]any{
			"project": ctx.Assessment.Metadata.Project,
			"source":  ctx.Inventory.Source.Type,
			"target":  ctx.Assessment.Target.Type,
		},
		"realm": map[string]any{
			"name":     inventory.Slug(ctx.Assessment.Metadata.Project),
			"provider": ctx.Inventory.Source.Site,
		},
		"clients":            identityClientCandidates(ctx.Inventory.Assets.IdentityApps),
		"groups":             identityGroupCandidates(ctx.Inventory.Assets.IdentityGroups),
		"policies":           identityPolicyCandidates(ctx.Inventory.Assets.IdentityPolicies),
		"mfaSettings":        identityMFACandidates(ctx.Inventory.Assets.MFASettings),
		"breakGlassAccounts": identityBreakGlassCandidates(ctx.Inventory.Assets.BreakGlassAccounts),
	}
	data, err := yaml.Marshal(candidate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "realm-client-candidate.yaml"), data, 0o644); err != nil {
		return err
	}
	readme := "# Identity Candidate Config\n\nThis directory contains deterministic Keycloak/Zitadel candidate configuration derived from redacted identity inventory. Review every client, redirect URI, policy, and break-glass account before use.\n"
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}

func identityClientCandidates(apps []inventory.IdentityApp) []map[string]any {
	out := make([]map[string]any, 0, len(apps))
	for _, app := range apps {
		out = append(out, map[string]any{
			"id":                       app.ID,
			"name":                     app.Name,
			"protocol":                 app.Protocol,
			"clientId":                 app.ClientID,
			"owners":                   sortedStrings(app.Owners),
			"groups":                   sortedStrings(app.Groups),
			"redirectUris":             sortedStrings(app.RedirectURIs),
			"grantTypes":               sortedStrings(app.GrantTypes),
			"samlSigningCertPresent":   app.SAMLSigningCertPresent,
			"samlSigningCertExpiresAt": app.SAMLSigningCertExpiresAt,
			"evidenceRef":              app.EvidenceRef,
		})
	}
	return out
}

func identityGroupCandidates(groups []inventory.IdentityGroup) []map[string]any {
	out := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		out = append(out, map[string]any{
			"id":          group.ID,
			"name":        group.Name,
			"owners":      sortedStrings(group.Owners),
			"members":     group.Members,
			"evidenceRef": group.EvidenceRef,
		})
	}
	return out
}

func identityPolicyCandidates(policies []inventory.IdentityPolicy) []map[string]any {
	out := make([]map[string]any, 0, len(policies))
	for _, policy := range policies {
		out = append(out, map[string]any{
			"id":           policy.ID,
			"name":         policy.Name,
			"type":         policy.Type,
			"groups":       sortedStrings(policy.Groups),
			"applications": sortedStrings(policy.Applications),
			"enforcesMfa":  policy.EnforcesMFA,
			"priority":     policy.Priority,
			"evidenceRef":  policy.EvidenceRef,
		})
	}
	return out
}

func identityMFACandidates(settings []inventory.MFASetting) []map[string]any {
	out := make([]map[string]any, 0, len(settings))
	for _, setting := range settings {
		out = append(out, map[string]any{
			"name":        setting.Name,
			"required":    setting.Required,
			"factors":     sortedStrings(setting.Factors),
			"evidenceRef": setting.EvidenceRef,
		})
	}
	return out
}

func identityBreakGlassCandidates(accounts []inventory.BreakGlassAccount) []map[string]any {
	out := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, map[string]any{
			"username":    account.Username,
			"mfaEnabled":  account.MFAEnabled,
			"managedBy":   account.ManagedBy,
			"evidenceRef": account.EvidenceRef,
		})
	}
	return out
}

func writeCacheParityReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Cache Parity")
	if len(ctx.Inventory.Assets.CacheRules) == 0 && len(ctx.Inventory.Assets.PageRules) == 0 {
		fmt.Fprintln(b, "- No cache or page-rule metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, rule := range ctx.Inventory.Assets.CacheRules {
		fmt.Fprintf(b, "- %s: pattern=%s action=%s edge-ttl=%ds browser-ttl=%ds cache-key=%s evidence=%s\n", rule.Name, rule.Pattern, rule.Action, rule.EdgeTTL, rule.BrowserTTL, strings.Join(rule.CacheKey, ", "), rule.EvidenceRef)
	}
	for _, rule := range ctx.Inventory.Assets.PageRules {
		fmt.Fprintf(b, "- page-rule %s: target=%s actions=%s evidence=%s\n", rule.ID, rule.Target, strings.Join(rule.Actions, ", "), rule.EvidenceRef)
	}
	writeFilteredFindings(b, ctx, "edge.cache.", "edge.page-rule.")
}

func writeWAFEnforcementRiskReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## WAF Enforcement Risk")
	if len(ctx.Inventory.Assets.WAFRules) == 0 && len(ctx.Inventory.Assets.BotRules) == 0 {
		fmt.Fprintln(b, "- No WAF or bot rule metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	for _, rule := range ctx.Inventory.Assets.WAFRules {
		fmt.Fprintf(b, "- WAF %s: enabled=%t action=%s managed=%t evidence=%s\n", rule.Name, rule.Enabled, rule.Action, rule.Managed, rule.EvidenceRef)
	}
	for _, rule := range ctx.Inventory.Assets.BotRules {
		fmt.Fprintf(b, "- Bot %s: enabled=%t action=%s evidence=%s\n", rule.Name, rule.Enabled, rule.Action, rule.EvidenceRef)
	}
	writeFilteredFindings(b, ctx, "edge.waf.", "edge.bot.", "edge.tls.", "edge.origin.")
}

func writeSelfHostedLLMReadinessReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Self-Hosted LLM Readiness")
	fmt.Fprintf(b, "- Model usage classes: %d\n", ctx.Inventory.Summary.AIModelUsageClasses)
	fmt.Fprintf(b, "- Token volume profiles: %d\n", ctx.Inventory.Summary.AITokenVolumes)
	fmt.Fprintf(b, "- Latency expectations: %d\n", ctx.Inventory.Summary.AILatencyExpectations)
	fmt.Fprintf(b, "- Sensitive prompt categories: %d\n", ctx.Inventory.Summary.AISensitivePromptCategories)
	fmt.Fprintf(b, "- Tool usages: %d\n", ctx.Inventory.Summary.AIToolUsages)
	fmt.Fprintf(b, "- Fallback behaviors: %d\n\n", ctx.Inventory.Summary.AIFallbackBehaviors)
	if len(ctx.Inventory.Assets.AIModelUsageClasses) == 0 {
		fmt.Fprintln(b, "- No AI provider usage metadata was captured.")
		fmt.Fprintln(b)
		return
	}
	volumes := tokenVolumeByUsageClass(ctx.Inventory.Assets.AITokenVolumes)
	latencies := latencyByUsageClass(ctx.Inventory.Assets.AILatencyExpectations)
	for _, usageClass := range ctx.Inventory.Assets.AIModelUsageClasses {
		fmt.Fprintf(b, "### %s\n", usageClass.Name)
		fmt.Fprintf(b, "- Source provider: %s\n", usageClass.Provider)
		fmt.Fprintf(b, "- Source models: %s\n", modelList(usageClass.Models))
		fmt.Fprintf(b, "- Owners: %s\n", modelList(usageClass.Owners))
		if volume, ok := volumes[usageClass.ID]; ok {
			fmt.Fprintf(b, "- Monthly model tokens: %d\n", volume.MonthlyInputTokens+volume.MonthlyOutputTokens)
			fmt.Fprintf(b, "- Peak tokens per minute: %d\n", volume.PeakTokensPerMinute)
		}
		if latency, ok := latencies[usageClass.ID]; ok {
			fmt.Fprintf(b, "- p95 latency expectation: %d ms\n", latency.P95Ms)
			fmt.Fprintf(b, "- Streaming required: %t\n", latency.StreamingRequired)
		}
		fmt.Fprintf(b, "- Evidence: %s\n\n", usageClass.EvidenceRef)
	}
	writeFilteredFindings(b, ctx, "ai.")
}

func writeVLLMSizingAssumptions(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## vLLM Sizing Assumptions")
	fmt.Fprintln(b, "- Capacity estimates are placeholders based on captured peak tokens per minute.")
	fmt.Fprintln(b, "- Final sizing requires benchmark data for the selected open-weight model, context length, batching policy, and hardware.")
	fmt.Fprintln(b)
	volumes := tokenVolumeByUsageClass(ctx.Inventory.Assets.AITokenVolumes)
	if len(volumes) == 0 {
		fmt.Fprintln(b, "- No token volume profiles were captured.")
		fmt.Fprintln(b)
		return
	}
	for _, usageClass := range ctx.Inventory.Assets.AIModelUsageClasses {
		volume, ok := volumes[usageClass.ID]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "- %s: peak=%d tokens/minute, baseline-vllm-replicas=%d, monthly-total=%d, evidence=%s\n", usageClass.Name, volume.PeakTokensPerMinute, recommendedVLLMReplicas(volume.PeakTokensPerMinute), volume.MonthlyInputTokens+volume.MonthlyOutputTokens, volume.EvidenceRef)
	}
	fmt.Fprintln(b)
}

func writeAIEvaluationPlan(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Evaluation Plan")
	if len(ctx.Inventory.Assets.AIModelUsageClasses) == 0 {
		fmt.Fprintln(b, "- No model usage classes were captured.")
		fmt.Fprintln(b)
		return
	}
	latencies := latencyByUsageClass(ctx.Inventory.Assets.AILatencyExpectations)
	for _, usageClass := range ctx.Inventory.Assets.AIModelUsageClasses {
		fmt.Fprintf(b, "### %s\n", usageClass.Name)
		fmt.Fprintln(b, "1. Build a redacted golden prompt set for this usage class.")
		fmt.Fprintln(b, "2. Compare source outputs against at least two candidate self-hosted models.")
		fmt.Fprintln(b, "3. Score quality, latency, refusal behavior, tool-call behavior, and fallback behavior.")
		if latency, ok := latencies[usageClass.ID]; ok {
			fmt.Fprintf(b, "4. Run load tests against p95=%d ms and timeout=%d ms expectations.\n", latency.P95Ms, latency.TimeoutMs)
		} else {
			fmt.Fprintln(b, "4. Capture latency expectations before declaring evaluation complete.")
		}
		fmt.Fprintf(b, "- Sensitive categories: %s\n", modelList(usageClass.SensitivePromptCategories))
		fmt.Fprintf(b, "- Tools: %s\n", modelList(usageClass.ToolRefs))
		fmt.Fprintf(b, "- Evidence: %s\n\n", usageClass.EvidenceRef)
	}
}

func writeAIDataSensitivityReport(b *bytes.Buffer, ctx *Context) {
	fmt.Fprintln(b, "## Data Sensitivity")
	if len(ctx.Inventory.Assets.AISensitivePromptCategories) == 0 {
		fmt.Fprintln(b, "- No sensitive prompt categories were captured.")
		fmt.Fprintln(b)
		return
	}
	for _, category := range ctx.Inventory.Assets.AISensitivePromptCategories {
		fmt.Fprintf(b, "- %s (%s): usage-class=%s pii=%t residency=%t retention=%s evidence=%s\n", category.Category, category.DataClass, category.UsageClassID, category.ContainsPII, category.RequiresDataResidency, category.Retention, category.EvidenceRef)
	}
	fmt.Fprintln(b)
	writeFilteredFindings(b, ctx, "ai.data.")
}

func writeAILiteLLMConfigCandidate(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "ai", "litellm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candidate := map[string]any{
		"apiVersion": "openexit.dev/v1alpha1",
		"kind":       "LiteLLMConfigCandidate",
		"metadata": map[string]any{
			"project": ctx.Assessment.Metadata.Project,
			"source":  ctx.Inventory.Source.Type,
			"target":  ctx.Assessment.Target.Type,
		},
		"model_list":             aiModelRouteCandidates(ctx),
		"router":                 aiRouterCandidate(ctx),
		"vllmSizingAssumptions":  aiVLLMSizingCandidates(ctx),
		"humanReviewRequired":    true,
		"credentialsIncluded":    false,
		"productionReady":        false,
		"candidateGeneratedFrom": "redacted-openexit-inventory",
	}
	data, err := yaml.Marshal(candidate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.candidate.yaml"), data, 0o644); err != nil {
		return err
	}
	readme := "# LiteLLM Config Candidate\n\nThis directory contains a deterministic LiteLLM routing candidate for vLLM-backed models. It intentionally contains no provider credentials and is not production-ready until model quality, latency, capacity, and tool controls are reviewed.\n"
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}

func aiModelRouteCandidates(ctx *Context) []map[string]any {
	volumes := tokenVolumeByUsageClass(ctx.Inventory.Assets.AITokenVolumes)
	latencies := latencyByUsageClass(ctx.Inventory.Assets.AILatencyExpectations)
	fallbacks := fallbackByUsageClass(ctx.Inventory.Assets.AIFallbackBehaviors)
	out := make([]map[string]any, 0, len(ctx.Inventory.Assets.AIModelUsageClasses))
	for _, usageClass := range ctx.Inventory.Assets.AIModelUsageClasses {
		route := map[string]any{
			"model_name":   inventory.Slug(usageClass.Name),
			"sourceModels": sortedStrings(usageClass.Models),
			"owners":       sortedStrings(usageClass.Owners),
			"litellm_params": map[string]any{
				"model":         "vllm/" + targetModelPlaceholder(usageClass),
				"apiBaseRef":    "vllm-service-" + inventory.Slug(usageClass.ID),
				"credentialRef": "runtime-managed-credential",
			},
			"evidenceRef": usageClass.EvidenceRef,
		}
		if volume, ok := volumes[usageClass.ID]; ok {
			route["peakTokensPerMinute"] = volume.PeakTokensPerMinute
		}
		if latency, ok := latencies[usageClass.ID]; ok {
			route["p95Ms"] = latency.P95Ms
			route["streamingRequired"] = latency.StreamingRequired
		}
		if fallback, ok := fallbacks[usageClass.ID]; ok {
			route["fallbackStrategy"] = fallback.Strategy
			route["manualQueue"] = fallback.ManualQueue
		}
		out = append(out, route)
	}
	return out
}

func aiRouterCandidate(ctx *Context) map[string]any {
	return map[string]any{
		"routingStrategy": "explicit-model-aliases",
		"gateway":         "litellm",
		"targetRuntime":   "vllm",
		"toolPolicy":      "deny-write-tools-until-reviewed",
		"evidenceBacked":  ctx.Inventory.Summary.AIModelUsageClasses > 0,
	}
}

func aiVLLMSizingCandidates(ctx *Context) []map[string]any {
	volumes := tokenVolumeByUsageClass(ctx.Inventory.Assets.AITokenVolumes)
	out := make([]map[string]any, 0, len(ctx.Inventory.Assets.AIModelUsageClasses))
	for _, usageClass := range ctx.Inventory.Assets.AIModelUsageClasses {
		volume, ok := volumes[usageClass.ID]
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"usageClassId":        usageClass.ID,
			"route":               inventory.Slug(usageClass.Name),
			"monthlyModelTokens":  volume.MonthlyInputTokens + volume.MonthlyOutputTokens,
			"peakTokensPerMinute": volume.PeakTokensPerMinute,
			"baselineReplicas":    recommendedVLLMReplicas(volume.PeakTokensPerMinute),
			"evidenceRef":         volume.EvidenceRef,
		})
	}
	return out
}

func tokenVolumeByUsageClass(volumes []inventory.AITokenVolume) map[string]inventory.AITokenVolume {
	out := map[string]inventory.AITokenVolume{}
	for _, volume := range volumes {
		out[volume.UsageClassID] = volume
	}
	return out
}

func latencyByUsageClass(latencies []inventory.AILatencyExpectation) map[string]inventory.AILatencyExpectation {
	out := map[string]inventory.AILatencyExpectation{}
	for _, latency := range latencies {
		out[latency.UsageClassID] = latency
	}
	return out
}

func fallbackByUsageClass(fallbacks []inventory.AIFallbackBehavior) map[string]inventory.AIFallbackBehavior {
	out := map[string]inventory.AIFallbackBehavior{}
	for _, fallback := range fallbacks {
		out[fallback.UsageClassID] = fallback
	}
	return out
}

func modelList(values []string) string {
	if len(values) == 0 {
		return "none captured"
	}
	return strings.Join(sortedStrings(values), ", ")
}

func recommendedVLLMReplicas(peakTokensPerMinute int) int {
	if peakTokensPerMinute <= 0 {
		return 1
	}
	replicas := (peakTokensPerMinute + 11999) / 12000
	if replicas < 1 {
		return 1
	}
	return replicas
}

func targetModelPlaceholder(usageClass inventory.AIModelUsageClass) string {
	if len(usageClass.Models) == 0 {
		return inventory.Slug(usageClass.ID) + "-candidate"
	}
	return inventory.Slug(usageClass.ID) + "-replacement-for-" + inventory.Slug(usageClass.Models[0])
}

func writeFilteredFindings(b *bytes.Buffer, ctx *Context, prefixes ...string) {
	var findings []assessment.Finding
	for _, finding := range ctx.Assessment.Findings {
		for _, prefix := range prefixes {
			if strings.HasPrefix(finding.ID, prefix) {
				findings = append(findings, finding)
				break
			}
		}
	}
	writeRiskRegister(b, ctx, findings)
}

func writeEdgeCandidateConfigs(ctx *Context) error {
	if err := writeEdgeVCLCandidates(ctx); err != nil {
		return err
	}
	if err := writeEdgeHAProxyCandidates(ctx); err != nil {
		return err
	}
	return writeEdgeCorazaCandidates(ctx)
}

func writeEdgeVCLCandidates(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "edge", "varnish")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "vcl 4.1;")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "# Candidate VCL generated by OpenExit. Review before use.")
	writeVCLBackends(&b, ctx.Inventory.Assets.Origins)
	fmt.Fprintln(&b, "sub vcl_recv {")
	for _, rule := range ctx.Inventory.Assets.CacheRules {
		pattern := vclRegex(rule.Pattern)
		if strings.EqualFold(rule.Action, "bypass") {
			fmt.Fprintf(&b, "    if (req.url ~ %q) { return (pass); }\n", pattern)
			continue
		}
		if strings.EqualFold(rule.Action, "cache") {
			fmt.Fprintf(&b, "    if (req.url ~ %q) { unset req.http.Cookie; }\n", pattern)
		}
	}
	for _, rule := range ctx.Inventory.Assets.PageRules {
		if rule.Enabled && containsString(rule.Actions, "cache_everything") {
			fmt.Fprintf(&b, "    if (req.http.host + req.url ~ %q) { unset req.http.Cookie; }\n", vclRegex(rule.Target))
		}
	}
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "sub vcl_backend_response {")
	for _, rule := range ctx.Inventory.Assets.CacheRules {
		if strings.EqualFold(rule.Action, "cache") && rule.EdgeTTL > 0 {
			fmt.Fprintf(&b, "    if (bereq.url ~ %q) { set beresp.ttl = %ds; }\n", vclRegex(rule.Pattern), rule.EdgeTTL)
		}
	}
	fmt.Fprintln(&b, "}")
	if err := os.WriteFile(filepath.Join(dir, "default.candidate.vcl"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Varnish VCL Candidates\n\nCandidate VCL translated from edge cache and page-rule metadata. Review cache keys, cookies, and origin behavior before use.\n"), 0o644)
}

func writeVCLBackends(b *strings.Builder, origins []inventory.OriginConfig) {
	if len(origins) == 0 {
		fmt.Fprintln(b, "backend default {")
		fmt.Fprintln(b, "    .host = \"127.0.0.1\";")
		fmt.Fprintln(b, "    .port = \"8080\";")
		fmt.Fprintln(b, "}")
		fmt.Fprintln(b)
		return
	}
	for i, origin := range origins {
		name := "origin_" + configIdentifier(origin.ID)
		if i == 0 {
			name = "default"
		}
		fmt.Fprintf(b, "backend %s {\n", name)
		fmt.Fprintf(b, "    .host = %q;\n", origin.Hostname)
		fmt.Fprintf(b, "    .port = %q;\n", fmt.Sprintf("%d", origin.Port))
		fmt.Fprintln(b, "}")
		fmt.Fprintln(b)
	}
}

func writeEdgeHAProxyCandidates(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "edge", "haproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Candidate HAProxy config generated by OpenExit. Review before use.")
	fmt.Fprintln(&b, "global")
	fmt.Fprintln(&b, "    log stdout format raw local0")
	fmt.Fprintln(&b, "defaults")
	fmt.Fprintln(&b, "    mode http")
	fmt.Fprintln(&b, "    timeout connect 5s")
	fmt.Fprintln(&b, "    timeout client 30s")
	fmt.Fprintln(&b, "    timeout server 30s")
	fmt.Fprintln(&b, "frontend edge_https")
	fmt.Fprintln(&b, "    bind *:443 ssl crt /etc/haproxy/certs/site.pem")
	for _, redirect := range ctx.Inventory.Assets.Redirects {
		fmt.Fprintf(&b, "    # %s evidence=%s\n", sanitizeConfigValue(redirect.Name), redirect.EvidenceRef)
		fmt.Fprintf(&b, "    http-request redirect code %d location %s if { hdr(host) -m str %s }\n", redirect.StatusCode, sanitizeConfigValue(redirect.Target), redirectHost(redirect.Source))
	}
	for _, origin := range ctx.Inventory.Assets.Origins {
		name := configIdentifier(origin.ID)
		fmt.Fprintf(&b, "    acl host_%s hdr(host) -i %s\n", name, sanitizeConfigValue(origin.HostHeader))
		fmt.Fprintf(&b, "    use_backend be_%s if host_%s\n", name, name)
	}
	if len(ctx.Inventory.Assets.Origins) > 0 {
		fmt.Fprintf(&b, "    default_backend be_%s\n", configIdentifier(ctx.Inventory.Assets.Origins[0].ID))
	}
	for _, origin := range ctx.Inventory.Assets.Origins {
		name := configIdentifier(origin.ID)
		verify := "required"
		if !origin.TLSVerify {
			verify = "none"
		}
		ssl := ""
		if strings.EqualFold(origin.Scheme, "https") {
			ssl = " ssl verify " + verify + " sni str(" + sanitizeConfigValue(origin.Hostname) + ")"
		}
		check := ""
		if origin.HealthCheck {
			check = " check"
		}
		fmt.Fprintf(&b, "backend be_%s\n", name)
		fmt.Fprintf(&b, "    server %s %s:%d%s%s\n", name, sanitizeConfigValue(origin.Hostname), origin.Port, ssl, check)
	}
	if err := os.WriteFile(filepath.Join(dir, "haproxy.candidate.cfg"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte("# HAProxy Candidates\n\nCandidate HAProxy routing, redirect, TLS, and origin definitions derived from edge metadata. Review certificates, ACLs, redirects, and health checks before use.\n"), 0o644)
}

func writeEdgeCorazaCandidates(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "edge", "coraza")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "# Candidate Coraza rules generated by OpenExit. Review before use.")
	fmt.Fprintln(&b, "SecRuleEngine On")
	for i, rule := range ctx.Inventory.Assets.WAFRules {
		action := "deny,status:403"
		if isObserveOnlyEdgeAction(rule.Action) {
			action = "pass,log"
		}
		if !rule.Enabled {
			action = "pass,nolog"
		}
		fmt.Fprintf(&b, "SecRule REQUEST_URI \"@contains %s\" \"id:%d,phase:2,%s,msg:'%s'\"\n", corazaMatchValue(rule.Expression), 100000+i, action, sanitizeConfigValue(rule.Name))
	}
	for i, rule := range ctx.Inventory.Assets.BotRules {
		action := "deny,status:403"
		if isObserveOnlyEdgeAction(rule.Action) || strings.Contains(strings.ToLower(rule.Action), "challenge") {
			action = "pass,log"
		}
		if !rule.Enabled {
			action = "pass,nolog"
		}
		fmt.Fprintf(&b, "SecRule REQUEST_HEADERS:User-Agent \"@rx .+\" \"id:%d,phase:1,%s,msg:'%s'\"\n", 110000+i, action, sanitizeConfigValue(rule.Name))
	}
	if err := os.WriteFile(filepath.Join(dir, "coraza-rules.candidate.conf"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Coraza Rule Candidates\n\nCandidate Coraza rules are conservative placeholders derived from WAF and bot metadata. Map provider-managed rules to OWASP CRS or custom SecRule definitions before use.\n"), 0o644)
}

func vclRegex(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.TrimPrefix(pattern, "https://")
	pattern = strings.TrimPrefix(pattern, "http://")
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	return pattern
}

func redirectHost(source string) string {
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	host, _, _ := strings.Cut(source, "/")
	return sanitizeConfigValue(strings.TrimSuffix(host, "*"))
}

func corazaMatchValue(expression string) string {
	if _, after, ok := strings.Cut(expression, "contains"); ok {
		return strings.Trim(strings.TrimSpace(after), "\"'")
	}
	return "/"
}

func sanitizeConfigValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, "\"", "")
	return strings.TrimSpace(value)
}

func configIdentifier(value string) string {
	return strings.ReplaceAll(inventory.Slug(value), "-", "_")
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func isObserveOnlyEdgeAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	return action == "log" || action == "count" || action == "simulate"
}

func writeGeneratedBy(b *bytes.Buffer) {
	fmt.Fprintln(b, "## Generated by OpenExit")
	fmt.Fprintln(b, "This artifact was generated deterministically from local OpenExit inventory and assessment manifests. It is not production-ready until reviewed.")
}

func firstN(findings []assessment.Finding, n int) []assessment.Finding {
	if len(findings) <= n {
		return findings
	}
	out := make([]assessment.Finding, n)
	copy(out, findings[:n])
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func titleCaseASCII(value string) string {
	parts := strings.Fields(value)
	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}
