package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type LiveCollector struct{}

func (LiveCollector) Name() string { return "datadog" }

func (LiveCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	apiKey, err := envSecret(req.Options["api-key-env"])
	if err != nil {
		return nil, err
	}
	appKey, err := envSecret(req.Options["app-key-env"])
	if err != nil {
		return nil, err
	}
	site := req.Options["site"]
	client, err := NewClient(site, apiKey, appKey)
	if err != nil {
		return nil, err
	}
	inv := inventory.New(req.Project, "datadog", site, version.Version, time.Now().UTC())
	var warnings []string
	if err := collectDashboards(ctx, client, req.ProjectDir, inv); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := collectMonitors(ctx, client, req.ProjectDir, inv); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := collectSLOs(ctx, client, req.ProjectDir, inv); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := collectIntegrations(ctx, client, req.ProjectDir, inv); err != nil {
		warnings = append(warnings, err.Error())
	}
	inv.Warnings = warnings
	inv.RecomputeSummary()
	if err := inventory.Validate(inv); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func collectDashboards(ctx context.Context, client *Client, projectDir string, inv *inventory.Inventory) error {
	var list struct {
		Dashboards []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"dashboards"`
	}
	if _, err := client.get(ctx, "/api/v1/dashboard", nil, &list); err != nil {
		return fmt.Errorf("collect dashboards: %w", err)
	}
	for _, item := range list.Dashboards {
		var raw map[string]any
		body, err := client.get(ctx, "/api/v1/dashboard/"+url.PathEscape(item.ID), nil, &raw)
		if err != nil {
			return fmt.Errorf("collect dashboard %s: %w", item.ID, err)
		}
		if err := writeEvidence(projectDir, "datadog/dashboards/"+safeID(item.ID)+".json", inventory.RedactBytes(body)); err != nil {
			return err
		}
		dashboard := dashboardFromRaw(item.ID, item.Title, item.URL, raw)
		for _, query := range dashboard.Queries {
			recordInventoryQueryMetricRefs(inv, query.Raw)
		}
		inv.Assets.Dashboards = append(inv.Assets.Dashboards, dashboard)
	}
	return nil
}

func dashboardFromRaw(id, title, dashboardURL string, raw map[string]any) inventory.Dashboard {
	if title == "" {
		if v, ok := raw["title"].(string); ok {
			title = v
		}
	}
	tags := stringSlice(raw["tags"])
	dataSources := dashboardRawDataSources(raw)
	templateVariables := dashboardRawTemplateVariables(raw)
	widgetsRaw, _ := raw["widgets"].([]any)
	var widgets []FixtureWidget
	var queries []inventory.Query
	for i, widget := range widgetsRaw {
		m, _ := widget.(map[string]any)
		def, _ := m["definition"].(map[string]any)
		kind, _ := def["type"].(string)
		source := rawString(def["data_source"])
		if source == "" {
			source = rawString(def["dataSource"])
		}
		widgets = append(widgets, FixtureWidget{Type: kind, DataSource: source})
		for _, q := range extractRawQueries(def) {
			queries = append(queries, inventory.Query{ID: queryID(id, len(queries)+i), Language: "datadog", Raw: inventory.RedactString(q)})
		}
	}
	dataSources = append(dataSources, dashboardDataSources(FixtureDashboard{Widgets: widgets})...)
	return inventory.Dashboard{
		ID:                id,
		Title:             title,
		URL:               inventory.RedactString(dashboardURL),
		Tags:              tags,
		DataSources:       uniqueSorted(dataSources),
		TemplateVariables: templateVariables,
		Widgets: inventory.WidgetSummary{
			Total:       len(widgets),
			Unsupported: countUnsupportedWidgets(widgets),
			Types:       FixtureDashboard{Widgets: widgets}.WidgetTypes(),
		},
		Queries:     queries,
		EvidenceRef: "evidence://datadog/dashboard/" + id,
	}
}

func extractRawQueries(def map[string]any) []string {
	var out []string
	requests, _ := def["requests"].([]any)
	for _, request := range requests {
		req, _ := request.(map[string]any)
		for _, key := range []string{"q", "query"} {
			if q, ok := req[key].(string); ok && q != "" {
				out = append(out, q)
			}
		}
		queries, _ := req["queries"].([]any)
		for _, query := range queries {
			qm, _ := query.(map[string]any)
			if q, ok := qm["query"].(string); ok && q != "" {
				out = append(out, q)
			}
		}
	}
	return out
}

func dashboardRawDataSources(raw map[string]any) []string {
	var out []string
	for _, key := range []string{"data_sources", "dataSources"} {
		out = append(out, stringSlice(raw[key])...)
	}
	return out
}

func dashboardRawTemplateVariables(raw map[string]any) []inventory.TemplateVariable {
	var out []inventory.TemplateVariable
	for _, key := range []string{"template_variables", "templateVariables"} {
		variables, _ := raw[key].([]any)
		for _, item := range variables {
			variable, _ := item.(map[string]any)
			name := rawString(variable["name"])
			query := rawString(variable["query"])
			if name == "" && query == "" {
				continue
			}
			out = append(out, inventory.TemplateVariable{Name: name, Query: inventory.RedactString(query)})
		}
	}
	return out
}

func collectMonitors(ctx context.Context, client *Client, projectDir string, inv *inventory.Inventory) error {
	pageSize := 100
	for page := 0; ; page++ {
		var monitors []liveMonitor
		query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
		body, err := client.get(ctx, "/api/v1/monitor", query, &monitors)
		if err != nil {
			return fmt.Errorf("collect monitors: %w", err)
		}
		if len(monitors) == 0 {
			break
		}
		if err := writeEvidence(projectDir, fmt.Sprintf("datadog/monitors/page-%d.json", page), inventory.RedactBytes(body)); err != nil {
			return err
		}
		for _, monitor := range monitors {
			monitorID := monitor.ID.String()
			if err := writeEvidence(projectDir, "datadog/monitors/"+safeID(monitorID)+".json", inventory.RedactBytes(prettyJSON(monitor))); err != nil {
				return err
			}
			recordInventoryQueryMetricRefs(inv, monitor.Query)
			inv.Assets.Monitors = append(inv.Assets.Monitors, inventory.Monitor{
				ID:                  monitorID,
				Name:                monitor.Name,
				Type:                monitor.Type,
				Query:               inventory.RedactString(monitor.Query),
				NotificationTargets: extractTargets(monitor.fixtureMonitor()),
				Tags:                monitor.Tags,
				RunbookURL:          monitor.RunbookURL,
				EvidenceRef:         "evidence://datadog/monitor/" + monitorID,
			})
		}
		if len(monitors) < pageSize {
			break
		}
	}
	return nil
}

func collectSLOs(ctx context.Context, client *Client, projectDir string, inv *inventory.Inventory) error {
	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name      string  `json:"name"`
				Target    float64 `json:"target_threshold"`
				Timeframe string  `json:"timeframe"`
			} `json:"attributes"`
		} `json:"data"`
	}
	body, err := client.get(ctx, "/api/v1/slo", nil, &out)
	if err != nil {
		return fmt.Errorf("collect slos: %w", err)
	}
	if err := writeEvidence(projectDir, "datadog/slos/list.json", inventory.RedactBytes(body)); err != nil {
		return err
	}
	for _, slo := range out.Data {
		if err := writeEvidence(projectDir, "datadog/slos/"+safeID(slo.ID)+".json", inventory.RedactBytes(prettyJSON(slo))); err != nil {
			return err
		}
		inv.Assets.SLOs = append(inv.Assets.SLOs, inventory.SLO{
			ID:          slo.ID,
			Name:        slo.Attributes.Name,
			Target:      slo.Attributes.Target,
			Timeframe:   slo.Attributes.Timeframe,
			EvidenceRef: "evidence://datadog/slo/" + slo.ID,
		})
	}
	return nil
}

func collectIntegrations(ctx context.Context, client *Client, projectDir string, inv *inventory.Inventory) error {
	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title      string   `json:"title"`
				Categories []string `json:"categories"`
				Installed  bool     `json:"installed"`
			} `json:"attributes"`
		} `json:"data"`
	}
	body, err := client.get(ctx, "/api/v2/integrations", nil, &out)
	if err != nil {
		return fmt.Errorf("collect integrations: %w", err)
	}
	if err := writeEvidence(projectDir, "datadog/integrations/list.json", inventory.RedactBytes(body)); err != nil {
		return err
	}
	for _, item := range out.Data {
		name := firstNonEmpty(item.Attributes.Title, item.ID)
		if name == "" {
			continue
		}
		integration := inventory.Integration{
			Name:        name,
			Enabled:     item.Attributes.Installed,
			Tags:        uniqueSorted(item.Attributes.Categories),
			EvidenceRef: "evidence://datadog/integration/" + safeID(name),
		}
		if err := writeEvidence(projectDir, "datadog/integrations/"+safeID(name)+".json", inventory.RedactBytes(prettyJSON(item))); err != nil {
			return err
		}
		inv.Assets.Integrations = append(inv.Assets.Integrations, integration)
	}
	sort.Slice(inv.Assets.Integrations, func(i, j int) bool { return inv.Assets.Integrations[i].Name < inv.Assets.Integrations[j].Name })
	return nil
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func rawString(value any) string {
	text, _ := value.(string)
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func uniqueSorted(values []string) []string {
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

func prettyJSON(value any) []byte {
	data, _ := json.MarshalIndent(value, "", "  ")
	return data
}

type liveMonitor struct {
	ID                  stringID `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Query               string   `json:"query"`
	Message             string   `json:"message"`
	NotificationTargets []string `json:"notificationTargets"`
	Tags                []string `json:"tags"`
	RunbookURL          string   `json:"runbookUrl"`
}

func (m liveMonitor) fixtureMonitor() FixtureMonitor {
	return FixtureMonitor{
		ID:                  m.ID.String(),
		Name:                m.Name,
		Type:                m.Type,
		Query:               m.Query,
		Message:             m.Message,
		NotificationTargets: m.NotificationTargets,
		Tags:                m.Tags,
		RunbookURL:          m.RunbookURL,
	}
}

type stringID string

func (id *stringID) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*id = stringID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*id = stringID(number.String())
		return nil
	}
	return fmt.Errorf("invalid id %s", string(data))
}

func (id stringID) String() string {
	return string(id)
}
