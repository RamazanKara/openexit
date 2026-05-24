package assessment

import (
	"context"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestAnalyzersProduceDeterministicFindings(t *testing.T) {
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	inv.Assets.Dashboards = []inventory.Dashboard{{
		ID:    "dash",
		Title: "Dashboard",
		Tags:  []string{"env:prod"},
		Widgets: inventory.WidgetSummary{
			Total:       2,
			Unsupported: 1,
		},
		Queries:     []inventory.Query{{ID: "q1", Language: "datadog", Raw: "avg:api.duration{env:prod} by {user_id}"}},
		EvidenceRef: "evidence://datadog/dashboard/dash",
	}}
	inv.Assets.Monitors = []inventory.Monitor{{
		ID:                  "mon",
		Name:                "Latency anomaly",
		Type:                "query alert",
		Query:               "avg(last_5m):anomalies(avg:api.duration{env:prod}, 'basic', 2) >= 1",
		NotificationTargets: []string{"webhook-custom"},
		Tags:                []string{"env:prod"},
		EvidenceRef:         "evidence://datadog/monitor/mon",
	}}
	inv.RecomputeSummary()
	result, err := Run(context.Background(), inv, "grafana-lgtm", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(result); err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) < 5 {
		t.Fatalf("expected several findings, got %d", len(result.Findings))
	}
	for i := 1; i < len(result.Findings); i++ {
		if result.Findings[i-1].ID > result.Findings[i].ID {
			t.Fatalf("findings not sorted: %s before %s", result.Findings[i-1].ID, result.Findings[i].ID)
		}
	}
}

func TestRiskRuleCoverage(t *testing.T) {
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	inv.Assets.Dashboards = []inventory.Dashboard{{
		ID:          "dash-risk",
		Title:       "Risk Dashboard",
		Tags:        []string{"env:prod"},
		DataSources: []string{"custom-events-api"},
		Widgets:     inventory.WidgetSummary{Total: 1},
		Queries: []inventory.Query{{
			ID:       "q1",
			Language: "datadog",
			Raw:      "avg:api.request.duration{env:prod} by {user_id}",
		}},
		TemplateVariables: []inventory.TemplateVariable{{
			Name:  "pod",
			Query: "tags(kube.pod.name{env:prod} by {pod})",
		}},
		EvidenceRef: "evidence://datadog/dashboard/dash-risk",
	}}
	inv.Assets.Monitors = []inventory.Monitor{{
		ID:                  "mon-risk",
		Name:                "Risk monitor",
		Type:                "query alert",
		Query:               "sum(last_5m):sum:trace.http.request.errors{env:prod}.as_count() > 10",
		NotificationTargets: []string{"unknown-webhook"},
		Tags:                []string{"env:prod"},
		EvidenceRef:         "evidence://datadog/monitor/mon-risk",
	}}
	inv.Assets.SLOs = []inventory.SLO{{
		ID:          "slo-risk",
		Name:        "Checkout availability",
		Target:      99.9,
		Timeframe:   "30d",
		EvidenceRef: "evidence://datadog/slo/slo-risk",
	}}
	inv.Assets.Metrics = []inventory.MetricRef{{Name: "logs.ingested.bytes"}, {Name: "trace.http.request.errors"}}
	inv.RecomputeSummary()
	result, err := Run(context.Background(), inv, "grafana-lgtm", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	expectedIDs := []string{
		"dd.dashboard.unknown-data-source.001",
		"dd.dashboard.template-variable-complexity.001",
		"dd.monitor.manual-routing-needed.001",
		"dd.slo.without-clear-sli.001",
		"dd.slo.burn-rate-alert-missing.001",
		"dd.slo.dashboard-missing.001",
		"dd.cost.log-volume-unknown.001",
		"dd.cost.trace-volume-unknown.001",
		"migration.alert-shadowing-required.001",
		"migration.dashboard-parity-required.001",
		"migration.manual-query-review-required.001",
		"migration.notification-routing-review-required.001",
		"migration.oncall-ownership-unclear.001",
	}
	for _, id := range expectedIDs {
		if !hasFindingID(result.Findings, id) {
			t.Fatalf("expected finding %s in %#v", id, result.Findings)
		}
	}
	if result.Score.Value <= 70 || result.Score.Level != "high" {
		t.Fatalf("expected high complexity score, got %+v", result.Score)
	}
}

func hasFindingID(findings []Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
