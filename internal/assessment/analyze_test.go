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
