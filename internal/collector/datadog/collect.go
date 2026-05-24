package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
	"gopkg.in/yaml.v3"
)

type FixtureCollector struct{}

func (FixtureCollector) Name() string { return "datadog-fixture" }

func (FixtureCollector) Collect(_ context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	input := req.Options["input"]
	if input == "" {
		return nil, fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	data = inventory.RedactBytes(data)
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	site := fixture.Site
	if site == "" {
		site = req.Options["site"]
	}
	inv := inventory.New(req.Project, "datadog", site, version.Version, time.Now().UTC())
	if err := normalizeFixture(req.ProjectDir, &fixture, inv, data); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func normalizeFixture(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte) error {
	if err := writeEvidence(projectDir, "datadog/raw-fixture.json", raw); err != nil {
		return err
	}
	metricNames := map[string]inventory.MetricRef{}
	for _, dashboard := range fixture.Dashboards {
		dashBytes, err := json.MarshalIndent(dashboard, "", "  ")
		if err != nil {
			return err
		}
		evidencePath := fmt.Sprintf("datadog/dashboards/%s.json", safeID(dashboard.ID))
		if err := writeEvidence(projectDir, evidencePath, inventory.RedactBytes(dashBytes)); err != nil {
			return err
		}
		queries := dashboardQueries(dashboard)
		for _, query := range queries {
			for _, name := range inventory.ExtractMetricNames(query.Raw) {
				metricNames[name] = inventory.MetricRef{Name: name}
			}
		}
		widgetTypes := dashboard.WidgetTypes()
		inv.Assets.Dashboards = append(inv.Assets.Dashboards, inventory.Dashboard{
			ID:    dashboard.ID,
			Title: dashboard.Title,
			URL:   inventory.RedactString(dashboard.URL),
			Tags:  append([]string{}, dashboard.Tags...),
			Widgets: inventory.WidgetSummary{
				Total:       len(dashboard.Widgets),
				Unsupported: countUnsupportedWidgets(dashboard.Widgets),
				Types:       widgetTypes,
			},
			Queries:     queries,
			EvidenceRef: "evidence://datadog/dashboard/" + dashboard.ID,
		})
	}
	for _, monitor := range fixture.Monitors {
		monitorBytes, err := json.MarshalIndent(monitor, "", "  ")
		if err != nil {
			return err
		}
		evidencePath := fmt.Sprintf("datadog/monitors/%s.json", safeID(monitor.ID))
		if err := writeEvidence(projectDir, evidencePath, inventory.RedactBytes(monitorBytes)); err != nil {
			return err
		}
		for _, name := range inventory.ExtractMetricNames(monitor.Query) {
			metricNames[name] = inventory.MetricRef{Name: name}
		}
		inv.Assets.Monitors = append(inv.Assets.Monitors, inventory.Monitor{
			ID:                  monitor.ID,
			Name:                monitor.Name,
			Type:                monitor.Type,
			Query:               inventory.RedactString(monitor.Query),
			NotificationTargets: extractTargets(monitor),
			Tags:                append([]string{}, monitor.Tags...),
			RunbookURL:          monitor.RunbookURL,
			EvidenceRef:         "evidence://datadog/monitor/" + monitor.ID,
		})
	}
	for _, slo := range fixture.SLOs {
		sloBytes, err := json.MarshalIndent(slo, "", "  ")
		if err != nil {
			return err
		}
		evidencePath := fmt.Sprintf("datadog/slos/%s.json", safeID(slo.ID))
		if err := writeEvidence(projectDir, evidencePath, inventory.RedactBytes(sloBytes)); err != nil {
			return err
		}
		inv.Assets.SLOs = append(inv.Assets.SLOs, inventory.SLO{
			ID:          slo.ID,
			Name:        slo.Name,
			Target:      slo.Target,
			Timeframe:   slo.Timeframe,
			EvidenceRef: "evidence://datadog/slo/" + slo.ID,
		})
	}
	for _, integration := range fixture.Integrations {
		inv.Assets.Integrations = append(inv.Assets.Integrations, inventory.Integration{
			Name:    integration.Name,
			Enabled: integration.Enabled,
			Tags:    append([]string{}, integration.Tags...),
		})
	}
	for _, metric := range fixture.Metrics {
		if metric.Name != "" {
			metricNames[metric.Name] = inventory.MetricRef{Name: metric.Name, Tags: append([]string{}, metric.Tags...)}
		}
	}
	for _, metric := range metricNames {
		inv.Assets.Metrics = append(inv.Assets.Metrics, metric)
	}
	sort.Slice(inv.Assets.Metrics, func(i, j int) bool { return inv.Assets.Metrics[i].Name < inv.Assets.Metrics[j].Name })
	sort.Slice(inv.Assets.Dashboards, func(i, j int) bool { return inv.Assets.Dashboards[i].ID < inv.Assets.Dashboards[j].ID })
	sort.Slice(inv.Assets.Monitors, func(i, j int) bool { return inv.Assets.Monitors[i].ID < inv.Assets.Monitors[j].ID })
	sort.Slice(inv.Assets.SLOs, func(i, j int) bool { return inv.Assets.SLOs[i].ID < inv.Assets.SLOs[j].ID })
	inv.RecomputeSummary()
	return inventory.Validate(inv)
}

func writeInventory(projectDir string, inv *inventory.Inventory) error {
	invDir := filepath.Join(projectDir, "inventory")
	if err := os.MkdirAll(invDir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(inv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(invDir, "openexit.inventory.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(invDir, "openexit.inventory.json"), jsonData, 0o644)
}

func writeEvidence(projectDir, rel string, data []byte) error {
	path := filepath.Join(projectDir, "evidence", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func countUnsupportedWidgets(widgets []FixtureWidget) int {
	count := 0
	for _, widget := range widgets {
		if !supportedWidget(widget.Type) {
			count++
		}
	}
	return count
}

func supportedWidget(kind string) bool {
	switch strings.ToLower(kind) {
	case "timeseries", "query_value", "toplist", "heatmap", "scatterplot":
		return true
	default:
		return false
	}
}

func dashboardQueries(d FixtureDashboard) []inventory.Query {
	var out []inventory.Query
	for i, q := range d.Queries {
		out = append(out, inventory.Query{ID: queryID(d.ID, i), Language: "datadog", Raw: inventory.RedactString(q)})
	}
	for i, widget := range d.Widgets {
		if widget.Query != "" {
			out = append(out, inventory.Query{ID: queryID(d.ID, len(out)+i), Language: "datadog", Raw: inventory.RedactString(widget.Query)})
		}
	}
	return out
}

func queryID(dashboardID string, idx int) string {
	return fmt.Sprintf("%s-query-%d", safeID(dashboardID), idx+1)
}

func extractTargets(m FixtureMonitor) []string {
	seen := map[string]struct{}{}
	for _, target := range m.NotificationTargets {
		if target != "" {
			seen[target] = struct{}{}
		}
	}
	for _, target := range parseMentions(m.Message) {
		seen[target] = struct{}{}
	}
	var out []string
	for target := range seen {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

func parseMentions(message string) []string {
	var out []string
	for _, field := range strings.Fields(message) {
		if strings.HasPrefix(field, "@") {
			out = append(out, strings.Trim(field, ".,;:"))
		}
	}
	return out
}

func safeID(id string) string {
	return inventory.Slug(id)
}
