package datadog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/evidence"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestLiveCollectorNormalizationWithMockedAPI(t *testing.T) {
	projectDir := t.TempDir()
	var sawAuthHeaders bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") == "api" && r.Header.Get("DD-APPLICATION-KEY") == "app" {
			sawAuthHeaders = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/dashboard":
			_, _ = w.Write([]byte(`{"dashboards":[{"id":"dash-1","title":"Live API","url":"https://app.datadoghq.eu/dashboard/dash-1"}]}`))
		case "/api/v1/dashboard/dash-1":
			_, _ = w.Write([]byte(`{
				"title":"Live API",
				"tags":["team:platform"],
				"widgets":[
					{"definition":{"type":"timeseries","requests":[{"queries":[{"query":"avg:system.cpu.user{env:prod} by {service}"}]}]}},
					{"definition":{"type":"note","content":"token=supersecretvalue"}}
				]
			}`))
		case "/api/v1/monitor":
			_, _ = w.Write([]byte(`[{
				"id":123456,
				"name":"Live high error rate",
				"type":"query alert",
				"query":"sum(last_5m):sum:trace.http.request.errors{env:prod}.as_count() > 10",
				"message":"Investigate @slack-platform token=supersecretvalue",
				"tags":["team:platform"]
			}]`))
		case "/api/v1/slo":
			_, _ = w.Write([]byte(`{"data":[{"id":"slo-1","attributes":{"name":"Availability","target_threshold":99.9,"timeframe":"30d"}}]}`))
		case "/api/v2/integrations":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"kubernetes","type":"integration","attributes":{"title":"Kubernetes","categories":["containers","orchestration"],"installed":true}},
				{"id":"pagerduty","type":"integration","attributes":{"title":"PagerDuty","categories":["alerting"],"installed":false}}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, apiKey: "api", appKey: "app", http: server.Client()}
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	if err := collectDashboards(context.Background(), client, projectDir, inv); err != nil {
		t.Fatal(err)
	}
	if err := collectMonitors(context.Background(), client, projectDir, inv); err != nil {
		t.Fatal(err)
	}
	if err := collectSLOs(context.Background(), client, projectDir, inv); err != nil {
		t.Fatal(err)
	}
	if err := collectIntegrations(context.Background(), client, projectDir, inv); err != nil {
		t.Fatal(err)
	}
	inv.RecomputeSummary()
	if err := inventory.Validate(inv); err != nil {
		t.Fatal(err)
	}
	if !sawAuthHeaders {
		t.Fatal("mock server did not receive Datadog auth headers")
	}
	if inv.Summary.Dashboards != 1 || inv.Summary.Monitors != 1 || inv.Summary.SLOs != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	if inv.Summary.Integrations != 2 {
		t.Fatalf("expected live integrations in summary, got %+v", inv.Summary)
	}
	if inv.Summary.UniqueMetrics != 2 {
		t.Fatalf("expected live queries to populate metric summary, got %+v", inv.Summary)
	}
	if integration := integrationByName(inv.Assets.Integrations, "Kubernetes"); integration == nil || !integration.Enabled || !hasAll(integration.Tags, "containers", "orchestration") {
		t.Fatalf("expected installed Kubernetes integration metadata, got %+v", inv.Assets.Integrations)
	}
	if integration := integrationByName(inv.Assets.Integrations, "PagerDuty"); integration == nil || integration.Enabled {
		t.Fatalf("expected disabled PagerDuty integration metadata, got %+v", inv.Assets.Integrations)
	}
	if metric := metricByName(inv.Assets.Metrics, "system.cpu.user"); metric == nil || !hasAll(metric.Tags, "env", "service") {
		t.Fatalf("expected dashboard metric tags to be captured, got %+v", inv.Assets.Metrics)
	}
	if metric := metricByName(inv.Assets.Metrics, "trace.http.request.errors"); metric == nil || !hasAll(metric.Tags, "env") {
		t.Fatalf("expected monitor metric tags to be captured, got %+v", inv.Assets.Metrics)
	}
	for _, ref := range []string{
		inv.Assets.Dashboards[0].EvidenceRef,
		inv.Assets.Monitors[0].EvidenceRef,
		inv.Assets.SLOs[0].EvidenceRef,
		inv.Assets.Integrations[0].EvidenceRef,
	} {
		path, err := evidence.PathForRef(projectDir, ref)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("evidence ref %s did not resolve: %v", ref, err)
		}
	}
	monitorEvidence := filepath.Join(projectDir, "evidence", "datadog", "monitors", "123456.json")
	data, err := os.ReadFile(monitorEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.ContainsSecret(string(data)) {
		t.Fatalf("monitor evidence was not redacted: %s", string(data))
	}
}

func TestCollectIntegrationsReturnsWarningFriendlyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/integrations" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":["missing integrations_read"]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, apiKey: "api", appKey: "app", http: server.Client()}
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0))
	err := collectIntegrations(context.Background(), client, t.TempDir(), inv)
	if err == nil || !strings.Contains(err.Error(), "collect integrations") {
		t.Fatalf("expected contextual integration collection error, got %v", err)
	}
	if len(inv.Assets.Integrations) != 0 {
		t.Fatalf("expected no integration assets on failed collection, got %+v", inv.Assets.Integrations)
	}
}

func TestDatadogBaseURLValidation(t *testing.T) {
	for _, site := range []string{"datadoghq.com", "datadoghq.eu", "us3.datadoghq.com", "us5.datadoghq.com", "ap1.datadoghq.com", "ap2.datadoghq.com"} {
		if _, err := datadogBaseURL(site); err != nil {
			t.Fatalf("expected site %s to be accepted: %v", site, err)
		}
	}
	if _, err := datadogBaseURL("http://api.datadoghq.com"); err == nil {
		t.Fatal("expected http URL to be rejected")
	}
	if _, err := datadogBaseURL("example.com"); err == nil {
		t.Fatal("expected unknown site to be rejected")
	}
}

func metricByName(metrics []inventory.MetricRef, name string) *inventory.MetricRef {
	for i := range metrics {
		if metrics[i].Name == name {
			return &metrics[i]
		}
	}
	return nil
}

func integrationByName(integrations []inventory.Integration, name string) *inventory.Integration {
	for i := range integrations {
		if integrations[i].Name == name {
			return &integrations[i]
		}
	}
	return nil
}

func hasAll(values []string, expected ...string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range expected {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}
