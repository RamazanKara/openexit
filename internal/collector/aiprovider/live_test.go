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

func TestLiveCollectorCollectsOpenAIUsageInventory(t *testing.T) {
	var usageCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-admin-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("OpenAI-Organization"); got != "org_test" {
			t.Fatalf("unexpected organization header: %q", got)
		}
		if got := r.Header.Get("OpenAI-Project"); got != "proj_test" {
			t.Fatalf("unexpected project header: %q", got)
		}
		switch r.URL.Path {
		case "/models":
			writeJSON(t, w, map[string]any{
				"object": "list",
				"data": []map[string]any{
					{"id": "gpt-4o-mini", "object": "model", "owned_by": "system"},
					{"id": "gpt-4o", "object": "model", "owned_by": "system"},
				},
			})
		case "/organization/usage/completions":
			usageCalls++
			if got := r.URL.Query().Get("group_by"); got != "model" {
				t.Fatalf("unexpected group_by query: %q", got)
			}
			switch r.URL.Query().Get("bucket_width") {
			case "1d":
				if got := r.URL.Query().Get("limit"); got != "31" {
					t.Fatalf("unexpected daily limit: %q", got)
				}
				writeJSON(t, w, map[string]any{
					"object": "page",
					"data": []map[string]any{
						{
							"object":     "bucket",
							"start_time": int64(1710000000),
							"end_time":   int64(1710086400),
							"results": []map[string]any{
								{
									"object":             "organization.usage.completions.result",
									"model":              "gpt-4o-mini",
									"input_tokens":       1200,
									"output_tokens":      300,
									"num_model_requests": 12,
								},
								{
									"object":             "organization.usage.completions.result",
									"model":              "gpt-4o",
									"input_tokens":       900,
									"output_tokens":      600,
									"num_model_requests": 6,
								},
							},
						},
						{
							"object":     "bucket",
							"start_time": int64(1710086400),
							"end_time":   int64(1710172800),
							"results": []map[string]any{
								{
									"object":             "organization.usage.completions.result",
									"model":              "gpt-4o-mini",
									"input_tokens":       800,
									"output_tokens":      200,
									"input_audio_tokens": 50,
									"num_model_requests": 8,
								},
							},
						},
					},
					"has_more": false,
				})
			case "1h":
				if got := r.URL.Query().Get("limit"); got != "168" {
					t.Fatalf("unexpected hourly limit: %q", got)
				}
				writeJSON(t, w, map[string]any{
					"object": "page",
					"data": []map[string]any{
						{
							"object":     "bucket",
							"start_time": int64(1710000000),
							"end_time":   int64(1710003600),
							"results": []map[string]any{
								{
									"object":        "organization.usage.completions.result",
									"model":         "gpt-4o-mini",
									"input_tokens":  100000,
									"output_tokens": 20000,
								},
								{
									"object":        "organization.usage.completions.result",
									"model":         "gpt-4o",
									"input_tokens":  30000,
									"output_tokens": 30000,
								},
							},
						},
					},
					"has_more": false,
				})
			default:
				t.Fatalf("unexpected bucket_width: %q", r.URL.Query().Get("bucket_width"))
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OPENAI_ADMIN_KEY", "test-admin-key")
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := aiprovider.LiveCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "ai-live-demo",
		Options: map[string]string{
			"admin-key-env":         "OPENAI_ADMIN_KEY",
			"base-url":              server.URL,
			"days":                  "30",
			"peak-days":             "7",
			"workspace":             "acme",
			"organization-id":       "org_test",
			"project-id":            "proj_test",
			"owners":                "platform-ai\nml-platform",
			"default-p95-ms":        "1800",
			"default-timeout-ms":    "9000",
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
	if inv.Source.Type != "ai-provider" || inv.Source.Site != "openai/acme" {
		t.Fatalf("unexpected source: %+v", inv.Source)
	}
	if inv.Summary.AIModelUsageClasses != 2 || inv.Summary.AITokenVolumes != 2 || inv.Summary.AILatencyExpectations != 2 || inv.Summary.AIFallbackBehaviors != 2 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	miniVolume := tokenVolumeByUsageClass(t, inv.Assets.AITokenVolumes, "openai-gpt-4o-mini")
	if miniVolume.MonthlyInputTokens != 2050 || miniVolume.MonthlyOutputTokens != 500 {
		t.Fatalf("unexpected gpt-4o-mini totals: %+v", miniVolume)
	}
	if miniVolume.PeakTokensPerMinute != 2000 {
		t.Fatalf("unexpected peak tokens per minute: %d", miniVolume.PeakTokensPerMinute)
	}
	miniClass := usageClassByID(t, inv.Assets.AIModelUsageClasses, "openai-gpt-4o-mini")
	if len(miniClass.Owners) != 2 || miniClass.FallbackBehavior != "openai-gpt-4o-mini-fallback" {
		t.Fatalf("unexpected usage class metadata: %+v", miniClass)
	}
	if len(inv.Warnings) == 0 {
		t.Fatalf("expected manual metadata warning")
	}
	rawPath := filepath.Join(projectDir, "evidence", "ai-provider", "raw-openai-api-collection.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("expected raw OpenAI collection evidence: %v", err)
	}
	if strings.Contains(string(raw), "test-admin-key") {
		t.Fatalf("raw evidence should not contain API keys")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func usageClassByID(t *testing.T, values []inventory.AIModelUsageClass, id string) inventory.AIModelUsageClass {
	t.Helper()
	for _, value := range values {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("missing usage class %s", id)
	return inventory.AIModelUsageClass{}
}

func tokenVolumeByUsageClass(t *testing.T, values []inventory.AITokenVolume, usageClassID string) inventory.AITokenVolume {
	t.Helper()
	for _, value := range values {
		if value.UsageClassID == usageClassID {
			return value
		}
	}
	t.Fatalf("missing token volume for %s", usageClassID)
	return inventory.AITokenVolume{}
}
