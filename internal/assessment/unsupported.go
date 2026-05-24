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
	}
	return findings, nil
}
