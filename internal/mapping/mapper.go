package mapping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "Mapping"
)

type Mapper interface {
	Name() string
	Source() string
	Target() string
	Map(ctx context.Context, inv *inventory.Inventory, findings []assessment.Finding) (*MappingResult, error)
}

type MappingResult struct {
	APIVersion       string         `json:"apiVersion" yaml:"apiVersion"`
	Kind             string         `json:"kind" yaml:"kind"`
	Metadata         Metadata       `json:"metadata" yaml:"metadata"`
	Source           Endpoint       `json:"source" yaml:"source"`
	Target           Endpoint       `json:"target" yaml:"target"`
	TargetType       string         `json:"targetType" yaml:"targetType"`
	DashboardDrafts  []DashboardMap `json:"dashboardDrafts" yaml:"dashboardDrafts"`
	AlertRuleDrafts  []AlertRuleMap `json:"alertRuleDrafts" yaml:"alertRuleDrafts"`
	UnsupportedItems []Unsupported  `json:"unsupportedItems" yaml:"unsupportedItems"`
	ManualReview     []ManualReview `json:"manualReview" yaml:"manualReview"`
}

type Metadata struct {
	Project     string    `json:"project" yaml:"project"`
	GeneratedAt time.Time `json:"generatedAt" yaml:"generatedAt"`
}

type Endpoint struct {
	Type string `json:"type" yaml:"type"`
}

type DashboardMap struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Title    string `json:"title" yaml:"title"`
	Level    int    `json:"level" yaml:"level"`
	Path     string `json:"path" yaml:"path"`
}

type AlertRuleMap struct {
	SourceID     string `json:"sourceId" yaml:"sourceId"`
	Alert        string `json:"alert" yaml:"alert"`
	Path         string `json:"path" yaml:"path"`
	ReviewNeeded bool   `json:"reviewNeeded" yaml:"reviewNeeded"`
}

type Unsupported struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Type     string `json:"type" yaml:"type"`
	Reason   string `json:"reason" yaml:"reason"`
}

type ManualReview struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Reason   string `json:"reason" yaml:"reason"`
}

func Build(inv *inventory.Inventory, a *assessment.Assessment, generatedAt time.Time) (*MappingResult, error) {
	if inv == nil {
		return nil, fmt.Errorf("inventory is required")
	}
	if a == nil {
		return nil, fmt.Errorf("assessment is required")
	}
	result := &MappingResult{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Project:     a.Metadata.Project,
			GeneratedAt: generatedAt.UTC(),
		},
		Source:           Endpoint{Type: inv.Source.Type},
		Target:           Endpoint{Type: a.Target.Type},
		TargetType:       a.Target.Type,
		DashboardDrafts:  []DashboardMap{},
		AlertRuleDrafts:  []AlertRuleMap{},
		UnsupportedItems: []Unsupported{},
		ManualReview:     []ManualReview{},
	}
	switch inv.Source.Type {
	case "datadog":
		mapDatadog(result, inv, a)
	default:
		mapGeneric(result, inv, a)
	}
	sortMapping(result)
	if err := Validate(result); err != nil {
		return nil, err
	}
	return result, nil
}

func Write(projectDir string, result *MappingResult) error {
	ensureNonNilLists(result)
	if err := Validate(result); err != nil {
		return err
	}
	dir := filepath.Join(projectDir, "mapping")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(result)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.mapping.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.mapping.json"), append(jsonData, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "mapping-summary.md"), []byte(RenderMarkdown(result)), 0o644)
}

func ensureNonNilLists(result *MappingResult) {
	if result == nil {
		return
	}
	if result.DashboardDrafts == nil {
		result.DashboardDrafts = []DashboardMap{}
	}
	if result.AlertRuleDrafts == nil {
		result.AlertRuleDrafts = []AlertRuleMap{}
	}
	if result.UnsupportedItems == nil {
		result.UnsupportedItems = []Unsupported{}
	}
	if result.ManualReview == nil {
		result.ManualReview = []ManualReview{}
	}
}

func Validate(result *MappingResult) error {
	var problems []string
	if result.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if result.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if strings.TrimSpace(result.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}
	if strings.TrimSpace(result.Source.Type) == "" {
		problems = append(problems, "source.type is required")
	}
	if strings.TrimSpace(result.Target.Type) == "" {
		problems = append(problems, "target.type is required")
	}
	if strings.TrimSpace(result.TargetType) == "" {
		problems = append(problems, "targetType is required")
	}
	for _, draft := range result.DashboardDrafts {
		if draft.SourceID == "" || draft.Title == "" || draft.Path == "" {
			problems = append(problems, "dashboard drafts require sourceId, title, and path")
		}
	}
	for _, draft := range result.AlertRuleDrafts {
		if draft.SourceID == "" || draft.Alert == "" || draft.Path == "" {
			problems = append(problems, "alert rule drafts require sourceId, alert, and path")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func RenderMarkdown(result *MappingResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Mapping Summary")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Project: %s\n", result.Metadata.Project)
	fmt.Fprintf(&b, "- Source: %s\n", result.Source.Type)
	fmt.Fprintf(&b, "- Target: %s\n", result.Target.Type)
	fmt.Fprintf(&b, "- Generated: %s\n", result.Metadata.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&b, "- Dashboard drafts: %d\n", len(result.DashboardDrafts))
	fmt.Fprintf(&b, "- Alert rule drafts: %d\n", len(result.AlertRuleDrafts))
	fmt.Fprintf(&b, "- Unsupported items: %d\n", len(result.UnsupportedItems))
	fmt.Fprintf(&b, "- Manual review items: %d\n", len(result.ManualReview))
	fmt.Fprintln(&b)
	if len(result.DashboardDrafts) > 0 {
		fmt.Fprintln(&b, "## Dashboard Drafts")
		for _, draft := range result.DashboardDrafts {
			fmt.Fprintf(&b, "- %s: level=%d path=%s source=%s\n", draft.Title, draft.Level, draft.Path, draft.SourceID)
		}
		fmt.Fprintln(&b)
	}
	if len(result.AlertRuleDrafts) > 0 {
		fmt.Fprintln(&b, "## Alert Rule Drafts")
		for _, draft := range result.AlertRuleDrafts {
			fmt.Fprintf(&b, "- %s: reviewNeeded=%t path=%s source=%s\n", draft.Alert, draft.ReviewNeeded, draft.Path, draft.SourceID)
		}
		fmt.Fprintln(&b)
	}
	if len(result.UnsupportedItems) > 0 {
		fmt.Fprintln(&b, "## Unsupported Items")
		for _, item := range result.UnsupportedItems {
			fmt.Fprintf(&b, "- %s %s: %s\n", item.Type, item.SourceID, item.Reason)
		}
		fmt.Fprintln(&b)
	}
	if len(result.ManualReview) > 0 {
		fmt.Fprintln(&b, "## Manual Review")
		for _, item := range result.ManualReview {
			fmt.Fprintf(&b, "- %s: %s\n", item.SourceID, item.Reason)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "## Generated By OpenExit")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This mapping summary was generated deterministically from local OpenExit inventory and assessment manifests. It does not perform production writes.")
	return b.String()
}

func mapDatadog(result *MappingResult, inv *inventory.Inventory, a *assessment.Assessment) {
	for _, dashboard := range inv.Assets.Dashboards {
		level := 2
		if dashboard.Widgets.Unsupported > 0 {
			level = 1
			result.UnsupportedItems = append(result.UnsupportedItems, Unsupported{
				SourceID: dashboard.ID,
				Type:     "dashboard-widget",
				Reason:   fmt.Sprintf("%d unsupported widget(s) require Grafana panel review", dashboard.Widgets.Unsupported),
			})
		}
		if len(dashboard.Queries) == 0 {
			level = 0
		}
		result.DashboardDrafts = append(result.DashboardDrafts, DashboardMap{
			SourceID: dashboard.ID,
			Title:    dashboard.Title,
			Level:    level,
			Path:     "generated-config/grafana/dashboards/" + inventory.Slug(dashboard.Title) + ".candidate.json",
		})
	}
	for _, monitor := range inv.Assets.Monitors {
		result.AlertRuleDrafts = append(result.AlertRuleDrafts, AlertRuleMap{
			SourceID:     monitor.ID,
			Alert:        alertName(monitor.Name),
			Path:         "generated-config/prometheus/rules/datadog-monitor-candidates.yaml",
			ReviewNeeded: true,
		})
	}
	for _, slo := range inv.Assets.SLOs {
		result.UnsupportedItems = append(result.UnsupportedItems, Unsupported{
			SourceID: slo.ID,
			Type:     "slo",
			Reason:   "SLOs require explicit target SLI, dashboard, and burn-rate alert mapping",
		})
	}
	addManualReview(result, a)
}

func mapGeneric(result *MappingResult, inv *inventory.Inventory, a *assessment.Assessment) {
	switch inv.Source.Type {
	case "github-enterprise":
		for _, repo := range inv.Assets.Repositories {
			result.ManualReview = append(result.ManualReview, ManualReview{SourceID: "repository:" + repo.Name, Reason: "Review Forgejo repository settings, ownership, branch protections, CI, packages, and webhook behavior"})
		}
	case "identity":
		for _, app := range inv.Assets.IdentityApps {
			result.ManualReview = append(result.ManualReview, ManualReview{SourceID: "identity-application:" + app.ID, Reason: "Review target client protocol, redirect URIs, group assignments, MFA, and signing metadata"})
		}
	case "edge":
		for _, origin := range inv.Assets.Origins {
			result.ManualReview = append(result.ManualReview, ManualReview{SourceID: "origin:" + origin.ID, Reason: "Review target HAProxy backend, TLS verification, health checks, and traffic cutover"})
		}
		for _, rule := range inv.Assets.WAFRules {
			result.ManualReview = append(result.ManualReview, ManualReview{SourceID: "waf-rule:" + rule.ID, Reason: "Review Coraza/CRS equivalent and enforcement mode"})
		}
	case "ai-provider":
		for _, usageClass := range inv.Assets.AIModelUsageClasses {
			result.ManualReview = append(result.ManualReview, ManualReview{SourceID: "ai-model-usage-class:" + usageClass.ID, Reason: "Review target model route, evaluation, latency, capacity, tool, and fallback behavior"})
		}
	}
	addManualReview(result, a)
}

func addManualReview(result *MappingResult, a *assessment.Assessment) {
	seen := map[string]struct{}{}
	for _, item := range result.ManualReview {
		seen[item.SourceID+" "+item.Reason] = struct{}{}
	}
	for _, finding := range a.Findings {
		if finding.Severity != "high" && !strings.Contains(strings.ToLower(finding.Title), "manual") && !strings.Contains(strings.ToLower(finding.ID), "manual") {
			continue
		}
		sourceID := strings.Join(finding.AffectedAssets, ", ")
		if sourceID == "" {
			sourceID = finding.ID
		}
		reason := finding.Title + ": " + finding.Recommendation
		key := sourceID + " " + reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result.ManualReview = append(result.ManualReview, ManualReview{SourceID: sourceID, Reason: reason})
	}
}

func sortMapping(result *MappingResult) {
	sort.Slice(result.DashboardDrafts, func(i, j int) bool { return result.DashboardDrafts[i].SourceID < result.DashboardDrafts[j].SourceID })
	sort.Slice(result.AlertRuleDrafts, func(i, j int) bool { return result.AlertRuleDrafts[i].SourceID < result.AlertRuleDrafts[j].SourceID })
	sort.Slice(result.UnsupportedItems, func(i, j int) bool {
		if result.UnsupportedItems[i].Type == result.UnsupportedItems[j].Type {
			return result.UnsupportedItems[i].SourceID < result.UnsupportedItems[j].SourceID
		}
		return result.UnsupportedItems[i].Type < result.UnsupportedItems[j].Type
	})
	sort.Slice(result.ManualReview, func(i, j int) bool {
		if result.ManualReview[i].SourceID == result.ManualReview[j].SourceID {
			return result.ManualReview[i].Reason < result.ManualReview[j].Reason
		}
		return result.ManualReview[i].SourceID < result.ManualReview[j].SourceID
	})
}

var alertNamePattern = regexp.MustCompile(`[^A-Za-z0-9]+`)

func alertName(name string) string {
	parts := alertNamePattern.Split(name, -1)
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(titleCaseASCII(strings.ToLower(part)))
	}
	if b.Len() == 0 {
		return "DatadogMonitorCandidate"
	}
	return b.String() + "Candidate"
}

func titleCaseASCII(value string) string {
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "")
}
