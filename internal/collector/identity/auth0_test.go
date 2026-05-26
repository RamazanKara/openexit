package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestAuth0CollectorCollectsIdentityInventory(t *testing.T) {
	var roleUsersCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-auth0-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		switch r.URL.Path {
		case "/api/v2/clients":
			writeJSON(t, w, map[string]any{
				"total":    2,
				"page":     0,
				"per_page": 100,
				"clients": []map[string]any{
					{
						"client_id":       "client_spa",
						"name":            "Customer Portal",
						"app_type":        "spa",
						"callbacks":       []string{"https://app.example.com/callback"},
						"allowed_origins": []string{"https://app.example.com"},
						"web_origins":     []string{"https://app.example.com"},
						"grant_types":     []string{"authorization_code", "refresh_token"},
						"client_metadata": map[string]any{"owner": "identity-team"},
					},
					{
						"client_id":   "client_saml",
						"name":        "Partner SAML",
						"app_type":    "regular_web",
						"callbacks":   []string{"https://partner.example.com/saml/acs"},
						"grant_types": []string{"implicit"},
						"addons":      map[string]any{"samlp": map[string]any{"enabled": true}},
						"owners":      []string{"partner-team"},
					},
				},
			})
		case "/api/v2/roles":
			writeJSON(t, w, map[string]any{
				"total": 1,
				"roles": []map[string]any{
					{"id": "rol_admin", "name": "Administrators", "description": "Tenant admins"},
				},
			})
		case "/api/v2/roles/rol_admin/users":
			roleUsersCalls++
			writeJSON(t, w, map[string]any{
				"total": 2,
				"users": []map[string]any{
					{"user_id": "auth0|one", "email": "one@example.com"},
					{"user_id": "auth0|two", "email": "two@example.com"},
				},
			})
		case "/api/v2/actions/actions":
			writeJSON(t, w, map[string]any{
				"total": 1,
				"actions": []map[string]any{
					{
						"id":                   "act_mfa",
						"name":                 "Require MFA For Admins",
						"runtime":              "node22",
						"status":               "built",
						"all_changes_deployed": true,
						"supported_triggers": []map[string]any{
							{"id": "post-login", "version": "v3", "status": "current"},
						},
					},
				},
			})
		case "/api/v2/rules":
			writeJSON(t, w, []map[string]any{
				{"id": "rul_legacy", "name": "Legacy login enrichment", "enabled": true, "order": 3, "stage": "login_success"},
			})
		case "/api/v2/guardian/factors":
			writeJSON(t, w, []map[string]any{
				{"name": "push-notification", "enabled": true},
				{"name": "sms", "enabled": false},
			})
		case "/api/v2/users":
			if !strings.Contains(r.URL.Query().Get("q"), `admin@example.com`) {
				t.Fatalf("unexpected user search query: %q", r.URL.Query().Get("q"))
			}
			writeJSON(t, w, map[string]any{
				"total": 1,
				"users": []map[string]any{
					{"user_id": "auth0|admin", "email": "admin@example.com", "multifactor": []string{"guardian"}},
				},
			})
		case "/api/v2/users/auth0|admin/authentication-methods":
			writeJSON(t, w, []map[string]any{
				{"id": "method_1", "type": "webauthn-roaming", "name": "security-key"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("AUTH0_MANAGEMENT_TOKEN", "test-auth0-token")
	projectDir := t.TempDir()
	c := Auth0Collector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "auth0-demo",
		Options: map[string]string{
			"domain":            server.URL,
			"token-env":         "AUTH0_MANAGEMENT_TOKEN",
			"break-glass-users": "admin@example.com",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if roleUsersCalls != 1 {
		t.Fatalf("expected one role users call, got %d", roleUsersCalls)
	}
	if inv.Source.Type != "identity" || inv.Source.Site != server.URL {
		t.Fatalf("unexpected source: %+v", inv.Source)
	}
	if inv.Summary.IdentityApps != 2 || inv.Summary.IdentityGroups != 1 || inv.Summary.IdentityPolicies != 2 || inv.Summary.MFASettings != 1 || inv.Summary.BreakGlassAccounts != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	if got := appProtocolByID(inv.Assets.IdentityApps, "client_saml"); got != "saml" {
		t.Fatalf("unexpected SAML app protocol: %s", got)
	}
	if inv.Assets.IdentityGroups[0].Members != 2 {
		t.Fatalf("unexpected role member count: %+v", inv.Assets.IdentityGroups[0])
	}
	if !inv.Assets.MFASettings[0].Required || len(inv.Assets.MFASettings[0].Factors) != 1 || inv.Assets.MFASettings[0].Factors[0] != "push-notification" {
		t.Fatalf("unexpected MFA settings: %+v", inv.Assets.MFASettings[0])
	}
	if !inv.Assets.BreakGlassAccounts[0].MFAEnabled {
		t.Fatalf("expected break-glass MFA to be enabled: %+v", inv.Assets.BreakGlassAccounts[0])
	}
	rawPath := filepath.Join(projectDir, "evidence", "identity", "raw-auth0-api-collection.json")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("expected Auth0 raw collection evidence: %v", err)
	}
	if strings.Contains(string(raw), "test-auth0-token") {
		t.Fatalf("raw evidence should not contain Auth0 token")
	}
}

func appProtocolByID(apps []inventory.IdentityApp, id string) string {
	for _, app := range apps {
		if app.ID == id {
			return app.Protocol
		}
	}
	return ""
}
