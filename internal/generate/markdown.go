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
		case "assessment", "risk-register", "manual-review", "cost-drivers", "target-architecture", "acceptance-criteria", "rollback-plan", "runbook", "restore-drill-checklist", "alert-shadowing-plan", "forgejo-migration-assessment", "ci-compatibility-report", "branch-protection-mapping", "runner-migration-plan", "repository-ownership-report", "identity-migration-risk-register", "break-glass-checklist", "identity-cutover-plan", "identity-rollback-plan":
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		case "realm-client-candidate":
			if err := writeIdentityRealmClientCandidate(ctx); err != nil {
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
	artifacts := markdownArtifacts
	if ctx.Inventory.Source.Type == "github-enterprise" {
		artifacts = []string{"assessment", "risk-register", "manual-review"}
	}
	if ctx.Inventory.Source.Type == "identity" {
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
		return nil
	}
	if ctx.Inventory.Source.Type == "identity" {
		for _, artifact := range identityMarkdownArtifacts {
			if err := writeMarkdown(ctx, artifact); err != nil {
				return err
			}
		}
		return writeIdentityRealmClientCandidate(ctx)
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
	return ArgoCD(ctx)
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
	title := strings.Title(strings.ReplaceAll(artifact, "-", " "))
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
	}
	writeGeneratedBy(&b)
	path := filepath.Join(ctx.ProjectDir, "assessment", artifact+".md")
	return os.WriteFile(path, b.Bytes(), 0o644)
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
