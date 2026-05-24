package assessment

import (
	"context"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type MigrationAnalyzer struct{}

func (MigrationAnalyzer) Name() string { return "migration-risks" }

func (MigrationAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	if len(inv.Assets.Monitors) > 0 {
		findings = append(findings, Finding{
			ID:             "migration.alert-shadowing-required.001",
			Severity:       "medium",
			Title:          "Alert shadowing is required",
			Description:    "Source monitors exist, so target alert candidates should run in shadow mode before cutover.",
			AffectedAssets: []string{"inventory:monitors"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Run alert candidates without production paging and compare behavior against Datadog over a representative period.",
		})
	}
	if len(inv.Assets.Dashboards) > 0 {
		findings = append(findings, Finding{
			ID:             "migration.dashboard-parity-required.001",
			Severity:       "medium",
			Title:          "Dashboard parity review is required",
			Description:    "Source dashboards exist, so generated Grafana candidates need owner review before migration sign-off.",
			AffectedAssets: []string{"inventory:dashboards"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Review candidate dashboards with service owners and record acceptable differences.",
		})
	}
	if hasManualQueryReview(inv) {
		findings = append(findings, Finding{
			ID:             "migration.manual-query-review-required.001",
			Severity:       "high",
			Title:          "Manual query review is required",
			Description:    "At least one source monitor query uses Datadog-specific syntax that should not be converted blindly.",
			AffectedAssets: []string{"inventory:queries"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Review query candidates with SRE owners and shadow target alert behavior before cutover.",
		})
	}
	if inv.Summary.NotificationTargets > 0 {
		findings = append(findings, Finding{
			ID:             "migration.notification-routing-review-required.001",
			Severity:       "medium",
			Title:          "Notification routing review is required",
			Description:    "Datadog notification handles need explicit target contact point and routing policy mapping.",
			AffectedAssets: []string{"inventory:notification-targets"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Create target routing policies and test them before any production paging change.",
		})
	}
	if hasUnclearOwnership(inv) {
		findings = append(findings, Finding{
			ID:             "migration.oncall-ownership-unclear.001",
			Severity:       "medium",
			Title:          "On-call ownership is unclear",
			Description:    "At least one dashboard or monitor lacks a team, owner, or service tag.",
			AffectedAssets: []string{"inventory:ownership"},
			EvidenceRefs:   firstEvidenceRefs(inv),
			Recommendation: "Assign owners before alert routing, dashboard parity review, and cutover planning.",
		})
	}
	return findings, nil
}

func hasManualQueryReview(inv *inventory.Inventory) bool {
	for _, monitor := range inv.Assets.Monitors {
		query := monitor.Query
		if hasDatadogSpecificQuery(query) || highCardinalityQuery(query) {
			return true
		}
	}
	return false
}

func hasUnclearOwnership(inv *inventory.Inventory) bool {
	for _, dashboard := range inv.Assets.Dashboards {
		if !inventory.HasOwnerTag(dashboard.Tags) {
			return true
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if !inventory.HasOwnerTag(monitor.Tags) {
			return true
		}
	}
	return false
}
