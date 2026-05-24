package assessment

import (
	"context"
	"fmt"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type CostDriverAnalyzer struct{}

func (CostDriverAnalyzer) Name() string { return "cost-drivers" }

func (CostDriverAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	if inv.Summary.UniqueMetrics > 250 {
		findings = append(findings, Finding{
			ID:             "dd.cost.many-custom-metrics.001",
			Severity:       "medium",
			Title:          "Many referenced metrics may affect target sizing",
			Description:    fmt.Sprintf("%d unique metric names were referenced.", inv.Summary.UniqueMetrics),
			AffectedAssets: []string{"inventory:metrics"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Estimate remote-write volume, retention, and cardinality before provisioning the target stack.",
		})
	}
	if inv.Summary.Dashboards > 0 || inv.Summary.Monitors > 0 {
		findings = append(findings, Finding{
			ID:             "dd.cost.retention-unknown.001",
			Severity:       "low",
			Title:          "Retention requirements are not captured",
			Description:    "Datadog inventory does not fully describe retention expectations for metrics, logs, or traces.",
			AffectedAssets: []string{"inventory:retention"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Document retention and volume assumptions before sizing Grafana LGTM components.",
		})
	}
	if !inv.Volumes.LogVolumeKnown && (hasLogSignals(inv) || inv.Summary.Dashboards > 0 || inv.Summary.Monitors > 0) {
		findings = append(findings, Finding{
			ID:             "dd.cost.log-volume-unknown.001",
			Severity:       "low",
			Title:          "Log volume is unknown",
			Description:    "The inventory does not include log volume assumptions needed for Loki sizing.",
			AffectedAssets: []string{"inventory:logs"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Capture daily ingest, retention, label strategy, and query expectations before target sizing.",
		})
	}
	if !inv.Volumes.TraceVolumeKnown && (hasTraceSignals(inv) || inv.Summary.Dashboards > 0 || inv.Summary.Monitors > 0) {
		findings = append(findings, Finding{
			ID:             "dd.cost.trace-volume-unknown.001",
			Severity:       "low",
			Title:          "Trace volume is unknown",
			Description:    "The inventory does not include trace volume assumptions needed for Tempo sizing.",
			AffectedAssets: []string{"inventory:traces"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Capture spans per second, retention, sampling policy, and search expectations before target sizing.",
		})
	}
	return findings, nil
}

func firstEvidenceRefs(inv *inventory.Inventory) []string {
	var refs []string
	for _, dashboard := range inv.Assets.Dashboards {
		if dashboard.EvidenceRef != "" {
			refs = append(refs, dashboard.EvidenceRef)
		}
		if len(refs) >= 3 {
			return refs
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if monitor.EvidenceRef != "" {
			refs = append(refs, monitor.EvidenceRef)
		}
		if len(refs) >= 3 {
			return refs
		}
	}
	for _, slo := range inv.Assets.SLOs {
		if slo.EvidenceRef != "" {
			refs = append(refs, slo.EvidenceRef)
		}
		if len(refs) >= 3 {
			return refs
		}
	}
	if len(refs) == 0 {
		return []string{"evidence://inventory/summary"}
	}
	return refs
}

func highCardinalityQuery(query string) bool {
	query = strings.ToLower(query)
	needles := []string{"{pod}", "{container_id}", "{trace_id}", "{user_id}", "{session_id}", " by {pod", " by {container_id", " by {user_id", " by {trace_id"}
	for _, needle := range needles {
		if strings.Contains(query, needle) {
			return true
		}
	}
	return false
}

func hasLogSignals(inv *inventory.Inventory) bool {
	return inventoryContains(inv, "logs", "log.")
}

func hasTraceSignals(inv *inventory.Inventory) bool {
	return inventoryContains(inv, "trace", "apm")
}

func inventoryContains(inv *inventory.Inventory, needles ...string) bool {
	for _, dashboard := range inv.Assets.Dashboards {
		for _, query := range dashboard.Queries {
			if containsAnyLower(query.Raw, needles...) {
				return true
			}
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if containsAnyLower(monitor.Query, needles...) {
			return true
		}
	}
	for _, metric := range inv.Assets.Metrics {
		if containsAnyLower(metric.Name, needles...) {
			return true
		}
	}
	return false
}

func containsAnyLower(value string, needles ...string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
