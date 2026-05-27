package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
	openmapping "github.com/RamazanKara/openexit/internal/mapping"
)

type grafanaDashboardCandidate struct {
	Title         string            `json:"title"`
	Tags          []string          `json:"tags"`
	Timezone      string            `json:"timezone"`
	SchemaVersion int               `json:"schemaVersion"`
	Panels        []grafanaPanel    `json:"panels"`
	Templating    map[string]any    `json:"templating"`
	Annotations   map[string]any    `json:"annotations"`
	OpenExit      map[string]string `json:"openexit"`
}

type grafanaPanel struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	GridPos     map[string]int `json:"gridPos"`
	Description string         `json:"description"`
	Targets     []any          `json:"targets"`
}

func addGrafanaDashboardCandidateChecks(projectDir string, inv *inventory.Inventory, result *openmapping.MappingResult, add func(string, string, string, bool)) {
	if inv == nil || result == nil {
		return
	}
	if len(result.DashboardDrafts) == 0 {
		return
	}
	var problems []string
	dashboardsByID := map[string]inventory.Dashboard{}
	for _, dashboard := range inv.Assets.Dashboards {
		dashboardsByID[dashboard.ID] = dashboard
	}
	expected := map[string]struct{}{}
	for _, draft := range result.DashboardDrafts {
		expected[filepath.ToSlash(draft.Path)] = struct{}{}
		path, err := projectRelativePath(projectDir, draft.Path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", draft.Path, err))
			continue
		}
		sourceDashboard, ok := dashboardsByID[draft.SourceID]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: source dashboard %q is not present in inventory", draft.Path, draft.SourceID))
			continue
		}
		candidate, err := readGrafanaDashboardCandidate(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", draft.Path, err))
			continue
		}
		problems = append(problems, validateGrafanaDashboardCandidate(draft, sourceDashboard, candidate)...)
	}
	problems = append(problems, validateGrafanaCandidateSet(projectDir, expected)...)
	problems = append(problems, validateGrafanaUnsupportedWidgetREADME(projectDir, inv)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("grafana-dashboard-candidates", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("grafana-dashboard-candidates", "passed", "", true)
}

func readGrafanaDashboardCandidate(path string) (*grafanaDashboardCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var candidate grafanaDashboardCandidate
	if err := json.Unmarshal(data, &candidate); err != nil {
		return nil, err
	}
	return &candidate, nil
}

func validateGrafanaDashboardCandidate(draft openmapping.DashboardMap, source inventory.Dashboard, candidate *grafanaDashboardCandidate) []string {
	var problems []string
	prefix := draft.Path
	if strings.TrimSpace(candidate.Title) == "" {
		problems = append(problems, prefix+": title is required")
	} else if candidate.Title != draft.Title {
		problems = append(problems, fmt.Sprintf("%s: title %q does not match mapping title %q", prefix, candidate.Title, draft.Title))
	}
	for _, tag := range []string{"openexit", "candidate", "source:datadog"} {
		if !containsString(candidate.Tags, tag) {
			problems = append(problems, fmt.Sprintf("%s: missing required tag %q", prefix, tag))
		}
	}
	if candidate.Timezone == "" {
		problems = append(problems, prefix+": timezone is required")
	}
	if candidate.SchemaVersion <= 0 {
		problems = append(problems, prefix+": schemaVersion must be positive")
	}
	if candidate.Panels == nil {
		problems = append(problems, prefix+": panels must be an array")
	}
	if list, ok := candidate.Templating["list"].([]any); !ok || list == nil {
		problems = append(problems, prefix+": templating.list must be an array")
	}
	if list, ok := candidate.Annotations["list"].([]any); !ok || list == nil {
		problems = append(problems, prefix+": annotations.list must be an array")
	}
	problems = append(problems, validateOpenExitDashboardMetadata(prefix, draft, source, candidate.OpenExit)...)
	if len(source.Queries) != len(candidate.Panels) {
		problems = append(problems, fmt.Sprintf("%s: panel count %d does not match source query count %d", prefix, len(candidate.Panels), len(source.Queries)))
	}
	for i, panel := range candidate.Panels {
		problems = append(problems, validateGrafanaPanel(prefix, i, panel)...)
	}
	return problems
}

func validateOpenExitDashboardMetadata(prefix string, draft openmapping.DashboardMap, source inventory.Dashboard, metadata map[string]string) []string {
	var problems []string
	if metadata == nil {
		return []string{prefix + ": openexit metadata is required"}
	}
	expected := map[string]string{
		"candidate":         "true",
		"productionReady":   "false",
		"source":            "datadog",
		"sourceDashboardID": draft.SourceID,
		"evidenceRef":       source.EvidenceRef,
	}
	for key, want := range expected {
		if got := metadata[key]; got != want {
			problems = append(problems, fmt.Sprintf("%s: openexit.%s must be %q, got %q", prefix, key, want, got))
		}
	}
	return problems
}

func validateGrafanaPanel(prefix string, idx int, panel grafanaPanel) []string {
	var problems []string
	panelName := fmt.Sprintf("%s panel %d", prefix, idx+1)
	if panel.Type == "" {
		problems = append(problems, panelName+": type is required")
	}
	if panel.Title == "" {
		problems = append(problems, panelName+": title is required")
	}
	if panel.GridPos["h"] <= 0 || panel.GridPos["w"] <= 0 {
		problems = append(problems, panelName+": gridPos h and w must be positive")
	}
	if !strings.Contains(panel.Description, "OpenExit candidate panel") {
		problems = append(problems, panelName+": description must identify the panel as an OpenExit candidate")
	}
	if len(panel.Targets) == 0 {
		problems = append(problems, panelName+": at least one target is required")
	}
	for targetIdx, target := range panel.Targets {
		targetMap, ok := target.(map[string]any)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s target %d: target must be an object", panelName, targetIdx+1))
			continue
		}
		if strings.TrimSpace(stringValue(targetMap["openexit_source_query"])) == "" {
			problems = append(problems, fmt.Sprintf("%s target %d: openexit_source_query is required", panelName, targetIdx+1))
		}
	}
	return problems
}

func validateGrafanaCandidateSet(projectDir string, expected map[string]struct{}) []string {
	var problems []string
	dir := filepath.Join(projectDir, "generated-config", "grafana", "dashboards")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"generated-config/grafana/dashboards: directory is required when mapping contains dashboard drafts"}
		}
		return []string{err.Error()}
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".candidate.json") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("generated-config", "grafana", "dashboards", entry.Name()))
		if _, ok := expected[rel]; !ok {
			problems = append(problems, rel+": candidate is not referenced by mapping/openexit.mapping.yaml")
		}
	}
	return problems
}

func validateGrafanaUnsupportedWidgetREADME(projectDir string, inv *inventory.Inventory) []string {
	var expected []inventory.Dashboard
	for _, dashboard := range inv.Assets.Dashboards {
		if dashboard.Widgets.Unsupported > 0 {
			expected = append(expected, dashboard)
		}
	}
	if len(expected) == 0 {
		return nil
	}
	path := filepath.Join(projectDir, "generated-config", "grafana", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/grafana/README.md: " + err.Error()}
	}
	readme := string(data)
	var problems []string
	for _, dashboard := range expected {
		if !strings.Contains(readme, dashboard.ID) || !strings.Contains(readme, fmt.Sprintf("%d unsupported widget", dashboard.Widgets.Unsupported)) {
			problems = append(problems, fmt.Sprintf("generated-config/grafana/README.md: unsupported widgets for dashboard %q are not documented", dashboard.ID))
		}
	}
	return problems
}

func projectRelativePath(projectDir, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be project-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project root")
	}
	return filepath.Join(projectDir, clean), nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
