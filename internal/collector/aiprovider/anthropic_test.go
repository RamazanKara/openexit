package aiprovider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/aiprovider"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestAnthropicCollectorCollectsUsageInventory(t *testing.T) {
	var usageCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "test-admin-key" {
			t.Fatalf("unexpected x-api-key header: %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("unexpected anthropic-version header: %q", got)
		}
		if r.URL.Path != "/v1/organizations/usage_report/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		usageCalls++
		if got := r.URL.Query()["group_by[]"]; len(got) != 1 || got[0] != "model" {
			t.Fatalf("unexpected group_by query: %#v", got)
		}
		if got := r.URL.Query()["workspace_ids[]"]; len(got) != 1 || got[0] != "wrkspc_platform" {
			t.Fatalf("unexpected workspace filter: %#v", got)
		}
		switch r.URL.Query().Get("bucket_width") {
		case "1d":
			if got := r.URL.Query().Get("limit"); got != "31" {
				t.Fatalf("unexpected daily limit: %q", got)
			}
			writeAnthropicJSON(t, w, map[string]any{
				"data": []map[string]any{
					{
						"starting_at": "2026-05-01T00:00:00Z",
						"ending_at":   "2026-05-02T00:00:00Z",
						"results": []map[string]any{
							{
								"model":                   "claude-sonnet-4-20250514",
								"uncached_input_tokens":   1000,
								"cache_read_input_tokens": 200,
								"cache_creation":          map[string]any{"ephemeral_1h_input_tokens": 300, "ephemeral_5m_input_tokens": 100},
								"output_tokens":           500,
								"workspace_id":            "wrkspc_platform",
								"service_tier":            "standard",
								"context_window":          "0-200k",
								"server_tool_use":         map[string]any{"web_search_requests": 2},
							},
							{
								"model":                   "claude-3-5-haiku-20241022",
								"uncached_input_tokens":   400,
								"cache_read_input_tokens": 0,
								"cache_creation":          map[string]any{},
								"output_tokens":           100,
								"workspace_id":            "wrkspc_platform",
								"service_tier":            "batch",
							},
						},
					},
					{
						"starting_at": "2026-05-02T00:00:00Z",
						"ending_at":   "2026-05-03T00:00:00Z",
						"results": []map[string]any{
							{
								"model":                 "claude-sonnet-4-20250514",
								"uncached_input_tokens": 700,
								"cache_creation":        map[string]any{},
								"output_tokens":         300,
								"server_tool_use":       map[string]any{"web_search_requests": 1},
							},
						},
					},
				},
				"has_more":  false,
				"next_page": nil,
			})
		case "1h":
			if got := r.URL.Query().Get("limit"); got != "168" {
				t.Fatalf("unexpected hourly limit: %q", got)
			}
			writeAnthropicJSON(t, w, map[string]any{
				"data": []map[string]any{
					{
						"starting_at": "2026-05-03T10:00:00Z",
						"ending_at":   "2026-05-03T11:00:00Z",
						"results": []map[string]any{
							{
								"model":                   "claude-sonnet-4-20250514",
								"uncached_input_tokens":   60000,
								"cache_read_input_tokens": 12000,
								"cache_creation":          map[string]any{"ephemeral_5m_input_tokens": 3000},
								"output_tokens":           15000,
							},
							{
								"model":                 "claude-3-5-haiku-20241022",
								"uncached_input_tokens": 3000,
								"cache_creation":        map[string]any{},
								"output_tokens":         3000,
							},
						},
					},
				},
				"has_more":  false,
				"next_page": nil,
			})
		default:
			t.Fatalf("unexpected bucket_width: %q", r.URL.Query().Get("bucket_width"))
		}
	}))
	defer server.Close()

	t.Setenv("ANTHROPIC_ADMIN_KEY", "test-admin-key")
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := aiprovider.AnthropicCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "anthropic-demo",
		Options: map[string]string{
			"admin-key-env":         "ANTHROPIC_ADMIN_KEY",
			"base-url":              server.URL,
			"anthropic-version":     "2023-06-01",
			"days":                  "30",
			"peak-days":             "7",
			"workspace":             "platform",
			"workspace-ids":         "wrkspc_platform",
			"owners":                "platform-ai",
			"default-p95-ms":        "2200",
			"default-timeout-ms":    "12000",
			"streaming-required":    "true",
			"fallback-strategy":     "manual-queue",
			"fallback-manual-queue": "true",
			"fallback-max-retries":  "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if usageCalls != 2 {
		t.Fatalf("expected daily and hourly usage calls, got %d", usageCalls)
	}
	if inv.Source.Type != "ai-provider" || inv.Source.Site != "anthropic/platform" {
		t.Fatalf("unexpected source: %+v", inv.Source)
	}
	if inv.Summary.AIModelUsageClasses != 2 || inv.Summary.AITokenVolumes != 2 || inv.Summary.AILatencyExpectations != 2 || inv.Summary.AIFallbackBehaviors != 2 || inv.Summary.AIToolUsages != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	sonnetVolume := tokenVolumeByUsageClass(t, inv.Assets.AITokenVolumes, "anthropic-claude-sonnet-4-20250514")
	if sonnetVolume.MonthlyInputTokens != 2300 || sonnetVolume.MonthlyOutputTokens != 800 {
		t.Fatalf("unexpected sonnet totals: %+v", sonnetVolume)
	}
	if sonnetVolume.PeakTokensPerMinute != 1500 {
		t.Fatalf("unexpected peak tokens per minute: %d", sonnetVolume.PeakTokensPerMinute)
	}
	sonnetClass := usageClassByID(t, inv.Assets.AIModelUsageClasses, "anthropic-claude-sonnet-4-20250514")
	if len(sonnetClass.ToolRefs) != 1 || sonnetClass.FallbackBehavior != "anthropic-claude-sonnet-4-20250514-fallback" {
		t.Fatalf("unexpected sonnet class metadata: %+v", sonnetClass)
	}
	tool := toolUsageByID(t, inv.Assets.AIToolUsages, "anthropic-claude-sonnet-4-20250514-web-search")
	if !tool.NetworkAccess || tool.WritesExternalState {
		t.Fatalf("unexpected tool metadata: %+v", tool)
	}
	rawPath := filepath.Join(projectDir, "evidence", "ai-provider", "raw-anthropic-api-collection.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("expected raw Anthropic collection evidence: %v", err)
	}
	if strings.Contains(string(raw), "test-admin-key") {
		t.Fatalf("raw evidence should not contain Anthropic admin key")
	}
}

func writeAnthropicJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func toolUsageByID(t *testing.T, values []inventory.AIToolUsage, id string) inventory.AIToolUsage {
	t.Helper()
	for _, value := range values {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("missing tool usage %s", id)
	return inventory.AIToolUsage{}
}
