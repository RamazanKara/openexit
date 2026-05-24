package assessment

import (
	"context"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type SLOAnalyzer struct{}

func (SLOAnalyzer) Name() string { return "slo-risks" }

func (SLOAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, slo := range inv.Assets.SLOs {
		findings = append(findings, Finding{
			ID:             "dd.slo.target-needs-review.001",
			Severity:       "medium",
			Title:          "SLO target needs migration review",
			Description:    "SLO migration requires mapping the SLI source, burn-rate alerts, dashboards, and ownership.",
			AffectedAssets: []string{"slo:" + slo.ID},
			EvidenceRefs:   []string{slo.EvidenceRef},
			Recommendation: "Review SLO semantics and create matching dashboards and burn-rate alerts before cutover.",
		})
		if slo.SLI == "" {
			findings = append(findings, Finding{
				ID:             "dd.slo.without-clear-sli.001",
				Severity:       "medium",
				Title:          "SLO has no clear SLI mapping",
				Description:    "The inventory does not include a clear service level indicator query for this SLO.",
				AffectedAssets: []string{"slo:" + slo.ID},
				EvidenceRefs:   []string{slo.EvidenceRef},
				Recommendation: "Identify the target SLI query and validate it against the source SLO before migration.",
			})
		}
		if len(slo.BurnRateMonitorIDs) == 0 {
			findings = append(findings, Finding{
				ID:             "dd.slo.burn-rate-alert-missing.001",
				Severity:       "medium",
				Title:          "SLO burn-rate alert mapping is missing",
				Description:    "No burn-rate monitor mapping was captured for this SLO.",
				AffectedAssets: []string{"slo:" + slo.ID},
				EvidenceRefs:   []string{slo.EvidenceRef},
				Recommendation: "Create and shadow target burn-rate alerts before SLO cutover.",
			})
		}
		if len(slo.DashboardRefs) == 0 {
			findings = append(findings, Finding{
				ID:             "dd.slo.dashboard-missing.001",
				Severity:       "low",
				Title:          "SLO dashboard mapping is missing",
				Description:    "No dashboard reference was captured for this SLO.",
				AffectedAssets: []string{"slo:" + slo.ID},
				EvidenceRefs:   []string{slo.EvidenceRef},
				Recommendation: "Add or confirm target SLO dashboard coverage for migration readiness.",
			})
		}
	}
	return findings, nil
}
