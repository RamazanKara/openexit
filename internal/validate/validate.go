package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/evidence"
	"github.com/RamazanKara/openexit/internal/inventory"
	openmapping "github.com/RamazanKara/openexit/internal/mapping"
	migrationplan "github.com/RamazanKara/openexit/internal/plan"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "ValidationReport"
)

var supportedTargetBySource = map[string]string{
	"datadog":           "grafana-lgtm",
	"github-enterprise": "forgejo",
	"identity":          "keycloak-zitadel",
	"edge":              "varnish-haproxy-coraza",
	"ai-provider":       "vllm-litellm",
}

type Report struct {
	APIVersion  string    `json:"apiVersion" yaml:"apiVersion"`
	Kind        string    `json:"kind" yaml:"kind"`
	Status      string    `json:"status" yaml:"status"`
	GeneratedAt time.Time `json:"generatedAt" yaml:"generatedAt"`
	Checks      []Check   `json:"checks" yaml:"checks"`
}

type Check struct {
	Name     string `json:"name" yaml:"name"`
	Status   string `json:"status" yaml:"status"`
	Message  string `json:"message,omitempty" yaml:"message,omitempty"`
	Critical bool   `json:"critical" yaml:"critical"`
}

func Run(projectDir string, strict bool) (*Report, error) {
	report := &Report{
		APIVersion:  APIVersion,
		Kind:        Kind,
		Status:      "passed",
		GeneratedAt: time.Now().UTC(),
	}
	schemaValidator, schemaErr := newSchemaValidator()
	add := func(name, status, message string, critical bool) {
		report.Checks = append(report.Checks, Check{Name: name, Status: status, Message: message, Critical: critical})
		if status == "failed" && (critical || strict) {
			report.Status = "failed"
		}
		if status == "warning" && strict {
			report.Status = "failed"
		}
	}
	if err := checkProject(projectDir); err != nil {
		add("project-config", "failed", err.Error(), true)
	} else {
		add("project-config", "passed", "", true)
	}
	cfg, cfgErr := loadProjectManifest(projectDir)
	inv, err := loadInventory(projectDir)
	if err != nil {
		add("inventory-schema", "failed", err.Error(), true)
	} else {
		add("inventory-schema", "passed", "", true)
	}
	a, err := loadAssessment(projectDir)
	if err != nil {
		add("assessment-schema", "failed", err.Error(), true)
	} else {
		add("assessment-schema", "passed", "", true)
	}
	mappingResult, err := loadMapping(projectDir)
	if err != nil {
		add("mapping-schema", "failed", err.Error(), true)
	} else {
		add("mapping-schema", "passed", "", true)
	}
	addYAMLChecks(projectDir, add)
	addJSONChecks(projectDir, add)
	if schemaErr != nil {
		add("jsonschema-compiler", "failed", schemaErr.Error(), true)
	} else {
		addSchemaChecks(projectDir, schemaValidator, add)
	}
	addExternalChecks(projectDir, add)
	if inv != nil && a != nil {
		if cfgErr == nil {
			addProjectConsistencyChecks(cfg, inv, a, add)
		}
		if mappingResult != nil {
			addMappingConsistencyChecks(inv, a, mappingResult, add)
			addGrafanaDashboardCandidateChecks(projectDir, inv, mappingResult, add)
		}
		addEvidenceChecks(projectDir, inv, a, add)
	}
	if p, err := loadMigrationPlan(projectDir); err != nil {
		if os.IsNotExist(err) {
			add("migration-plan", "warning", "migration plan not generated; run openexit generate --all or openexit generate --artifact migration-plan", false)
		} else {
			add("migration-plan", "failed", err.Error(), true)
		}
	} else {
		add("migration-plan", "passed", "", true)
		addMigrationPlanArtifactChecks(projectDir, p, add)
	}
	addSecretScan(projectDir, add)
	if schemaErr == nil {
		addValidationReportSchemaCheck(schemaValidator, report, add)
	}
	sort.SliceStable(report.Checks, func(i, j int) bool { return report.Checks[i].Name < report.Checks[j].Name })
	if err := writeReport(projectDir, report); err != nil {
		return report, err
	}
	if report.Status == "failed" {
		return report, fmt.Errorf("validation failed")
	}
	return report, nil
}

type projectManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Source struct {
		Type string `yaml:"type"`
	} `yaml:"source"`
	Target struct {
		Type string `yaml:"type"`
	} `yaml:"target"`
	Policy struct {
		AllowNetworkWrites    bool `yaml:"allowNetworkWrites"`
		AllowProductionWrites bool `yaml:"allowProductionWrites"`
	} `yaml:"policy"`
}

func checkProject(projectDir string) error {
	cfg, err := loadProjectManifest(projectDir)
	if err != nil {
		return err
	}
	var problems []string
	if cfg.APIVersion != APIVersion {
		problems = append(problems, "apiVersion must be openexit.dev/v1alpha1")
	}
	if cfg.Kind != "Project" {
		problems = append(problems, "kind must be Project")
	}
	if cfg.Metadata.Name == "" {
		problems = append(problems, "metadata.name is required")
	}
	sourceType := strings.TrimSpace(cfg.Source.Type)
	targetType := strings.TrimSpace(cfg.Target.Type)
	if sourceType == "" {
		problems = append(problems, "source.type is required")
	}
	if targetType == "" {
		problems = append(problems, "target.type is required")
	}
	if sourceType != "" && targetType != "" {
		expectedTarget, ok := supportedTargetBySource[sourceType]
		if !ok {
			problems = append(problems, fmt.Sprintf("source.type %q is not supported", sourceType))
		} else if targetType != expectedTarget {
			problems = append(problems, fmt.Sprintf("target.type must be %q for source.type %q", expectedTarget, sourceType))
		}
	}
	if cfg.Policy.AllowNetworkWrites || cfg.Policy.AllowProductionWrites {
		problems = append(problems, "project policy must not allow network or production writes")
	}
	for _, dir := range []string{"inventory", "assessment", "mapping", "generated-config", "evidence", "validation"} {
		info, err := os.Stat(filepath.Join(projectDir, dir))
		if err != nil || !info.IsDir() {
			problems = append(problems, "missing directory "+dir)
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func loadProjectManifest(projectDir string) (*projectManifest, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "openexit.yaml"))
	if err != nil {
		return nil, err
	}
	var cfg projectManifest
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func loadInventory(projectDir string) (*inventory.Inventory, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "inventory", "openexit.inventory.yaml"))
	if err != nil {
		return nil, err
	}
	var inv inventory.Inventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	if err := inventory.Validate(&inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

func loadAssessment(projectDir string) (*assessment.Assessment, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		return nil, err
	}
	var a assessment.Assessment
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	if err := assessment.Validate(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

func loadMapping(projectDir string) (*openmapping.MappingResult, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "mapping", "openexit.mapping.yaml"))
	if err != nil {
		return nil, err
	}
	var result openmapping.MappingResult
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if err := openmapping.Validate(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadMigrationPlan(projectDir string) (*migrationplan.Plan, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.migration-plan.yaml"))
	if err != nil {
		return nil, err
	}
	var p migrationplan.Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if err := migrationplan.Validate(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func addYAMLChecks(projectDir string, add func(string, string, string, bool)) {
	files := append(globRecursive(projectDir, ".yaml"), globRecursive(projectDir, ".yml")...)
	sort.Strings(files)
	for _, path := range files {
		data, err := os.ReadFile(path)
		name := "yaml-parse-" + relName(projectDir, path)
		if err != nil {
			add(name, "failed", err.Error(), true)
			continue
		}
		var value any
		if err := yaml.Unmarshal(data, &value); err != nil {
			add(name, "failed", err.Error(), true)
			continue
		}
		add(name, "passed", "", true)
	}
}

func addJSONChecks(projectDir string, add func(string, string, string, bool)) {
	for _, path := range globRecursive(projectDir, ".json") {
		data, err := os.ReadFile(path)
		name := "json-parse-" + relName(projectDir, path)
		if err != nil {
			add(name, "failed", err.Error(), true)
			continue
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			add(name, "failed", err.Error(), true)
			continue
		}
		add(name, "passed", "", true)
	}
}

func addExternalChecks(projectDir string, add func(string, string, string, bool)) {
	rules := filepath.Join(projectDir, "generated-config", "prometheus", "rules", "datadog-monitor-candidates.yaml")
	if _, err := os.Stat(rules); err == nil {
		if path, err := exec.LookPath("promtool"); err == nil {
			cmd := exec.Command(path, "check", "rules", rules)
			if out, err := cmd.CombinedOutput(); err != nil {
				add("prometheus-rules-promtool", "failed", string(out), true)
			} else {
				add("prometheus-rules-promtool", "passed", "", true)
			}
		} else {
			add("prometheus-rules-promtool", "warning", "promtool not found; YAML parse still ran", false)
		}
	}
	argocd := filepath.Join(projectDir, "generated-config", "argocd", "grafana-stack-application.candidate.yaml")
	if _, err := os.Stat(argocd); err == nil {
		if path, err := exec.LookPath("kubeconform"); err == nil {
			cmd := exec.Command(path, "-ignore-missing-schemas", argocd)
			if out, err := cmd.CombinedOutput(); err != nil {
				add("argocd-kubeconform", "failed", string(out), false)
			} else {
				add("argocd-kubeconform", "passed", "", false)
			}
		} else {
			add("argocd-kubeconform", "warning", "kubeconform not found; YAML parse still ran", false)
		}
	}
}

func addProjectConsistencyChecks(cfg *projectManifest, inv *inventory.Inventory, a *assessment.Assessment, add func(string, string, string, bool)) {
	var problems []string
	if cfg.Source.Type != inv.Source.Type {
		problems = append(problems, fmt.Sprintf("project source %q does not match inventory source %q", cfg.Source.Type, inv.Source.Type))
	}
	if inv.Source.Type != a.Source.Type {
		problems = append(problems, fmt.Sprintf("inventory source %q does not match assessment source %q", inv.Source.Type, a.Source.Type))
	}
	if cfg.Target.Type != a.Target.Type {
		problems = append(problems, fmt.Sprintf("project target %q does not match assessment target %q", cfg.Target.Type, a.Target.Type))
	}
	if len(problems) > 0 {
		add("project-manifest-consistency", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("project-manifest-consistency", "passed", "", true)
}

func addMappingConsistencyChecks(inv *inventory.Inventory, a *assessment.Assessment, result *openmapping.MappingResult, add func(string, string, string, bool)) {
	var problems []string
	if result.Source.Type != inv.Source.Type {
		problems = append(problems, fmt.Sprintf("mapping source %q does not match inventory source %q", result.Source.Type, inv.Source.Type))
	}
	if result.Target.Type != a.Target.Type {
		problems = append(problems, fmt.Sprintf("mapping target %q does not match assessment target %q", result.Target.Type, a.Target.Type))
	}
	if result.TargetType != a.Target.Type {
		problems = append(problems, fmt.Sprintf("mapping targetType %q does not match assessment target %q", result.TargetType, a.Target.Type))
	}
	if len(problems) > 0 {
		add("mapping-consistency", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("mapping-consistency", "passed", "", true)
}

func addMigrationPlanArtifactChecks(projectDir string, p *migrationplan.Plan, add func(string, string, string, bool)) {
	seen := map[string]struct{}{}
	var missing []string
	for _, phase := range p.Phases {
		for _, rel := range phase.RequiredArtifacts {
			rel = strings.TrimSpace(rel)
			if rel == "" {
				continue
			}
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			info, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel)))
			if err != nil || info.IsDir() {
				missing = append(missing, rel)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		add("migration-plan-artifacts", "failed", "required artifacts missing: "+strings.Join(missing, ", "), true)
		return
	}
	add("migration-plan-artifacts", "passed", "", true)
}

func addEvidenceChecks(projectDir string, inv *inventory.Inventory, a *assessment.Assessment, add func(string, string, string, bool)) {
	refs := map[string]struct{}{}
	for _, dashboard := range inv.Assets.Dashboards {
		refs[dashboard.EvidenceRef] = struct{}{}
	}
	for _, monitor := range inv.Assets.Monitors {
		refs[monitor.EvidenceRef] = struct{}{}
	}
	for _, slo := range inv.Assets.SLOs {
		refs[slo.EvidenceRef] = struct{}{}
	}
	for _, repo := range inv.Assets.Repositories {
		refs[repo.EvidenceRef] = struct{}{}
	}
	for _, team := range inv.Assets.Teams {
		refs[team.EvidenceRef] = struct{}{}
	}
	for _, protection := range inv.Assets.BranchProtections {
		refs[protection.EvidenceRef] = struct{}{}
	}
	for _, workflow := range inv.Assets.ActionsWorkflows {
		refs[workflow.EvidenceRef] = struct{}{}
	}
	for _, secret := range inv.Assets.Secrets {
		refs[secret.EvidenceRef] = struct{}{}
	}
	for _, runner := range inv.Assets.Runners {
		refs[runner.EvidenceRef] = struct{}{}
	}
	for _, key := range inv.Assets.DeployKeys {
		refs[key.EvidenceRef] = struct{}{}
	}
	for _, app := range inv.Assets.GitHubApps {
		refs[app.EvidenceRef] = struct{}{}
	}
	for _, app := range inv.Assets.IdentityApps {
		refs[app.EvidenceRef] = struct{}{}
	}
	for _, group := range inv.Assets.IdentityGroups {
		refs[group.EvidenceRef] = struct{}{}
	}
	for _, policy := range inv.Assets.IdentityPolicies {
		refs[policy.EvidenceRef] = struct{}{}
	}
	for _, setting := range inv.Assets.MFASettings {
		refs[setting.EvidenceRef] = struct{}{}
	}
	for _, account := range inv.Assets.BreakGlassAccounts {
		refs[account.EvidenceRef] = struct{}{}
	}
	for _, record := range inv.Assets.DNSRecords {
		refs[record.EvidenceRef] = struct{}{}
	}
	for _, rule := range inv.Assets.WAFRules {
		refs[rule.EvidenceRef] = struct{}{}
	}
	for _, rule := range inv.Assets.CacheRules {
		refs[rule.EvidenceRef] = struct{}{}
	}
	for _, redirect := range inv.Assets.Redirects {
		refs[redirect.EvidenceRef] = struct{}{}
	}
	for _, origin := range inv.Assets.Origins {
		refs[origin.EvidenceRef] = struct{}{}
	}
	for _, setting := range inv.Assets.TLSSettings {
		refs[setting.EvidenceRef] = struct{}{}
	}
	for _, rule := range inv.Assets.BotRules {
		refs[rule.EvidenceRef] = struct{}{}
	}
	for _, rule := range inv.Assets.PageRules {
		refs[rule.EvidenceRef] = struct{}{}
	}
	for _, usageClass := range inv.Assets.AIModelUsageClasses {
		refs[usageClass.EvidenceRef] = struct{}{}
	}
	for _, volume := range inv.Assets.AITokenVolumes {
		refs[volume.EvidenceRef] = struct{}{}
	}
	for _, latency := range inv.Assets.AILatencyExpectations {
		refs[latency.EvidenceRef] = struct{}{}
	}
	for _, category := range inv.Assets.AISensitivePromptCategories {
		refs[category.EvidenceRef] = struct{}{}
	}
	for _, tool := range inv.Assets.AIToolUsages {
		refs[tool.EvidenceRef] = struct{}{}
	}
	for _, fallback := range inv.Assets.AIFallbackBehaviors {
		refs[fallback.EvidenceRef] = struct{}{}
	}
	for _, finding := range a.Findings {
		for _, ref := range finding.EvidenceRefs {
			refs[ref] = struct{}{}
		}
	}
	for ref := range refs {
		if ref == "evidence://inventory/summary" {
			continue
		}
		path, err := evidence.PathForRef(projectDir, ref)
		if err != nil {
			add("evidence-ref-"+ref, "failed", err.Error(), true)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			add("evidence-ref-"+ref, "failed", err.Error(), true)
			continue
		}
		add("evidence-ref-"+ref, "passed", "", true)
	}
}

func addSecretScan(projectDir string, add func(string, string, string, bool)) {
	var findings []string
	for _, path := range secretScanFiles(projectDir) {
		if strings.Contains(path, string(filepath.Separator)+"validation"+string(filepath.Separator)) {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil && inventory.ContainsSecret(string(data)) {
			findings = append(findings, relName(projectDir, path))
		}
	}
	if len(findings) > 0 {
		add("secret-scan", "failed", "potential secret-like values found in "+strings.Join(findings, ", "), true)
		return
	}
	add("secret-scan", "passed", "", true)
}

func secretScanFiles(projectDir string) []string {
	suffixes := []string{".json", ".yaml", ".yml", ".md", ".txt", ".cfg", ".conf", ".vcl"}
	seen := map[string]struct{}{}
	var files []string
	for _, suffix := range suffixes {
		for _, path := range globRecursive(projectDir, suffix) {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func writeReport(projectDir string, report *Report) error {
	dir := filepath.Join(projectDir, "validation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(report)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.validation.yaml"), data, 0o644); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Validation Report\n\nStatus: %s\n\n", report.Status)
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "- %s: %s", check.Name, check.Status)
		if check.Message != "" {
			fmt.Fprintf(&b, " - %s", strings.TrimSpace(check.Message))
		}
		fmt.Fprintln(&b)
	}
	return os.WriteFile(filepath.Join(dir, "validation-report.md"), []byte(b.String()), 0o644)
}

func globRecursive(root, suffix string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func relName(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
