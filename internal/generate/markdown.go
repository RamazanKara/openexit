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
		case "assessment", "risk-register", "manual-review", "cost-drivers", "target-architecture", "acceptance-criteria", "rollback-plan", "runbook", "restore-drill-checklist", "alert-shadowing-plan":
			if err := writeMarkdown(ctx, artifact); err != nil {
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
	for _, artifact := range markdownArtifacts {
		if err := writeMarkdown(ctx, artifact); err != nil {
			return err
		}
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
	fmt.Fprintf(b, "- Dashboards: %d\n", ctx.Inventory.Summary.Dashboards)
	fmt.Fprintf(b, "- Monitors: %d\n", ctx.Inventory.Summary.Monitors)
	fmt.Fprintf(b, "- SLOs: %d\n", ctx.Inventory.Summary.SLOs)
	fmt.Fprintf(b, "- Integrations: %d\n", ctx.Inventory.Summary.Integrations)
	fmt.Fprintf(b, "- Unique metrics: %d\n\n", ctx.Inventory.Summary.UniqueMetrics)
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
