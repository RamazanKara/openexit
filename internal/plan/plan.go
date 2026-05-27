package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "MigrationPlan"
)

type Plan struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Source     Endpoint `json:"source" yaml:"source"`
	Target     Endpoint `json:"target" yaml:"target"`
	Summary    Summary  `json:"summary" yaml:"summary"`
	Phases     []Phase  `json:"phases" yaml:"phases"`
	Warnings   []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type Metadata struct {
	Project     string    `json:"project" yaml:"project"`
	GeneratedAt time.Time `json:"generatedAt" yaml:"generatedAt"`
}

type Endpoint struct {
	Type string `json:"type" yaml:"type"`
}

type Summary struct {
	Status            string `json:"status" yaml:"status"`
	RequiredArtifacts int    `json:"requiredArtifacts" yaml:"requiredArtifacts"`
	MissingArtifacts  int    `json:"missingArtifacts" yaml:"missingArtifacts"`
}

type Phase struct {
	Name              string   `json:"name" yaml:"name"`
	Objective         string   `json:"objective" yaml:"objective"`
	Status            string   `json:"status" yaml:"status"`
	RequiredArtifacts []string `json:"requiredArtifacts" yaml:"requiredArtifacts"`
	MissingArtifacts  []string `json:"missingArtifacts,omitempty" yaml:"missingArtifacts,omitempty"`
	ExitCriteria      []string `json:"exitCriteria" yaml:"exitCriteria"`
}

type phaseTemplate struct {
	Name              string
	Objective         string
	RequiredArtifacts []string
	ExitCriteria      []string
}

func Build(projectDir string, inv *inventory.Inventory, a *assessment.Assessment, generatedAt time.Time) (*Plan, error) {
	if inv == nil {
		return nil, fmt.Errorf("inventory is required")
	}
	if a == nil {
		return nil, fmt.Errorf("assessment is required")
	}
	templates := phaseTemplates(inv.Source.Type)
	plan := &Plan{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Project:     a.Metadata.Project,
			GeneratedAt: generatedAt.UTC(),
		},
		Source: Endpoint{Type: inv.Source.Type},
		Target: Endpoint{Type: a.Target.Type},
	}
	uniqueRequired := map[string]struct{}{}
	uniqueMissing := map[string]struct{}{}
	for _, tmpl := range templates {
		phase := Phase{
			Name:              tmpl.Name,
			Objective:         tmpl.Objective,
			RequiredArtifacts: sortedUnique(tmpl.RequiredArtifacts),
			ExitCriteria:      append([]string{}, tmpl.ExitCriteria...),
			Status:            "ready",
		}
		for _, artifact := range phase.RequiredArtifacts {
			uniqueRequired[artifact] = struct{}{}
			if !fileExists(projectDir, artifact) {
				phase.MissingArtifacts = append(phase.MissingArtifacts, artifact)
				uniqueMissing[artifact] = struct{}{}
			}
		}
		if len(phase.MissingArtifacts) > 0 {
			phase.Status = "incomplete"
		}
		plan.Phases = append(plan.Phases, phase)
	}
	plan.Summary = Summary{
		Status:            "ready",
		RequiredArtifacts: len(uniqueRequired),
		MissingArtifacts:  len(uniqueMissing),
	}
	if len(uniqueMissing) > 0 {
		plan.Summary.Status = "incomplete"
	}
	if err := Validate(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func Write(projectDir string, plan *Plan) error {
	if err := Validate(plan); err != nil {
		return err
	}
	dir := filepath.Join(projectDir, "assessment")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.migration-plan.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.migration-plan.json"), append(jsonData, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "migration-plan.md"), []byte(RenderMarkdown(plan)), 0o644)
}

func Validate(plan *Plan) error {
	var problems []string
	if plan.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if plan.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if strings.TrimSpace(plan.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}
	if strings.TrimSpace(plan.Source.Type) == "" {
		problems = append(problems, "source.type is required")
	}
	if strings.TrimSpace(plan.Target.Type) == "" {
		problems = append(problems, "target.type is required")
	}
	if plan.Summary.Status != "ready" && plan.Summary.Status != "incomplete" {
		problems = append(problems, "summary.status must be ready or incomplete")
	}
	if len(plan.Phases) == 0 {
		problems = append(problems, "phases are required")
	}
	for _, phase := range plan.Phases {
		if strings.TrimSpace(phase.Name) == "" {
			problems = append(problems, "each phase requires name")
		}
		if phase.Status != "ready" && phase.Status != "incomplete" {
			problems = append(problems, fmt.Sprintf("phase %q status must be ready or incomplete", phase.Name))
		}
		if len(phase.ExitCriteria) == 0 {
			problems = append(problems, fmt.Sprintf("phase %q requires exitCriteria", phase.Name))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func RenderMarkdown(plan *Plan) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Migration Plan")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Project: %s\n", plan.Metadata.Project)
	fmt.Fprintf(&b, "- Source: %s\n", plan.Source.Type)
	fmt.Fprintf(&b, "- Target: %s\n", plan.Target.Type)
	fmt.Fprintf(&b, "- Generated: %s\n", plan.Metadata.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&b, "- Status: %s\n", plan.Summary.Status)
	fmt.Fprintf(&b, "- Required artifacts: %d\n", plan.Summary.RequiredArtifacts)
	fmt.Fprintf(&b, "- Missing artifacts: %d\n", plan.Summary.MissingArtifacts)
	fmt.Fprintln(&b)
	for _, phase := range plan.Phases {
		fmt.Fprintf(&b, "## %s\n", titleCaseASCII(phase.Name))
		fmt.Fprintf(&b, "- Status: %s\n", phase.Status)
		fmt.Fprintf(&b, "- Objective: %s\n", phase.Objective)
		fmt.Fprintln(&b, "- Required artifacts:")
		if len(phase.RequiredArtifacts) == 0 {
			fmt.Fprintln(&b, "  - None")
		} else {
			for _, artifact := range phase.RequiredArtifacts {
				fmt.Fprintf(&b, "  - %s\n", artifact)
			}
		}
		if len(phase.MissingArtifacts) > 0 {
			fmt.Fprintln(&b, "- Missing artifacts:")
			for _, artifact := range phase.MissingArtifacts {
				fmt.Fprintf(&b, "  - %s\n", artifact)
			}
		}
		fmt.Fprintln(&b, "- Exit criteria:")
		for _, criterion := range phase.ExitCriteria {
			fmt.Fprintf(&b, "  - %s\n", criterion)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "## Generated By OpenExit")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This migration plan was generated deterministically from local OpenExit inventory, assessment, and generated artifact paths. It does not perform production writes.")
	return b.String()
}

func phaseTemplates(source string) []phaseTemplate {
	commonAssessment := []string{
		"assessment/assessment.md",
		"assessment/risk-register.md",
		"assessment/manual-review.md",
	}
	switch source {
	case "github-enterprise":
		return []phaseTemplate{
			{
				Name:              "assessment",
				Objective:         "Confirm repository, ownership, CI, branch protection, runner, secret metadata, and manual-review scope.",
				RequiredArtifacts: appendArtifacts(commonAssessment, "assessment/forgejo-migration-assessment.md", "assessment/repository-ownership-report.md"),
				ExitCriteria:      []string{"Repository owners and critical workflow maintainers are identified.", "High-severity findings have an accountable reviewer."},
			},
			{
				Name:              "pilot",
				Objective:         "Prepare Forgejo migration candidates for a non-production pilot.",
				RequiredArtifacts: []string{"generated-config/forgejo/migration-candidate.yaml", "assessment/ci-compatibility-report.md", "assessment/branch-protection-mapping.md", "assessment/runner-migration-plan.md"},
				ExitCriteria:      []string{"Candidate repository settings are reviewed in a non-production Forgejo environment.", "Runner and workflow gaps have owners."},
			},
			{
				Name:              "shadow",
				Objective:         "Run source and target workflows in parallel where feasible before cutover.",
				RequiredArtifacts: []string{"assessment/ci-compatibility-report.md", "assessment/branch-protection-mapping.md"},
				ExitCriteria:      []string{"Required checks, review rules, and runner labels are validated against representative repositories."},
			},
			{
				Name:              "cutover",
				Objective:         "Move repositories only after migration candidates and manual reviews are accepted.",
				RequiredArtifacts: []string{"assessment/forgejo-migration-assessment.md", "assessment/manual-review.md"},
				ExitCriteria:      []string{"Write access, deploy keys, package publishing, and webhook behavior have owner sign-off."},
			},
		}
	case "identity":
		return []phaseTemplate{
			{
				Name:              "assessment",
				Objective:         "Confirm application, group, policy, MFA, and break-glass risk before target realm design.",
				RequiredArtifacts: appendArtifacts(commonAssessment, "assessment/identity-migration-risk-register.md", "assessment/break-glass-checklist.md"),
				ExitCriteria:      []string{"Application owners have reviewed high-risk redirect URI, grant, and MFA findings.", "Break-glass custody is documented."},
			},
			{
				Name:              "pilot",
				Objective:         "Prepare target realm and client candidates for non-production identity tests.",
				RequiredArtifacts: []string{"generated-config/identity/realm-client-candidate.yaml"},
				ExitCriteria:      []string{"OIDC and SAML clients are imported into a non-production target realm.", "Owners validate redirect and federation behavior."},
			},
			{
				Name:              "shadow",
				Objective:         "Run parallel login and authorization tests before issuer or federation changes.",
				RequiredArtifacts: []string{"assessment/identity-cutover-plan.md", "assessment/break-glass-checklist.md"},
				ExitCriteria:      []string{"Critical applications pass login, logout, group claim, and MFA tests.", "Emergency access is tested in the target."},
			},
			{
				Name:              "cutover",
				Objective:         "Change identity routing only after the cutover and rollback paths are approved.",
				RequiredArtifacts: []string{"assessment/identity-cutover-plan.md", "assessment/identity-rollback-plan.md"},
				ExitCriteria:      []string{"Rollback owners and source tenant preservation windows are approved."},
			},
		}
	case "edge":
		return []phaseTemplate{
			{
				Name:              "assessment",
				Objective:         "Confirm DNS, cache, WAF, TLS, origin, redirect, bot, and page-rule parity risk.",
				RequiredArtifacts: appendArtifacts(commonAssessment, "assessment/cache-parity-report.md", "assessment/waf-enforcement-risk-report.md"),
				ExitCriteria:      []string{"Cache and WAF behavior differences are accepted or assigned.", "Origin health and TLS gaps have owners."},
			},
			{
				Name:              "pilot",
				Objective:         "Prepare Varnish, HAProxy, and Coraza candidates for isolated edge testing.",
				RequiredArtifacts: []string{"generated-config/edge/varnish/default.candidate.vcl", "generated-config/edge/haproxy/haproxy.candidate.cfg", "generated-config/edge/coraza/coraza-rules.candidate.conf"},
				ExitCriteria:      []string{"Candidate configs parse and are tested against representative requests.", "Origin routing and certificate assumptions are reviewed."},
			},
			{
				Name:              "shadow",
				Objective:         "Compare source and target edge behavior before DNS or traffic migration.",
				RequiredArtifacts: []string{"assessment/cache-parity-report.md", "assessment/waf-enforcement-risk-report.md"},
				ExitCriteria:      []string{"Cache hit behavior, redirects, and WAF decisions are compared with source behavior."},
			},
			{
				Name:              "cutover",
				Objective:         "Move edge traffic only after manual-review findings and origin safety checks are complete.",
				RequiredArtifacts: []string{"assessment/manual-review.md", "generated-config/edge/haproxy/haproxy.candidate.cfg"},
				ExitCriteria:      []string{"Rollback DNS and origin routing steps are documented outside the generated candidates."},
			},
		}
	case "ai-provider":
		return []phaseTemplate{
			{
				Name:              "assessment",
				Objective:         "Confirm model, token-volume, data-sensitivity, tool, latency, and fallback risk.",
				RequiredArtifacts: appendArtifacts(commonAssessment, "assessment/self-hosted-llm-readiness-report.md", "assessment/data-sensitivity-report.md"),
				ExitCriteria:      []string{"Sensitive prompt categories and tool behavior have policy owners.", "High-volume usage classes have sizing owners."},
			},
			{
				Name:              "pilot",
				Objective:         "Prepare LiteLLM routing and vLLM sizing candidates for non-production evaluation.",
				RequiredArtifacts: []string{"generated-config/ai/litellm/config.candidate.yaml", "assessment/vllm-sizing-assumptions.md"},
				ExitCriteria:      []string{"Candidate model routes are tested without production traffic.", "Sizing assumptions are benchmarked on representative hardware."},
			},
			{
				Name:              "shadow",
				Objective:         "Compare source provider and self-hosted model behavior before traffic migration.",
				RequiredArtifacts: []string{"assessment/evaluation-plan.md", "assessment/data-sensitivity-report.md"},
				ExitCriteria:      []string{"Evaluation sets cover quality, latency, refusal behavior, tool use, and fallback behavior."},
			},
			{
				Name:              "cutover",
				Objective:         "Route production model traffic only after evaluation, capacity, fallback, and data-control sign-off.",
				RequiredArtifacts: []string{"assessment/self-hosted-llm-readiness-report.md", "assessment/manual-review.md"},
				ExitCriteria:      []string{"Fallback and manual-queue behavior are accepted by service owners."},
			},
		}
	default:
		return []phaseTemplate{
			{
				Name:              "assessment",
				Objective:         "Confirm inventory, findings, cost drivers, and manual-review scope.",
				RequiredArtifacts: appendArtifacts(commonAssessment, "assessment/cost-drivers.md"),
				ExitCriteria:      []string{"All high-severity findings have owners.", "Evidence references resolve to redacted local evidence."},
			},
			{
				Name:              "pilot",
				Objective:         "Prepare Grafana, Prometheus, OpenTelemetry, and ArgoCD candidates for a non-production pilot.",
				RequiredArtifacts: []string{"assessment/target-architecture.md", "generated-config/grafana/README.md", "generated-config/prometheus/rules/datadog-monitor-candidates.yaml", "generated-config/opentelemetry/collector.candidate.yaml", "generated-config/argocd/grafana-stack-application.candidate.yaml"},
				ExitCriteria:      []string{"Candidate dashboards and alert rules are reviewed in a non-production target.", "Collector and GitOps sketches are reviewed by platform owners."},
			},
			{
				Name:              "shadow",
				Objective:         "Run target alert and dashboard candidates alongside Datadog before cutover.",
				RequiredArtifacts: []string{"assessment/acceptance-criteria.md", "assessment/alert-shadowing-plan.md"},
				ExitCriteria:      []string{"Alert shadowing covers representative incidents and quiet periods.", "Dashboard parity differences are accepted by owners."},
			},
			{
				Name:              "cutover",
				Objective:         "Change operational routing only after rollback, runbook, and restore-drill checks are accepted.",
				RequiredArtifacts: []string{"assessment/rollback-plan.md", "assessment/runbook.md", "assessment/restore-drill-checklist.md"},
				ExitCriteria:      []string{"Rollback path is tested.", "Source Datadog monitors remain available until target behavior is accepted."},
			},
		}
	}
}

func appendArtifacts(base []string, values ...string) []string {
	out := append([]string{}, base...)
	out = append(out, values...)
	return out
}

func sortedUnique(values []string) []string {
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

func fileExists(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && !info.IsDir()
}

func titleCaseASCII(value string) string {
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
