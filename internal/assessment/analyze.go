package assessment

import (
	"context"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/inventory"
)

func Run(ctx context.Context, inv *inventory.Inventory, target string, now time.Time) (*Assessment, error) {
	analyzers := []Analyzer{
		DashboardAnalyzer{},
		MonitorAnalyzer{},
		OwnershipAnalyzer{},
		CardinalityAnalyzer{},
		SLOAnalyzer{},
		CostDriverAnalyzer{},
		MigrationAnalyzer{},
		GitHubEnterpriseAnalyzer{},
	}
	var findings []Finding
	for _, analyzer := range analyzers {
		next, err := analyzer.Analyze(ctx, inv)
		if err != nil {
			return nil, err
		}
		findings = append(findings, next...)
	}
	SortFindings(findings)
	score := ScoreInventory(inv, findings)
	return New(inv.Metadata.Project, inv.Source.Type, target, now, findings, score), nil
}

func severityCount(findings []Finding, severity string) int {
	count := 0
	for _, finding := range findings {
		if strings.EqualFold(finding.Severity, severity) {
			count++
		}
	}
	return count
}
