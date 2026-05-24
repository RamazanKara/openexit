package assessment

import (
	"context"
	"fmt"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type DashboardAnalyzer struct{}

func (DashboardAnalyzer) Name() string { return "dashboard-risks" }

func (DashboardAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, dashboard := range inv.Assets.Dashboards {
		if dashboard.Widgets.Unsupported > 0 {
			findings = append(findings, Finding{
				ID:             "dd.dashboard.unsupported-widget.001",
				Severity:       "medium",
				Title:          "Unsupported dashboard widgets found",
				Description:    fmt.Sprintf("%s has %d widget(s) that need manual Grafana review.", dashboard.Title, dashboard.Widgets.Unsupported),
				AffectedAssets: []string{"dashboard:" + dashboard.ID},
				EvidenceRefs:   []string{dashboard.EvidenceRef},
				Recommendation: "Generate a Grafana dashboard candidate and manually review unsupported widgets before parity sign-off.",
			})
		}
		if dashboard.Widgets.Total > 25 {
			findings = append(findings, Finding{
				ID:             "dd.dashboard.too-many-panels.001",
				Severity:       "medium",
				Title:          "Large dashboard requires migration planning",
				Description:    fmt.Sprintf("%s contains %d widgets.", dashboard.Title, dashboard.Widgets.Total),
				AffectedAssets: []string{"dashboard:" + dashboard.ID},
				EvidenceRefs:   []string{dashboard.EvidenceRef},
				Recommendation: "Review layout and ownership before importing a large generated dashboard candidate.",
			})
		}
		for _, source := range dashboard.DataSources {
			if unknownDashboardDataSource(source) {
				findings = append(findings, Finding{
					ID:             "dd.dashboard.unknown-data-source.001",
					Severity:       "medium",
					Title:          "Dashboard data source requires target mapping",
					Description:    fmt.Sprintf("%s references data source %q, which OpenExit cannot map automatically.", dashboard.Title, source),
					AffectedAssets: []string{"dashboard:" + dashboard.ID},
					EvidenceRefs:   []string{dashboard.EvidenceRef},
					Recommendation: "Map the data source to Grafana LGTM manually and validate dashboard panel behavior.",
				})
				break
			}
		}
		if complexTemplateVariables(dashboard.TemplateVariables) {
			findings = append(findings, Finding{
				ID:             "dd.dashboard.template-variable-complexity.001",
				Severity:       "medium",
				Title:          "Dashboard template variables require review",
				Description:    fmt.Sprintf("%s uses template variables that may need manual Grafana variable mapping.", dashboard.Title),
				AffectedAssets: []string{"dashboard:" + dashboard.ID},
				EvidenceRefs:   []string{dashboard.EvidenceRef},
				Recommendation: "Review template variable queries, defaults, and refresh behavior before dashboard import.",
			})
		}
	}
	return findings, nil
}

type MonitorAnalyzer struct{}

func (MonitorAnalyzer) Name() string { return "monitor-risks" }

func (MonitorAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, monitor := range inv.Assets.Monitors {
		query := strings.ToLower(monitor.Query)
		switch {
		case strings.Contains(query, "anomal"):
			findings = append(findings, monitorFinding(monitor, "dd.monitor.anomaly-detection.001", "high", "Anomaly detection monitor needs manual replacement", "Datadog anomaly detection has no automatic PromQL equivalent.", "Design an explicit Prometheus/Grafana alerting strategy and shadow it before cutover."))
		case strings.Contains(query, "outlier"):
			findings = append(findings, monitorFinding(monitor, "dd.monitor.outlier-detection.001", "high", "Outlier detection monitor needs manual replacement", "Datadog outlier detection has no automatic PromQL equivalent.", "Create a reviewed alert candidate and validate it against historical incidents."))
		case strings.Contains(query, "forecast"):
			findings = append(findings, monitorFinding(monitor, "dd.monitor.forecast-function.001", "high", "Forecast monitor needs manual replacement", "Datadog forecast functions require human review.", "Replace with an explicit SLO or capacity signal before migration."))
		case strings.Contains(strings.ToLower(monitor.Type), "composite"):
			findings = append(findings, monitorFinding(monitor, "dd.monitor.composite.001", "high", "Composite monitor requires manual alert design", "Composite monitors depend on other monitors and should not be converted blindly.", "Rebuild the alert logic explicitly in the target platform."))
		case hasDatadogSpecificQuery(query):
			findings = append(findings, monitorFinding(monitor, "dd.monitor.query.translation.001", "high", "Datadog-specific monitor query requires manual conversion", "The monitor query uses Datadog syntax or functions that require PromQL review.", "Create a PromQL candidate and require SRE review before alert shadowing."))
		}
		if monitor.RunbookURL == "" {
			findings = append(findings, monitorFinding(monitor, "dd.monitor.missing-runbook.001", "low", "Monitor has no runbook link", "No runbook URL was found for this alert.", "Add or confirm operational guidance before alert cutover."))
		}
		for _, target := range monitor.NotificationTargets {
			if unknownNotificationTarget(target) {
				findings = append(findings, monitorFinding(monitor, "dd.monitor.unknown-notification-target.001", "medium", "Notification target requires routing review", fmt.Sprintf("Notification target %q is not recognized as a safe automatic route.", target), "Map notification routing manually and test paging behavior during shadowing."))
				findings = append(findings, monitorFinding(monitor, "dd.monitor.manual-routing-needed.001", "medium", "Manual alert routing review is needed", "OpenExit does not automatically map Datadog notification handles to target alert routes.", "Create explicit target contact points and routing policies, then test them without production paging."))
				break
			}
		}
	}
	return findings, nil
}

func unknownDashboardDataSource(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return false
	}
	known := []string{"datadog", "metrics", "logs", "traces", "rum", "apm", "synthetics"}
	for _, value := range known {
		if source == value || strings.Contains(source, value) {
			return false
		}
	}
	return true
}

func complexTemplateVariables(variables []inventory.TemplateVariable) bool {
	if len(variables) > 5 {
		return true
	}
	for _, variable := range variables {
		query := strings.ToLower(variable.Query)
		if strings.Contains(query, "*") || strings.Contains(query, " by ") || strings.Contains(query, "rollup(") || strings.Contains(query, "tags(") || strings.Contains(query, "hosts(") {
			return true
		}
	}
	return false
}

func monitorFinding(m inventory.Monitor, id, severity, title, description, recommendation string) Finding {
	return Finding{
		ID:             id,
		Severity:       severity,
		Title:          title,
		Description:    description,
		AffectedAssets: []string{"monitor:" + m.ID},
		EvidenceRefs:   []string{m.EvidenceRef},
		Recommendation: recommendation,
	}
}

func hasDatadogSpecificQuery(query string) bool {
	needles := []string{"rollup(", ".rollup(", "as_count()", "as_rate()", "last_", "change(", "pct_change", "timeshift(", "default_zero(", "exclude_null("}
	for _, needle := range needles {
		if strings.Contains(query, needle) {
			return true
		}
	}
	return false
}

func unknownNotificationTarget(target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	knownPrefixes := []string{"slack", "@slack", "pagerduty", "@pagerduty", "email", "mailto:", "@team", "@opsgenie"}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(target, prefix) {
			return false
		}
	}
	return true
}
