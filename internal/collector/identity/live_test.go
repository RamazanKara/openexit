package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/collector"
)

func TestLiveCollectorCollectsOktaInventory(t *testing.T) {
	var authSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "SSWS test-token" {
			authSeen = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps":
			writeJSON(t, w, []map[string]any{
				{
					"id":         "app-api",
					"name":       "oidc_client",
					"label":      "Production API",
					"signOnMode": "OPENID_CONNECT",
					"profile":    map[string]any{"owner": "platform-identity"},
					"credentials": map[string]any{
						"oauthClient": map[string]any{"client_id": "api-web"},
					},
					"settings": map[string]any{
						"oauthClient": map[string]any{
							"redirect_uris": []string{"https://api.example.com/oauth/callback"},
							"grant_types":   []string{"authorization_code", "refresh_token"},
						},
					},
				},
				{
					"id":         "app-saml",
					"name":       "saml_app",
					"label":      "HR Portal",
					"signOnMode": "SAML_2_0",
					"credentials": map[string]any{
						"signing": map[string]any{"kid": "sig-1", "expiresAt": "2027-01-02T03:04:05Z"},
					},
					"settings": map[string]any{
						"signOn": map[string]any{"ssoAcsUrl": "https://hr.example.com/saml/acs"},
					},
				},
			})
		case "/api/v1/apps/app-api/groups":
			writeJSON(t, w, []map[string]any{{"id": "grp-engineering", "profile": map[string]any{"name": "Engineering"}}})
		case "/api/v1/apps/app-saml/groups":
			writeJSON(t, w, []map[string]any{{"id": "grp-hr", "profile": map[string]any{"name": "HR"}}})
		case "/api/v1/groups":
			writeJSON(t, w, []map[string]any{
				{"id": "grp-engineering", "profile": map[string]any{"name": "Engineering", "owner": "platform-identity"}},
				{"id": "grp-hr", "profile": map[string]any{"name": "HR", "owners": []string{"people-ops"}}},
			})
		case "/api/v1/groups/grp-engineering/users":
			writeJSON(t, w, []map[string]any{{"id": "00u1"}, {"id": "00u2"}})
		case "/api/v1/groups/grp-hr/users":
			writeJSON(t, w, []map[string]any{{"id": "00u3"}})
		case "/api/v1/policies":
			switch r.URL.Query().Get("type") {
			case "OKTA_SIGN_ON":
				writeJSON(t, w, []map[string]any{{
					"id":       "pol-global",
					"name":     "Global Sign-On",
					"type":     "OKTA_SIGN_ON",
					"priority": 1,
					"conditions": map[string]any{
						"people": map[string]any{"groups": map[string]any{"include": []string{"grp-engineering"}}},
					},
				}})
			default:
				writeJSON(t, w, []map[string]any{})
			}
		case "/api/v1/policies/pol-global/rules":
			writeJSON(t, w, []map[string]any{{
				"id":       "rule-global",
				"name":     "Require MFA",
				"priority": 1,
				"actions": map[string]any{
					"signon": map[string]any{"requireFactor": true},
				},
			}})
		case "/api/v1/org/factors":
			writeJSON(t, w, []map[string]any{
				{"id": "google_otp", "provider": "GOOGLE", "factorType": "token:software:totp", "status": "ACTIVE"},
				{"id": "okta_sms", "provider": "OKTA", "factorType": "sms", "status": "INACTIVE"},
			})
		case "/api/v1/users/breakglass-admin":
			writeJSON(t, w, map[string]any{"id": "00ubg", "profile": map[string]any{"login": "breakglass-admin"}})
		case "/api/v1/users/00ubg/factors":
			writeJSON(t, w, []map[string]any{{"id": "google_otp", "status": "ACTIVE"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("OKTA_API_TOKEN", "test-token")
	projectDir := t.TempDir()
	inv, err := LiveCollector{}.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "identity-demo",
		Source:     "identity",
		Options: map[string]string{
			"org-url":           server.URL,
			"token-env":         "OKTA_API_TOKEN",
			"break-glass-users": "breakglass-admin",
		},
	})
	if err != nil {
		t.Fatalf("collect okta inventory: %v", err)
	}
	if !authSeen {
		t.Fatalf("expected collector to send Okta token authentication")
	}
	if inv.Summary.IdentityApps != 2 || inv.Summary.IdentityGroups != 2 || inv.Summary.IdentityPolicies != 1 || inv.Summary.MFASettings != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	app := inv.Assets.IdentityApps[0]
	if app.ID != "app-api" || app.Protocol != "oidc" || app.ClientID != "api-web" || len(app.Groups) != 1 || len(app.Owners) != 1 {
		t.Fatalf("unexpected app metadata: %+v", app)
	}
	saml := inv.Assets.IdentityApps[1]
	if saml.Protocol != "saml" || !saml.SAMLSigningCertPresent || saml.SAMLSigningCertExpiresAt == "" {
		t.Fatalf("unexpected SAML metadata: %+v", saml)
	}
	if inv.Assets.IdentityGroups[0].Members != 2 {
		t.Fatalf("unexpected group member count: %+v", inv.Assets.IdentityGroups[0])
	}
	if !inv.Assets.IdentityPolicies[0].EnforcesMFA {
		t.Fatalf("expected policy to enforce MFA: %+v", inv.Assets.IdentityPolicies[0])
	}
	if !inv.Assets.MFASettings[0].Required || len(inv.Assets.MFASettings[0].Factors) != 1 {
		t.Fatalf("unexpected MFA settings: %+v", inv.Assets.MFASettings[0])
	}
	if !inv.Assets.BreakGlassAccounts[0].MFAEnabled {
		t.Fatalf("expected break-glass MFA metadata: %+v", inv.Assets.BreakGlassAccounts[0])
	}
	if _, err := os.Stat(filepath.Join(projectDir, "evidence", "identity", "raw-okta-api-collection.json")); err != nil {
		t.Fatalf("expected raw Okta API evidence: %v", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json response: %v", err)
	}
}
