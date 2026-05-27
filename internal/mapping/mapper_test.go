package mapping

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestBuildDatadogMapping(t *testing.T) {
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0).UTC())
	inv.Assets.Dashboards = []inventory.Dashboard{
		{ID: "dash-1", Title: "Production API", Widgets: inventory.WidgetSummary{Total: 2, Unsupported: 1}, Queries: []inventory.Query{{ID: "q1", Raw: "avg:system.cpu.user{env:prod}"}}},
	}
	inv.Assets.Monitors = []inventory.Monitor{
		{ID: "mon-1", Name: "High error rate", Query: "sum(last_5m):avg:trace.http.request.errors{env:prod} > 10"},
	}
	inv.Assets.SLOs = []inventory.SLO{{ID: "slo-1", Name: "API availability"}}
	a := assessment.New("demo", "datadog", "grafana-lgtm", time.Unix(0, 0).UTC(), []assessment.Finding{
		{ID: "dd.monitor.query.translation.001", Severity: "high", Title: "Datadog-specific monitor query requires manual conversion", AffectedAssets: []string{"monitor:mon-1"}, EvidenceRefs: []string{"evidence://datadog/monitor/mon-1"}, Recommendation: "Review PromQL."},
	}, assessment.Score{Level: "medium"})

	result, err := Build(inv, a, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("build mapping: %v", err)
	}
	if result.Kind != Kind || result.TargetType != "grafana-lgtm" {
		t.Fatalf("unexpected mapping metadata: %+v", result)
	}
	if len(result.DashboardDrafts) != 1 || result.DashboardDrafts[0].Level != 1 {
		t.Fatalf("unexpected dashboard drafts: %+v", result.DashboardDrafts)
	}
	if len(result.AlertRuleDrafts) != 1 || result.AlertRuleDrafts[0].Alert != "HighErrorRateCandidate" {
		t.Fatalf("unexpected alert drafts: %+v", result.AlertRuleDrafts)
	}
	if len(result.UnsupportedItems) != 2 {
		t.Fatalf("expected dashboard and SLO unsupported mapping items: %+v", result.UnsupportedItems)
	}
	if len(result.ManualReview) == 0 {
		t.Fatalf("expected manual review mapping items")
	}
}

func TestWriteMappingArtifacts(t *testing.T) {
	result := &MappingResult{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata:   Metadata{Project: "demo", GeneratedAt: time.Unix(0, 0).UTC()},
		Source:     Endpoint{Type: "datadog"},
		Target:     Endpoint{Type: "grafana-lgtm"},
		TargetType: "grafana-lgtm",
	}
	projectDir := t.TempDir()
	if err := Write(projectDir, result); err != nil {
		t.Fatalf("write mapping: %v", err)
	}
	for _, rel := range []string{
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}
