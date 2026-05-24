package assessment

import (
	"context"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type OwnershipAnalyzer struct{}

func (OwnershipAnalyzer) Name() string { return "ownership" }

func (OwnershipAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, dashboard := range inv.Assets.Dashboards {
		if !inventory.HasOwnerTag(dashboard.Tags) {
			findings = append(findings, Finding{
				ID:             "dd.ownership.missing.001",
				Severity:       "medium",
				Title:          "Dashboard ownership is unclear",
				Description:    "No team, owner, or service tag was found for this dashboard.",
				AffectedAssets: []string{"dashboard:" + dashboard.ID},
				EvidenceRefs:   []string{dashboard.EvidenceRef},
				Recommendation: "Assign an owner before dashboard parity review and cutover planning.",
			})
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if !inventory.HasOwnerTag(monitor.Tags) {
			findings = append(findings, Finding{
				ID:             "dd.monitor.missing-owner.001",
				Severity:       "medium",
				Title:          "Monitor ownership is unclear",
				Description:    "No team, owner, or service tag was found for this monitor.",
				AffectedAssets: []string{"monitor:" + monitor.ID},
				EvidenceRefs:   []string{monitor.EvidenceRef},
				Recommendation: "Assign an alert owner before migrating routing or paging behavior.",
			})
		}
	}
	return findings, nil
}
