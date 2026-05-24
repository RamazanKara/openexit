package assessment

import (
	"context"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type CardinalityAnalyzer struct{}

func (CardinalityAnalyzer) Name() string { return "cardinality" }

func (CardinalityAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, dashboard := range inv.Assets.Dashboards {
		for _, query := range dashboard.Queries {
			if highCardinalityQuery(query.Raw) {
				findings = append(findings, Finding{
					ID:             "dd.cost.high-cardinality.001",
					Severity:       "medium",
					Title:          "High-cardinality query detected",
					Description:    "The query groups or filters by a tag that may create high series cardinality in Prometheus-compatible storage.",
					AffectedAssets: []string{"dashboard:" + dashboard.ID},
					EvidenceRefs:   []string{dashboard.EvidenceRef},
					Recommendation: "Review label strategy and remote-write cost before migration.",
				})
				break
			}
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if highCardinalityQuery(monitor.Query) {
			findings = append(findings, Finding{
				ID:             "dd.cost.high-cardinality.001",
				Severity:       "medium",
				Title:          "High-cardinality query detected",
				Description:    "The monitor query groups or filters by a tag that may create high series cardinality in Prometheus-compatible storage.",
				AffectedAssets: []string{"monitor:" + monitor.ID},
				EvidenceRefs:   []string{monitor.EvidenceRef},
				Recommendation: "Review label strategy and alert cardinality before migration.",
			})
		}
	}
	return findings, nil
}
