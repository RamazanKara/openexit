package datadogplan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type grafanaDashboard struct {
	Title         string         `json:"title"`
	Tags          []string       `json:"tags"`
	Timezone      string         `json:"timezone"`
	SchemaVersion int            `json:"schemaVersion"`
	Version       int            `json:"version"`
	Editable      bool           `json:"editable"`
	Panels        []grafanaPanel `json:"panels"`
	Templating    map[string]any `json:"templating"`
	Annotations   map[string]any `json:"annotations"`
	OpenExit      map[string]any `json:"openexit"`
}

type grafanaPanel struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	GridPos     map[string]int `json:"gridPos"`
	Datasource  any            `json:"datasource,omitempty"`
	Targets     []any          `json:"targets,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
}

type prometheusRuleFile struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

type prometheusRuleGroup struct {
	Name  string                `yaml:"name"`
	Rules []prometheusAlertRule `yaml:"rules"`
}

type prometheusAlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func generateConversions(stage string, inv *Inventory) ([]Conversion, error) {
	configSources := configSourceRefs(inv.Resources)
	if err := generateTelemetryConfigs(stage, configSources); err != nil {
		return nil, err
	}
	conversions := make([]Conversion, 0, len(inv.Resources))
	for _, resource := range inv.Resources {
		conversion := baseConversion(resource)
		var err error
		switch resource.Kind {
		case "dashboard":
			conversion, err = generateDashboard(stage, resource)
		case "monitor":
			conversion, err = generateMonitor(stage, resource)
		case "synthetic_test", "synthetic_variable", "synthetic_location":
			conversion.Status = StatusUnsupported
			conversion.ReasonCodes = []string{"target.synthetic-capability-out-of-scope"}
			conversion.Summary = "Synthetic Monitoring is outside the Grafana/Prometheus/OpenTelemetry v0.1 target."
		case "integration", "aws_integration", "azure_integration", "gcp_integration", "gcp_legacy_integration", "log_pipeline", "log_pipeline_order", "log_index", "log_archive", "log_metric", "apm_retention_filter", "span_metric", "service_definition":
			conversion.Status = StatusManual
			conversion.ReasonCodes = []string{"telemetry.topology-review"}
			conversion.Summary = "Detected configuration is linked to candidate Alloy and OpenTelemetry pipelines but requires topology-specific reconstruction."
			conversion.Outputs = []OutputRef{
				{Path: "generated/alloy/config.alloy", Kind: "alloy-config"},
				{Path: "generated/opentelemetry/collector.yaml", Kind: "opentelemetry-config"},
			}
			conversion.SemanticChanges = []SemanticChange{{
				Code:        "telemetry.generic-otlp-pipeline",
				Description: "OpenExit emits a credential-free OTLP baseline and preserves the Datadog configuration as evidence instead of guessing receivers, endpoints, or secrets.",
				Impact:      "manual",
			}}
		case "metric":
			conversion.Status = StatusManual
			conversion.ReasonCodes = []string{"metric.identity-review"}
			conversion.Summary = "Metric metadata is inventoried, but metric identity, label cardinality, and instrumentation ownership require review."
		case "slo", "slo_correction":
			conversion.Status = StatusManual
			conversion.ReasonCodes = []string{"slo.sli-reconstruction"}
			conversion.Summary = "SLO targets are preserved, but the target SLI and burn-rate rules require explicit reconstruction."
		case "downtime", "monitor_policy":
			conversion.Status = StatusManual
			conversion.ReasonCodes = []string{"alerting.policy-review"}
			conversion.Summary = "Alerting policy requires manual mapping to Grafana Alerting or Alertmanager semantics."
		case "dashboard_list", "powerpack", "notebook":
			conversion.Status = StatusManual
			conversion.ReasonCodes = []string{"content.organization-review"}
			conversion.Summary = "Content and organization metadata is preserved for manual reconstruction."
		default:
			conversion.Status = StatusUnsupported
			conversion.ReasonCodes = []string{"resource.converter-unavailable"}
			conversion.Summary = "No deterministic converter exists for this resource kind."
		}
		if err != nil {
			return nil, err
		}
		conversion.ReasonCodes = SortedUnique(conversion.ReasonCodes)
		conversion.Outputs = sortOutputs(conversion.Outputs)
		conversions = append(conversions, conversion)
	}
	sort.Slice(conversions, func(i, j int) bool { return conversions[i].SourceRef < conversions[j].SourceRef })
	return conversions, nil
}

func baseConversion(resource Resource) Conversion {
	return Conversion{
		SourceRef:    resource.Ref,
		SourceKind:   resource.Kind,
		SourceName:   resource.Name,
		SourceURL:    resource.SourceURL,
		EvidencePath: resource.Evidence.Path,
		Status:       StatusManual,
		ReasonCodes:  []string{},
		Summary:      "Manual review required.",
		Outputs:      []OutputRef{},
	}
}

func generateDashboard(stage string, resource Resource) (Conversion, error) {
	conversion := baseConversion(resource)
	widgets := dashboardWidgets(resource.Spec)
	panels := make([]grafanaPanel, 0, len(widgets)*2)
	components := make([]Component, 0, len(widgets))
	appendPanel := func(panel grafanaPanel) {
		index := len(panels)
		panel.ID = index + 1
		panel.GridPos = map[string]int{"h": 8, "w": 12, "x": (index % 2) * 12, "y": (index / 2) * 8}
		panels = append(panels, panel)
	}
	manual := false
	approximate := false
	for index, widget := range widgets {
		kind := strings.ToLower(firstString(widget, "type", "kind"))
		if kind == "" {
			kind = "unknown"
		}
		title := firstString(widget, "title", "name")
		if title == "" {
			title = fmt.Sprintf("Datadog widget %d", index+1)
		}
		entries := dashboardQueryEntries(widget)
		if len(entries) == 0 {
			component := Component{ID: fmt.Sprintf("widget-%d", index+1), Kind: kind}
			if content := firstString(widget, "text", "content", "note"); isTextWidget(kind) && content != "" {
				component.Status = StatusExact
				component.ReasonCodes = []string{"dashboard.text-preserved"}
				appendPanel(grafanaPanel{Type: "text", Title: title, Description: "OpenExit candidate derived from " + resource.Ref + ".", Options: map[string]any{"mode": "markdown", "content": content}})
			} else {
				component.Status = StatusManual
				component.ReasonCodes = []string{"dashboard.widget-unsupported"}
				component.Review = "Widget has no deterministic Grafana representation."
				manual = true
				appendPanel(grafanaPanel{Type: "text", Title: title, Description: "OpenExit candidate derived from " + resource.Ref + ".", Options: map[string]any{"mode": "markdown", "content": "### Manual widget reconstruction required\n\nDatadog widget type: `" + kind + "`"}})
			}
			components = append(components, component)
			continue
		}

		var targets []any
		var manualReview strings.Builder
		for queryIndex, entry := range entries {
			component := Component{ID: fmt.Sprintf("widget-%d-query-%d", index+1, queryIndex+1), Kind: kind + "/" + entry.Field, SourceQuery: entry.Value}
			result := convertDashboardMetricQuery(entry.Value)
			if entry.Field == "formula" {
				result = queryConversion{ReasonCode: "query.formula", Review: "Datadog formulas require manual reconstruction after their named queries are mapped."}
			}
			if result.OK {
				component.Status = StatusApproximate
				component.ReasonCodes = []string{result.ReasonCode}
				component.TargetQuery = result.Expr
				component.Review = result.Review
				approximate = true
				targets = append(targets, map[string]any{
					"refId":                  grafanaRefID(len(targets)),
					"expr":                   result.Expr,
					"legendFormat":           "{{instance}}",
					"openexitSourcePath":     entry.Path,
					"openexitSourceQuery":    entry.Value,
					"openexitStatus":         StatusApproximate,
					"openexitReviewRequired": true,
				})
			} else {
				component.Status = StatusManual
				component.ReasonCodes = []string{result.ReasonCode}
				component.Review = result.Review
				manual = true
				manualReview.WriteString("#### `" + entry.Path + "`\n\n")
				manualReview.WriteString(indentMarkdownCode(entry.Value) + "\n\n" + result.Review + "\n\n")
			}
			components = append(components, component)
		}
		if len(targets) > 0 {
			appendPanel(grafanaPanel{
				Type: grafanaPanelType(kind), Title: title, Description: "OpenExit candidate derived from " + resource.Ref + ". Review before import.",
				Datasource: map[string]string{"type": "prometheus", "uid": "${DS_PROMETHEUS}"}, Targets: targets,
			})
		}
		if manualReview.Len() > 0 {
			appendPanel(grafanaPanel{
				Type: "text", Title: title + " — manual conversion", Description: "Unconverted source expressions from " + resource.Ref + ".",
				Options: map[string]any{"mode": "markdown", "content": "### Manual query conversion required\n\n" + manualReview.String()},
			})
		}
	}
	if len(panels) == 0 {
		manual = true
		components = append(components, Component{ID: "dashboard", Kind: "dashboard", Status: StatusManual, ReasonCodes: []string{"dashboard.empty-definition"}, Review: "Dashboard has no convertible widgets or queries."})
		appendPanel(grafanaPanel{Type: "text", Title: "Manual dashboard review", Description: "OpenExit candidate derived from " + resource.Ref + ".", Options: map[string]any{"mode": "markdown", "content": "No convertible Datadog widgets were found. Review the source evidence."}})
	}
	// Even a text-only dashboard is not lossless at the resource level: Grafana
	// receives a normalized grid, candidate metadata, and a new data-source
	// binding. Exact is reserved for component-level content we can preserve.
	status := StatusApproximate
	reasons := []string{"dashboard.layout-approximation", "dashboard.metadata-preserved"}
	changes := []SemanticChange{{
		Code:        "dashboard.layout-normalized",
		Description: "Datadog widget placement is normalized to a review grid and Grafana candidate metadata is added.",
		Impact:      "review",
	}}
	if approximate {
		reasons = append(reasons, "dashboard.promql-approximation")
		changes = append(changes, SemanticChange{Code: "dashboard.datasource-change", Description: "Datadog metric queries were rewritten as PromQL candidates using a placeholder Prometheus data source UID.", Impact: "review"})
	}
	if manual {
		status = StatusManual
		reasons = append(reasons, "dashboard.manual-components")
		changes = append(changes, SemanticChange{Code: "dashboard.manual-panels", Description: "Unconverted widgets are rendered as visible text review panels rather than fake data queries.", Impact: "manual"})
	}
	dashboard := grafanaDashboard{
		Title: resource.Name, Tags: SortedUnique(append([]string{"openexit", "candidate", "source:datadog"}, resource.Tags...)),
		Timezone: "browser", SchemaVersion: 39, Version: 1, Editable: true, Panels: panels,
		Templating: map[string]any{"list": []any{map[string]any{
			"name": "DS_PROMETHEUS", "label": "Prometheus", "type": "datasource", "query": "prometheus", "refresh": 1,
		}}}, Annotations: map[string]any{"list": []any{}},
		OpenExit: map[string]any{"sourceRef": resource.Ref, "evidencePath": resource.Evidence.Path, "status": status, "productionReady": false, "rulesetVersion": RulesetVersion},
	}
	name := safeFilename(resource.ID, resource.Ref) + ".json"
	rel := "generated/grafana/dashboards/" + name
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(rel)), dashboard); err != nil {
		return Conversion{}, err
	}
	conversion.Status = status
	conversion.ReasonCodes = reasons
	conversion.Summary = "Generated a reviewable Grafana dashboard candidate with explicit per-widget conversion results."
	conversion.SemanticChanges = changes
	conversion.Components = components
	conversion.Outputs = []OutputRef{{Path: rel, Kind: "grafana-dashboard"}}
	return conversion, nil
}

type dashboardQueryEntry struct {
	Path  string
	Field string
	Value string
}

func dashboardQueryEntries(value any) []dashboardQueryEntry {
	var entries []dashboardQueryEntry
	collectDashboardQueryEntries(value, "widget", &entries)
	return entries
}

func collectDashboardQueryEntries(value any, path string, entries *[]dashboardQueryEntry) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := path + "." + key
			if key == "query" || key == "q" || key == "formula" {
				if query := stringValue(typed[key]); query != "" {
					*entries = append(*entries, dashboardQueryEntry{Path: childPath, Field: key, Value: query})
					continue
				}
			}
			collectDashboardQueryEntries(typed[key], childPath, entries)
		}
	case []any:
		for index, child := range typed {
			collectDashboardQueryEntries(child, fmt.Sprintf("%s[%d]", path, index), entries)
		}
	}
}

func grafanaRefID(index int) string {
	if index >= 0 && index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("Q%d", index+1)
}

func indentMarkdownCode(value string) string {
	return "    " + strings.ReplaceAll(value, "\n", "\n    ")
}

func generateMonitor(stage string, resource Resource) (Conversion, error) {
	conversion := baseConversion(resource)
	query := specString(resource.Spec, "query")
	if query == "" {
		query = specString(resource.Spec, "attributes", "query")
	}
	result := convertMonitorQuery(query)
	component := Component{ID: "monitor-query", Kind: "prometheus-alert", SourceQuery: query, ReasonCodes: []string{result.ReasonCode}, Review: result.Review}
	if !result.OK {
		component.Status = StatusManual
		conversion.Status = StatusManual
		conversion.ReasonCodes = []string{result.ReasonCode}
		conversion.Summary = "No executable alert rule was emitted because the Datadog monitor is outside the safe deterministic subset."
		conversion.Components = []Component{component}
		return conversion, nil
	}
	component.Status = StatusApproximate
	component.TargetQuery = result.Expr
	alertName := promAlertName(resource.Name, resource.Ref)
	rules := prometheusRuleFile{Groups: []prometheusRuleGroup{{
		Name: "openexit-datadog-" + DigestBytes([]byte(resource.Ref))[:8],
		Rules: []prometheusAlertRule{{
			Alert: alertName,
			Expr:  result.Expr,
			For:   "0m",
			Labels: map[string]string{
				"severity": "warning", "openexit_candidate": "true", "production_ready": "false",
				"source": "datadog", "source_ref": resource.Ref, "conversion": StatusApproximate,
			},
			Annotations: map[string]string{
				"summary": resource.Name, "openexit_source_query": query, "openexit_review": result.Review,
			},
		}},
	}}}
	data, err := yaml.Marshal(rules)
	if err != nil {
		return Conversion{}, err
	}
	rel := "generated/prometheus/rules/" + safeFilename(resource.ID, resource.Ref) + ".yaml"
	if err := WriteText(filepath.Join(stage, filepath.FromSlash(rel)), string(data)); err != nil {
		return Conversion{}, err
	}
	conversion.Status = StatusApproximate
	conversion.ReasonCodes = []string{result.ReasonCode}
	conversion.Summary = "Generated a Prometheus alert-rule candidate for a recognized static Datadog threshold."
	conversion.Components = []Component{component}
	conversion.SemanticChanges = []SemanticChange{{Code: "monitor.evaluation-semantics", Description: result.Review, Impact: "review"}}
	conversion.Outputs = []OutputRef{{Path: rel, Kind: "prometheus-alert-rule"}}
	return conversion, nil
}

func generateTelemetryConfigs(stage string, sourceRefs []string) error {
	commentLines := []string{"// Source resources linked to this candidate:"}
	yamlComments := []string{"# Source resources linked to this candidate:"}
	if len(sourceRefs) == 0 {
		commentLines = append(commentLines, "// - target baseline (no Datadog telemetry configuration resources were present)")
		yamlComments = append(yamlComments, "# - target baseline (no Datadog telemetry configuration resources were present)")
	} else {
		for _, ref := range sourceRefs {
			commentLines = append(commentLines, "// - "+ref)
			yamlComments = append(yamlComments, "# - "+ref)
		}
	}
	alloy := strings.Join(commentLines, "\n") + `
// OpenExit candidate only. Review topology, endpoints, TLS, authentication, sizing, and processors.
otelcol.receiver.otlp "openexit" {
  grpc { }
  http { }
  output {
    metrics = [otelcol.processor.batch.openexit.input]
    logs    = [otelcol.processor.batch.openexit.input]
    traces  = [otelcol.processor.batch.openexit.input]
  }
}

otelcol.processor.batch "openexit" {
  output {
    metrics = [otelcol.exporter.otlphttp.lgtm.input]
    logs    = [otelcol.exporter.otlphttp.lgtm.input]
    traces  = [otelcol.exporter.otlphttp.lgtm.input]
  }
}

otelcol.exporter.otlphttp "lgtm" {
  client {
    endpoint = sys.env("OPENEXIT_OTLP_ENDPOINT")
  }
}
`
	otel := strings.Join(yamlComments, "\n") + `
# OpenExit candidate only. Review topology, endpoints, TLS, authentication, sizing, and processors.
receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  batch: {}
exporters:
  otlphttp/lgtm:
    endpoint: "${env:OPENEXIT_OTLP_ENDPOINT}"
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/lgtm]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/lgtm]
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/lgtm]
`
	if err := WriteText(filepath.Join(stage, "generated", "alloy", "config.alloy"), alloy); err != nil {
		return err
	}
	return WriteText(filepath.Join(stage, "generated", "opentelemetry", "collector.yaml"), otel)
}

func dashboardWidgets(spec map[string]any) []map[string]any {
	var widgets []map[string]any
	if raw, ok := spec["widgets"].([]any); ok {
		for _, value := range raw {
			widget, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if definition, ok := widget["definition"].(map[string]any); ok {
				copy := make(map[string]any, len(definition)+1)
				for key, value := range definition {
					copy[key] = value
				}
				if title := stringValue(widget["title"]); title != "" && stringValue(copy["title"]) == "" {
					copy["title"] = title
				}
				widget = copy
			}
			widgets = append(widgets, widget)
		}
	}
	if queries, ok := spec["queries"].([]any); ok {
		for index, value := range queries {
			if query := stringValue(value); query != "" {
				widgets = append(widgets, map[string]any{"type": "timeseries", "title": fmt.Sprintf("Dashboard query %d", index+1), "query": query})
			}
		}
	}
	return widgets
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := stringValue(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func specString(spec map[string]any, keys ...string) string {
	return stringValue(nestedValue(spec, keys...))
}

func isTextWidget(kind string) bool {
	switch kind {
	case "note", "free_text", "text", "markdown":
		return true
	default:
		return false
	}
}

func grafanaPanelType(kind string) string {
	switch kind {
	case "query_value", "queryvalue", "value":
		return "stat"
	case "toplist", "table":
		return "table"
	case "heatmap":
		return "heatmap"
	default:
		return "timeseries"
	}
}

func configSourceRefs(resources []Resource) []string {
	allowed := map[string]bool{
		"integration": true, "aws_integration": true, "azure_integration": true, "gcp_integration": true, "gcp_legacy_integration": true,
		"log_pipeline": true, "log_pipeline_order": true, "log_index": true, "log_archive": true, "log_metric": true,
		"apm_retention_filter": true, "span_metric": true, "service_definition": true,
	}
	var refs []string
	for _, resource := range resources {
		if allowed[resource.Kind] {
			refs = append(refs, resource.Ref)
		}
	}
	return SortedUnique(refs)
}

func sortOutputs(outputs []OutputRef) []OutputRef {
	sort.Slice(outputs, func(i, j int) bool {
		if outputs[i].Path == outputs[j].Path {
			return outputs[i].Kind < outputs[j].Kind
		}
		return outputs[i].Path < outputs[j].Path
	})
	return outputs
}
