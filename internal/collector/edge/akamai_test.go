package edge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestAkamaiCollectorCollectsEdgeInventory(t *testing.T) {
	var authSeen bool
	var papiPrefixSeen bool
	var accountSwitchSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			authSeen = true
		}
		if r.Header.Get("PAPI-Use-Prefixes") == "true" {
			papiPrefixSeen = true
		}
		if r.URL.Query().Get("accountSwitchKey") == "A-CCT1234:A-CCT5678" {
			accountSwitchSeen = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/config-dns/v2/zones/example.com/recordsets":
			writeJSON(t, w, map[string]any{"recordsets": []map[string]any{
				{"name": "api.example.com", "type": "A", "ttl": 300, "rdata": []string{"203.0.113.20"}},
				{"name": "www.example.com", "type": "CNAME", "ttl": 600, "rdata": []string{"www.example.com.edgekey.net"}},
			}})
		case "/papi/v1/properties/prp_123/versions/latest":
			assertPAPIQuery(t, r)
			writeJSON(t, w, map[string]any{"propertyVersion": 5})
		case "/papi/v1/properties/prp_123/versions/5/hostnames":
			assertPAPIQuery(t, r)
			writeJSON(t, w, map[string]any{"hostnames": map[string]any{"items": []map[string]any{
				{"cnameFrom": "shop.example.com", "cnameTo": "shop.example.com.edgesuite.net", "certProvisioningType": "CPS_MANAGED"},
			}}})
		case "/papi/v1/properties/prp_123/versions/5/rules":
			assertPAPIQuery(t, r)
			writeJSON(t, w, map[string]any{
				"propertyId":      "prp_123",
				"propertyName":    "example-property",
				"propertyVersion": 5,
				"rules": map[string]any{
					"name": "default",
					"behaviors": []map[string]any{
						{"name": "origin", "options": map[string]any{"hostname": "origin.example.net", "httpsPort": 443, "forwardHostHeader": "REQUEST_HOST_HEADER", "verificationMode": "PLATFORM_SETTINGS"}},
						{"name": "caching", "options": map[string]any{"behavior": "MAX_AGE", "ttl": 600, "cacheKeyHostname": "ORIGIN_HOSTNAME"}},
						{"name": "redirect", "options": map[string]any{"destinationHostname": "www.example.com", "responseCode": 301}},
						{"name": "httpStrictTransportSecurity", "options": map[string]any{"enabled": true, "maxAge": 31536000}},
						{"name": "applicationSecurity", "options": map[string]any{"enabled": true, "policy": "pol_1"}},
						{"name": "botManager", "options": map[string]any{"enabled": true, "mode": "monitor"}},
					},
					"children": []map[string]any{
						{
							"name":     "assets",
							"criteria": []map[string]any{{"name": "path", "options": map[string]any{"matchOperator": "MATCHES_ONE_OF", "values": []string{"/assets/*"}}}},
							"behaviors": []map[string]any{
								{"name": "caching", "options": map[string]any{"behavior": "MAX_AGE", "ttl": 86400}},
							},
						},
					},
				},
			})
		case "/appsec/v1/configs":
			writeJSON(t, w, map[string]any{"configurations": []map[string]any{{"id": 123, "name": "appsec", "latestVersion": 7}}})
		case "/appsec/v1/configs/123/versions/7/security-policies":
			if r.URL.Query().Get("detail") != "true" {
				t.Fatalf("expected detail query for security policies")
			}
			writeJSON(t, w, map[string]any{"policies": []map[string]any{{"policyId": "pol_1", "policyName": "Production policy"}}})
		case "/appsec/v1/configs/123/versions/7/security-policies/pol_1/custom-rules":
			enabled := true
			writeJSON(t, w, map[string]any{"customRules": []map[string]any{
				{"id": 99, "name": "Block admin probes", "action": "deny", "enabled": enabled, "conditions": []map[string]any{{"name": "path", "options": map[string]any{"values": []string{"/admin/*"}}}}},
			}})
		case "/appsec/v1/configs/123/versions/7/security-policies/pol_1/protections":
			writeJSON(t, w, map[string]any{"protections": map[string]any{"botProtection": "monitor"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	t.Setenv("AKAMAI_HOST", u.Host)
	t.Setenv("AKAMAI_CLIENT_TOKEN", "test-client-token")
	t.Setenv("AKAMAI_ACCESS_TOKEN", "test-access-token")
	t.Setenv("AKAMAI_CLIENT_SECRET", "test-client-secret")
	t.Setenv("AKAMAI_ACCOUNT_KEY", "A-CCT1234:A-CCT5678")

	projectDir := t.TempDir()
	inv, err := AkamaiCollector{}.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "akamai-demo",
		Source:     "edge",
		Options: map[string]string{
			"base-url":            server.URL,
			"zones":               "example.com",
			"property-ids":        "prp_123",
			"contract-id":         "ctr_1",
			"group-id":            "grp_1",
			"security-config-ids": "123:7",
		},
	})
	if err != nil {
		t.Fatalf("collect Akamai inventory: %v", err)
	}
	if !authSeen {
		t.Fatalf("expected Akamai EdgeGrid authorization header")
	}
	if !papiPrefixSeen {
		t.Fatalf("expected PAPI prefix header")
	}
	if !accountSwitchSeen {
		t.Fatalf("expected account switch key query parameter")
	}
	if inv.Summary.DNSRecords != 3 || inv.Summary.Origins != 1 || inv.Summary.CacheRules != 2 || inv.Summary.Redirects != 1 {
		t.Fatalf("unexpected edge delivery summary: %+v", inv.Summary)
	}
	if inv.Summary.WAFRules != 2 || inv.Summary.BotRules != 2 || inv.Summary.TLSSettings != 2 {
		t.Fatalf("unexpected edge security summary: %+v", inv.Summary)
	}
	if inv.Assets.Origins[0].Hostname != "origin.example.net" || inv.Assets.Origins[0].HostHeader != "request-host-header" {
		t.Fatalf("unexpected origin metadata: %+v", inv.Assets.Origins[0])
	}
	if !hasAkamaiCustomRule(inv.Assets.WAFRules, "Block admin probes") {
		t.Fatalf("expected AppSec custom rule metadata: %+v", inv.Assets.WAFRules)
	}
	rawPath := filepath.Join(projectDir, "evidence", "edge", "raw-akamai-api-collection.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("expected raw Akamai API evidence: %v", err)
	}
	if string(raw) == "" || containsAny(string(raw), "test-client-secret", "test-access-token", "test-client-token") {
		t.Fatalf("raw evidence should be present and credential-free: %s", string(raw))
	}
}

func assertPAPIQuery(t *testing.T, r *http.Request) {
	t.Helper()
	if r.URL.Query().Get("contractId") != "ctr_1" || r.URL.Query().Get("groupId") != "grp_1" {
		t.Fatalf("expected PAPI contract/group query, got %s", r.URL.RawQuery)
	}
}

func hasAkamaiCustomRule(rules []inventory.WAFRule, name string) bool {
	for _, rule := range rules {
		if rule.Name == name {
			return true
		}
	}
	return false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
