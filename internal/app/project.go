package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/mapping"
	"github.com/RamazanKara/openexit/internal/validate"
	"gopkg.in/yaml.v3"
)

var requiredProjectDirs = []string{
	"inventory",
	"assessment",
	"mapping",
	"generated-config",
	"evidence",
	"validation",
}

type ProjectStatus struct {
	ProjectDir     string           `json:"projectDir" yaml:"projectDir"`
	ConfigOK       bool             `json:"configOk" yaml:"configOk"`
	Source         string           `json:"source,omitempty" yaml:"source,omitempty"`
	Target         string           `json:"target,omitempty" yaml:"target,omitempty"`
	Layout         string           `json:"layout" yaml:"layout"`
	Missing        []string         `json:"missing,omitempty" yaml:"missing,omitempty"`
	Inventory      InventoryStatus  `json:"inventory" yaml:"inventory"`
	Assessment     AssessmentStatus `json:"assessment" yaml:"assessment"`
	Mapping        MappingStatus    `json:"mapping" yaml:"mapping"`
	Generated      GeneratedStatus  `json:"generated" yaml:"generated"`
	Validation     ValidationStatus `json:"validation" yaml:"validation"`
	ReadyForExport bool             `json:"readyForExport" yaml:"readyForExport"`
	NextActions    []string         `json:"nextActions" yaml:"nextActions"`
}

type InventoryStatus struct {
	Present  bool           `json:"present" yaml:"present"`
	Assets   map[string]int `json:"assets,omitempty" yaml:"assets,omitempty"`
	Warnings int            `json:"warnings" yaml:"warnings"`
	Error    string         `json:"error,omitempty" yaml:"error,omitempty"`
}

type AssessmentStatus struct {
	Present  bool           `json:"present" yaml:"present"`
	Findings int            `json:"findings" yaml:"findings"`
	Severity map[string]int `json:"severity,omitempty" yaml:"severity,omitempty"`
	Score    int            `json:"score" yaml:"score"`
	Level    string         `json:"level,omitempty" yaml:"level,omitempty"`
	Warnings int            `json:"warnings" yaml:"warnings"`
	Error    string         `json:"error,omitempty" yaml:"error,omitempty"`
}

type MappingStatus struct {
	Present          bool   `json:"present" yaml:"present"`
	DashboardDrafts  int    `json:"dashboardDrafts" yaml:"dashboardDrafts"`
	AlertRuleDrafts  int    `json:"alertRuleDrafts" yaml:"alertRuleDrafts"`
	UnsupportedItems int    `json:"unsupportedItems" yaml:"unsupportedItems"`
	ManualReview     int    `json:"manualReview" yaml:"manualReview"`
	Error            string `json:"error,omitempty" yaml:"error,omitempty"`
}

type GeneratedStatus struct {
	Present      bool     `json:"present" yaml:"present"`
	Files        int      `json:"files" yaml:"files"`
	Candidates   int      `json:"candidates" yaml:"candidates"`
	KeyArtifacts []string `json:"keyArtifacts,omitempty" yaml:"keyArtifacts,omitempty"`
	Error        string   `json:"error,omitempty" yaml:"error,omitempty"`
}

type ValidationStatus struct {
	Present  bool   `json:"present" yaml:"present"`
	Status   string `json:"status,omitempty" yaml:"status,omitempty"`
	Checks   int    `json:"checks" yaml:"checks"`
	Passed   int    `json:"passed" yaml:"passed"`
	Failed   int    `json:"failed" yaml:"failed"`
	Warnings int    `json:"warnings" yaml:"warnings"`
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
}

func InitProject(projectDir string) error {
	return InitProjectWithEndpoints(projectDir, defaultSource, defaultTargetForSource(defaultSource))
}

func InitProjectWithEndpoints(projectDir, source, target string) error {
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	for _, dir := range requiredProjectDirs {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	configPath := filepath.Join(projectDir, projectFileName)
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", configPath, err)
	}
	cfg := DefaultProjectConfig(projectDir)
	if source != "" {
		cfg.Source.Type = source
	}
	if target != "" {
		cfg.Target.Type = target
	} else {
		cfg.Target.Type = defaultTargetForSource(cfg.Source.Type)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid project options: %w", err)
	}
	return WriteProjectConfig(projectDir, cfg)
}

func CheckProject(projectDir string) (*ProjectStatus, error) {
	status := &ProjectStatus{ProjectDir: projectDir, Layout: "missing"}
	cfg, err := LoadProjectConfig(projectDir)
	if err != nil {
		return status, err
	}
	status.ConfigOK = true
	status.Source = cfg.Source.Type
	status.Target = cfg.Target.Type
	for _, dir := range requiredProjectDirs {
		path := filepath.Join(projectDir, dir)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			status.Missing = append(status.Missing, dir)
		}
	}
	if len(status.Missing) == 0 {
		status.Layout = "ok"
	}
	status.Inventory = inspectInventory(projectDir)
	status.Assessment = inspectAssessment(projectDir)
	status.Mapping = inspectMapping(projectDir)
	status.Generated = inspectGenerated(projectDir)
	status.Validation = inspectValidation(projectDir)
	status.ReadyForExport = status.layoutOK() &&
		status.Inventory.Present && status.Inventory.Error == "" &&
		status.Assessment.Present && status.Assessment.Error == "" &&
		status.Mapping.Present && status.Mapping.Error == "" &&
		status.Generated.Present && status.Generated.Error == "" &&
		status.Validation.Present && status.Validation.Error == "" &&
		status.Validation.Status == "passed"
	status.NextActions = nextProjectActions(status)
	return status, nil
}

func RequiredProjectDirs() []string {
	out := make([]string, len(requiredProjectDirs))
	copy(out, requiredProjectDirs)
	return out
}

func (status *ProjectStatus) layoutOK() bool {
	return status != nil && status.ConfigOK && len(status.Missing) == 0
}

func inspectInventory(projectDir string) InventoryStatus {
	var inv inventory.Inventory
	present, errText := readOptionalYAML(projectDir, filepath.Join("inventory", "openexit.inventory.yaml"), &inv)
	status := InventoryStatus{Present: present}
	if !present {
		return status
	}
	status.Warnings = len(inv.Warnings)
	status.Assets = inventoryAssetCounts(&inv)
	if errText != "" {
		status.Error = errText
	} else if err := inventory.Validate(&inv); err != nil {
		status.Error = err.Error()
	}
	return status
}

func inspectAssessment(projectDir string) AssessmentStatus {
	var a assessment.Assessment
	present, errText := readOptionalYAML(projectDir, filepath.Join("assessment", "openexit.assessment.yaml"), &a)
	status := AssessmentStatus{Present: present}
	if !present {
		return status
	}
	status.Findings = len(a.Findings)
	status.Severity = severityCounts(a.Findings)
	status.Score = a.Score.Value
	status.Level = a.Score.Level
	status.Warnings = len(a.Warnings)
	if errText != "" {
		status.Error = errText
	} else if err := assessment.Validate(&a); err != nil {
		status.Error = err.Error()
	}
	return status
}

func inspectMapping(projectDir string) MappingStatus {
	var result mapping.MappingResult
	present, errText := readOptionalYAML(projectDir, filepath.Join("mapping", "openexit.mapping.yaml"), &result)
	status := MappingStatus{Present: present}
	if !present {
		return status
	}
	status.DashboardDrafts = len(result.DashboardDrafts)
	status.AlertRuleDrafts = len(result.AlertRuleDrafts)
	status.UnsupportedItems = len(result.UnsupportedItems)
	status.ManualReview = len(result.ManualReview)
	if errText != "" {
		status.Error = errText
	} else if err := mapping.Validate(&result); err != nil {
		status.Error = err.Error()
	}
	return status
}

func inspectGenerated(projectDir string) GeneratedStatus {
	root := filepath.Join(projectDir, "generated-config")
	status := GeneratedStatus{}
	info, err := os.Stat(root)
	if err != nil {
		if !os.IsNotExist(err) {
			status.Error = err.Error()
		}
		return status
	}
	if !info.IsDir() {
		status.Error = "generated-config is not a directory"
		return status
	}
	var artifacts []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		status.Files++
		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(strings.ToLower(filepath.Base(path)), "candidate") {
			status.Candidates++
			artifacts = append(artifacts, rel)
		}
		return nil
	})
	if err != nil {
		status.Error = err.Error()
	}
	status.Present = status.Files > 0
	sort.Strings(artifacts)
	if len(artifacts) > 8 {
		artifacts = artifacts[:8]
	}
	status.KeyArtifacts = artifacts
	return status
}

func inspectValidation(projectDir string) ValidationStatus {
	var report validate.Report
	present, errText := readOptionalYAML(projectDir, filepath.Join("validation", "openexit.validation.yaml"), &report)
	status := ValidationStatus{Present: present}
	if !present {
		return status
	}
	status.Status = strings.TrimSpace(report.Status)
	if status.Status == "" {
		status.Status = "unknown"
	}
	status.Checks = len(report.Checks)
	for _, check := range report.Checks {
		switch strings.ToLower(strings.TrimSpace(check.Status)) {
		case "passed":
			status.Passed++
		case "failed":
			status.Failed++
		case "warning":
			status.Warnings++
		}
	}
	if errText != "" {
		status.Error = errText
	}
	return status
}

func readOptionalYAML(projectDir, rel string, out any) (bool, string) {
	path := filepath.Join(projectDir, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ""
		}
		return true, err.Error()
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return true, err.Error()
	}
	return true, ""
}

func inventoryAssetCounts(inv *inventory.Inventory) map[string]int {
	counts := map[string]int{}
	addCount(counts, "dashboards", inv.Summary.Dashboards, len(inv.Assets.Dashboards))
	addCount(counts, "monitors", inv.Summary.Monitors, len(inv.Assets.Monitors))
	addCount(counts, "slos", inv.Summary.SLOs, len(inv.Assets.SLOs))
	addCount(counts, "integrations", inv.Summary.Integrations, len(inv.Assets.Integrations))
	addCount(counts, "uniqueMetrics", inv.Summary.UniqueMetrics, 0)
	addCount(counts, "notificationTargets", inv.Summary.NotificationTargets, 0)
	addCount(counts, "repositories", inv.Summary.Repositories, len(inv.Assets.Repositories))
	addCount(counts, "teams", inv.Summary.Teams, len(inv.Assets.Teams))
	addCount(counts, "branchProtections", inv.Summary.BranchProtections, len(inv.Assets.BranchProtections))
	addCount(counts, "actionsWorkflows", inv.Summary.ActionsWorkflows, len(inv.Assets.ActionsWorkflows))
	addCount(counts, "secrets", inv.Summary.Secrets, len(inv.Assets.Secrets))
	addCount(counts, "runners", inv.Summary.Runners, len(inv.Assets.Runners))
	addCount(counts, "deployKeys", inv.Summary.DeployKeys, len(inv.Assets.DeployKeys))
	addCount(counts, "githubApps", inv.Summary.GitHubApps, len(inv.Assets.GitHubApps))
	addCount(counts, "identityApplications", inv.Summary.IdentityApps, len(inv.Assets.IdentityApps))
	addCount(counts, "identityGroups", inv.Summary.IdentityGroups, len(inv.Assets.IdentityGroups))
	addCount(counts, "identityPolicies", inv.Summary.IdentityPolicies, len(inv.Assets.IdentityPolicies))
	addCount(counts, "mfaSettings", inv.Summary.MFASettings, len(inv.Assets.MFASettings))
	addCount(counts, "breakGlassAccounts", inv.Summary.BreakGlassAccounts, len(inv.Assets.BreakGlassAccounts))
	addCount(counts, "dnsRecords", inv.Summary.DNSRecords, len(inv.Assets.DNSRecords))
	addCount(counts, "wafRules", inv.Summary.WAFRules, len(inv.Assets.WAFRules))
	addCount(counts, "cacheRules", inv.Summary.CacheRules, len(inv.Assets.CacheRules))
	addCount(counts, "redirects", inv.Summary.Redirects, len(inv.Assets.Redirects))
	addCount(counts, "origins", inv.Summary.Origins, len(inv.Assets.Origins))
	addCount(counts, "tlsSettings", inv.Summary.TLSSettings, len(inv.Assets.TLSSettings))
	addCount(counts, "botRules", inv.Summary.BotRules, len(inv.Assets.BotRules))
	addCount(counts, "pageRules", inv.Summary.PageRules, len(inv.Assets.PageRules))
	addCount(counts, "aiModelUsageClasses", inv.Summary.AIModelUsageClasses, len(inv.Assets.AIModelUsageClasses))
	addCount(counts, "aiTokenVolumes", inv.Summary.AITokenVolumes, len(inv.Assets.AITokenVolumes))
	addCount(counts, "aiLatencyExpectations", inv.Summary.AILatencyExpectations, len(inv.Assets.AILatencyExpectations))
	addCount(counts, "aiSensitivePromptCategories", inv.Summary.AISensitivePromptCategories, len(inv.Assets.AISensitivePromptCategories))
	addCount(counts, "aiToolUsages", inv.Summary.AIToolUsages, len(inv.Assets.AIToolUsages))
	addCount(counts, "aiFallbackBehaviors", inv.Summary.AIFallbackBehaviors, len(inv.Assets.AIFallbackBehaviors))
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func addCount(counts map[string]int, name string, summaryCount, fallbackCount int) {
	value := summaryCount
	if value == 0 {
		value = fallbackCount
	}
	if value > 0 {
		counts[name] = value
	}
}

func severityCounts(findings []assessment.Finding) map[string]int {
	counts := map[string]int{}
	for _, finding := range findings {
		severity := strings.ToLower(strings.TrimSpace(finding.Severity))
		if severity == "" {
			severity = "unknown"
		}
		counts[severity]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func nextProjectActions(status *ProjectStatus) []string {
	if status == nil {
		return nil
	}
	projectArg := status.ProjectDir
	switch {
	case !status.layoutOK():
		return []string{"restore required project directories: " + strings.Join(status.Missing, ", ")}
	case !status.Inventory.Present:
		return []string{collectSuggestion(status.Source, projectArg)}
	case status.Inventory.Error != "":
		return []string{"fix inventory manifest, then rerun openexit collect"}
	case !status.Assessment.Present:
		return []string{fmt.Sprintf("openexit assess --project %s --target %s", projectArg, status.Target)}
	case status.Assessment.Error != "":
		return []string{"fix assessment manifest, then rerun openexit assess"}
	case !status.Mapping.Present:
		return []string{fmt.Sprintf("openexit map --project %s", projectArg)}
	case status.Mapping.Error != "":
		return []string{"fix mapping manifest, then rerun openexit map"}
	case !status.Generated.Present || status.Generated.Candidates == 0:
		return []string{fmt.Sprintf("openexit generate --project %s --all", projectArg)}
	case status.Generated.Error != "":
		return []string{"fix generated artifacts, then rerun openexit generate --all"}
	case !status.Validation.Present:
		return []string{fmt.Sprintf("openexit validate --project %s", projectArg)}
	case status.Validation.Error != "":
		return []string{"fix validation report, then rerun openexit validate"}
	case status.Validation.Status != "passed" || status.Validation.Failed > 0:
		return []string{fmt.Sprintf("fix failed validation checks, then rerun openexit validate --project %s", projectArg)}
	default:
		return []string{fmt.Sprintf("openexit export --project %s --format zip --out openexit-evidence.zip", projectArg)}
	}
}

func collectSuggestion(source, projectArg string) string {
	switch source {
	case "github-enterprise":
		return fmt.Sprintf("openexit collect github-fixture --project %s --input <fixture.json>", projectArg)
	case "identity":
		return fmt.Sprintf("openexit collect identity-fixture --project %s --input <fixture.json>", projectArg)
	case "edge":
		return fmt.Sprintf("openexit collect edge-fixture --project %s --input <fixture.json>", projectArg)
	case "ai-provider":
		return fmt.Sprintf("openexit collect ai-fixture --project %s --input <fixture.json>", projectArg)
	default:
		return fmt.Sprintf("openexit collect fixture --project %s --input <fixture.json>", projectArg)
	}
}
