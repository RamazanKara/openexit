package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

func TestConvertDatadogMonitorQuerySimpleThreshold(t *testing.T) {
	conversion := convertDatadogMonitorQuery("sum(last_5m):sum:trace.http.request.errors{env:prod,service:api}.as_count() > 10")
	if conversion.Level != "simple-threshold" {
		t.Fatalf("expected simple conversion, got %s", conversion.Level)
	}
	if !strings.Contains(conversion.Expr, `sum(increase(trace_http_request_errors{env="prod",service="api"}[5m])) > 10`) {
		t.Fatalf("unexpected expression: %s", conversion.Expr)
	}
	if !strings.Contains(conversion.Review, "Review metric naming") {
		t.Fatalf("review warning missing: %s", conversion.Review)
	}
}

func TestConvertDatadogMonitorQueryComplexPlaceholder(t *testing.T) {
	conversion := convertDatadogMonitorQuery("avg(last_15m):anomalies(avg:trace.http.request.duration{env:prod}, 'basic', 2) >= 1")
	if conversion.Level != "manual-review" {
		t.Fatalf("expected manual-review conversion, got %s", conversion.Level)
	}
	if !strings.Contains(conversion.Expr, "vector(0)") {
		t.Fatalf("expected placeholder vector: %s", conversion.Expr)
	}
}

func TestPrometheusGenerationYAML(t *testing.T) {
	projectDir := t.TempDir()
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	inv.Assets.Monitors = []inventory.Monitor{{
		ID:          "m-1",
		Name:        "High error rate",
		Type:        "query alert",
		Query:       "sum(last_5m):sum:trace.http.request.errors{env:prod}.as_count() > 10",
		EvidenceRef: "evidence://datadog/monitor/m-1",
	}}
	ctx := &Context{
		ProjectDir: projectDir,
		Inventory:  inv,
		Assessment: assessment.New("demo", "datadog", "grafana-lgtm", time.Unix(0, 0), nil, assessment.Score{}),
	}
	if err := Prometheus(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, "generated-config", "prometheus", "rules", "datadog-monitor-candidates.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed ruleFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Groups) != 1 || len(parsed.Groups[0].Rules) != 1 {
		t.Fatalf("unexpected rule file: %+v", parsed)
	}
	if parsed.Groups[0].Rules[0].Labels["conversion"] != "simple-threshold" {
		t.Fatalf("conversion label missing: %+v", parsed.Groups[0].Rules[0].Labels)
	}
}
