package edge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestLiveCollectorCollectsCloudflareInventory(t *testing.T) {
	var authSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-token" {
			authSeen = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/zones/zone123":
			writeEnvelope(t, w, map[string]any{"id": "zone123", "name": "example.com"})
		case "/zones/zone123/dns_records":
			writePagedEnvelope(t, w, []map[string]any{
				{"id": "dns-api", "name": "api.example.com", "type": "A", "content": "203.0.113.10", "ttl": 3600, "proxied": true},
				{"id": "dns-wildcard", "name": "*.example.com", "type": "CNAME", "content": "origin.example.net", "ttl": 86400, "proxied": false},
			})
		case "/zones/zone123/settings/ssl":
			writeEnvelope(t, w, map[string]any{"id": "ssl", "value": "flexible"})
		case "/zones/zone123/settings/min_tls_version":
			writeEnvelope(t, w, map[string]any{"id": "min_tls_version", "value": "1.0"})
		case "/zones/zone123/settings/always_use_https":
			writeEnvelope(t, w, map[string]any{"id": "always_use_https", "value": "on"})
		case "/zones/zone123/settings/security_header":
			writeEnvelope(t, w, map[string]any{
				"id": "security_header",
				"value": map[string]any{
					"strict_transport_security": map[string]any{"enabled": false},
				},
			})
		case "/zones/zone123/rulesets/phases/http_request_firewall_custom/entrypoint":
			writeEnvelope(t, w, map[string]any{
				"id":    "rs-waf",
				"name":  "Custom WAF",
				"phase": "http_request_firewall_custom",
				"rules": []map[string]any{
					{"id": "waf-sqli", "description": "SQLi", "expression": `http.request.uri.path contains "/api"`, "action": "block", "enabled": true},
					{"id": "bot-low-score", "description": "Bot low score", "expression": "cf.bot_management.score lt 30", "action": "challenge", "enabled": true},
				},
			})
		case "/zones/zone123/rulesets/phases/http_request_firewall_managed/entrypoint":
			writeEnvelope(t, w, map[string]any{
				"id":    "rs-managed",
				"name":  "Managed WAF",
				"phase": "http_request_firewall_managed",
				"rules": []map[string]any{
					{"id": "managed-owasp", "description": "OWASP CRS", "expression": "true", "action": "execute", "enabled": true},
				},
			})
		case "/zones/zone123/rulesets/phases/http_request_cache_settings/entrypoint":
			writeEnvelope(t, w, map[string]any{
				"id":    "rs-cache",
				"name":  "Cache Rules",
				"phase": "http_request_cache_settings",
				"rules": []map[string]any{
					{
						"id":          "cache-static",
						"description": "Static assets",
						"expression":  `http.request.uri.path contains "/assets/"`,
						"action":      "set_cache_settings",
						"enabled":     true,
						"action_parameters": map[string]any{
							"cache":       true,
							"edge_ttl":    map[string]any{"default": 86400},
							"browser_ttl": map[string]any{"default": 3600},
							"cache_key":   map[string]any{"custom_key": map[string]any{"query_string": true, "host": true}},
						},
					},
				},
			})
		case "/zones/zone123/rulesets/phases/http_request_dynamic_redirect/entrypoint":
			writeEnvelope(t, w, map[string]any{
				"id":    "rs-redirect",
				"name":  "Redirect Rules",
				"phase": "http_request_dynamic_redirect",
				"rules": []map[string]any{
					{
						"id":          "redir-www",
						"description": "WWW canonical",
						"expression":  `http.host eq "www.example.com"`,
						"action":      "redirect",
						"enabled":     true,
						"action_parameters": map[string]any{
							"from_value": map[string]any{
								"target_url":            map[string]any{"value": "https://example.com/$1"},
								"status_code":           301,
								"preserve_query_string": true,
							},
						},
					},
				},
			})
		case "/zones/zone123/rulesets/phases/http_request_sbfm/entrypoint":
			http.NotFound(w, r)
		case "/zones/zone123/pagerules":
			writePagedEnvelope(t, w, []map[string]any{
				{
					"id":       "page-cache-html",
					"status":   "active",
					"priority": 1,
					"targets":  []map[string]any{{"constraint": map[string]any{"value": "https://example.com/blog/*"}}},
					"actions":  []map[string]any{{"id": "cache_everything"}, {"id": "edge_cache_ttl", "value": 7200}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("CLOUDFLARE_API_TOKEN", "test-token")
	projectDir := t.TempDir()
	inv, err := LiveCollector{}.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "edge-demo",
		Source:     "edge",
		Options: map[string]string{
			"zone-id":   "zone123",
			"token-env": "CLOUDFLARE_API_TOKEN",
			"base-url":  server.URL,
		},
	})
	if err != nil {
		t.Fatalf("collect cloudflare inventory: %v", err)
	}
	if !authSeen {
		t.Fatalf("expected collector to send Cloudflare bearer authentication")
	}
	if inv.Summary.DNSRecords != 2 || inv.Summary.WAFRules != 3 || inv.Summary.CacheRules != 1 || inv.Summary.Redirects != 2 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	if inv.Assets.DNSRecords[0].Value == "" || inv.Assets.DNSRecords[0].EvidenceRef == "" {
		t.Fatalf("unexpected DNS metadata: %+v", inv.Assets.DNSRecords[0])
	}
	if !containsManagedWAF(inv.Assets.WAFRules) {
		t.Fatalf("expected managed WAF metadata: %+v", inv.Assets.WAFRules)
	}
	if inv.Assets.CacheRules[0].EdgeTTL != 86400 || len(inv.Assets.CacheRules[0].CacheKey) == 0 {
		t.Fatalf("unexpected cache rule metadata: %+v", inv.Assets.CacheRules[0])
	}
	if inv.Assets.TLSSettings[0].Mode != "flexible" || inv.Assets.TLSSettings[0].MinVersion != "1.0" {
		t.Fatalf("unexpected TLS metadata: %+v", inv.Assets.TLSSettings[0])
	}
	if len(inv.Assets.BotRules) != 1 || inv.Assets.BotRules[0].Action != "challenge" {
		t.Fatalf("unexpected bot rule metadata: %+v", inv.Assets.BotRules)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "evidence", "edge", "raw-cloudflare-api-collection.json")); err != nil {
		t.Fatalf("expected raw Cloudflare API evidence: %v", err)
	}
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	writeJSON(t, w, map[string]any{"success": true, "errors": []any{}, "messages": []any{}, "result": result})
}

func writePagedEnvelope(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	writeJSON(t, w, map[string]any{
		"success":     true,
		"errors":      []any{},
		"messages":    []any{},
		"result":      result,
		"result_info": map[string]any{"page": 1, "total_pages": 1},
	})
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json response: %v", err)
	}
}

func containsManagedWAF(rules []inventory.WAFRule) bool {
	for _, rule := range rules {
		if rule.Managed {
			return true
		}
	}
	return false
}
