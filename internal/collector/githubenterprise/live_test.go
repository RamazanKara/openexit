package githubenterprise

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/collector"
)

func TestLiveCollectorCollectsGitHubEnterpriseInventory(t *testing.T) {
	var authSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-token" {
			authSeen = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/orgs/acme/repos":
			writeJSON(t, w, []map[string]any{{
				"name":            "platform-api",
				"full_name":       "acme/platform-api",
				"visibility":      "private",
				"default_branch":  "main",
				"archived":        false,
				"topics":          []string{"payments", "platform"},
				"has_pages":       true,
				"has_packages":    true,
				"has_discussions": true,
			}})
		case "/repos/acme/platform-api/teams":
			writeJSON(t, w, []map[string]any{{"name": "Platform", "slug": "platform"}})
		case "/repos/acme/platform-api/branches/main/protection":
			writeJSON(t, w, map[string]any{
				"required_status_checks": map[string]any{
					"contexts": []string{"lint"},
					"checks":   []map[string]any{{"context": "test"}},
				},
				"required_pull_request_reviews": map[string]any{
					"required_approving_review_count": 2,
					"require_code_owner_reviews":      true,
				},
				"restrictions":       map[string]any{},
				"allow_force_pushes": map[string]any{"enabled": false},
			})
		case "/repos/acme/platform-api/actions/workflows":
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"workflows": []map[string]any{{
					"id":    7,
					"name":  "CI",
					"path":  ".github/workflows/ci.yml",
					"state": "active",
				}},
			})
		case "/repos/acme/platform-api/contents/.github/CODEOWNERS":
			writeGitHubContent(t, w, "* @acme/platform")
		case "/repos/acme/platform-api/contents/.github/workflows/ci.yml":
			writeGitHubContent(t, w, `name: CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          password: ${{ secrets.DOCKERHUB_TOKEN }}
  deploy:
    runs-on: [self-hosted, linux]
`)
		case "/repos/acme/platform-api/actions/secrets":
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"secrets":     []map[string]any{{"name": "DOCKERHUB_TOKEN"}},
			})
		case "/repos/acme/platform-api/keys":
			writeJSON(t, w, []map[string]any{{"title": "deploy-prod", "read_only": false}})
		case "/orgs/acme/teams":
			writeJSON(t, w, []map[string]any{{"name": "Platform", "slug": "platform"}})
		case "/orgs/acme/teams/platform/repos":
			writeJSON(t, w, []map[string]any{{"name": "platform-api", "full_name": "acme/platform-api"}})
		case "/orgs/acme/teams/platform/members":
			if r.URL.Query().Get("role") == "maintainer" {
				writeJSON(t, w, []map[string]any{{"login": "ada"}})
				return
			}
			writeJSON(t, w, []map[string]any{{"login": "ada"}, {"login": "grace"}})
		case "/orgs/acme/actions/runners":
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"runners": []map[string]any{{
					"name":   "runner-1",
					"status": "online",
					"labels": []map[string]any{{"name": "self-hosted"}, {"name": "linux"}},
				}},
			})
		case "/orgs/acme/actions/secrets":
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"secrets":     []map[string]any{{"name": "ORG_SHARED_SECRET"}},
			})
		case "/orgs/acme/installations":
			writeJSON(t, w, map[string]any{
				"total_count": 1,
				"installations": []map[string]any{{
					"id":                   25381,
					"app_id":               2218,
					"app_slug":             "release-bot",
					"repository_selection": "selected",
					"permissions": map[string]string{
						"contents":      "write",
						"metadata":      "read",
						"pull_requests": "write",
					},
					"events": []string{"pull_request", "workflow_run"},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")
	projectDir := t.TempDir()
	inv, err := LiveCollector{}.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "demo",
		Source:     "github-enterprise",
		Options: map[string]string{
			"owner":      "acme",
			"owner-type": "org",
			"base-url":   server.URL,
			"token-env":  "GITHUB_TOKEN",
		},
	})
	if err != nil {
		t.Fatalf("collect live github inventory: %v", err)
	}
	if !authSeen {
		t.Fatalf("expected collector to send bearer authentication")
	}
	if inv.Summary.Repositories != 1 || inv.Summary.ActionsWorkflows != 1 || inv.Summary.Teams != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	repo := inv.Assets.Repositories[0]
	if repo.Name != "acme/platform-api" || !repo.ActionsEnabled || !repo.HasCODEOWNERS || !repo.UsesGitHubPackages {
		t.Fatalf("unexpected repository metadata: %+v", repo)
	}
	protection := inv.Assets.BranchProtections[0]
	if protection.RequiredReviews != 2 || !protection.RequireCodeOwnerReview || !containsAll(protection.RequiredStatusChecks, "lint", "test") {
		t.Fatalf("unexpected branch protection metadata: %+v", protection)
	}
	workflow := inv.Assets.ActionsWorkflows[0]
	if !workflow.UsesGitHubHosted || !workflow.UsesSelfHosted || !containsAll(workflow.Actions, "actions/checkout@v4", "docker/login-action@v3") || !containsAll(workflow.Secrets, "DOCKERHUB_TOKEN") {
		t.Fatalf("unexpected workflow metadata: %+v", workflow)
	}
	if inv.Assets.Secrets[0].Name == "" || inv.Assets.Secrets[0].Scope == "" {
		t.Fatalf("expected secret metadata without secret values: %+v", inv.Assets.Secrets)
	}
	if inv.Assets.Runners[0].Name != "runner-1" || !inv.Assets.Runners[0].Online {
		t.Fatalf("unexpected runner metadata: %+v", inv.Assets.Runners[0])
	}
	if inv.Assets.DeployKeys[0].Title != "deploy-prod" || inv.Assets.DeployKeys[0].ReadOnly {
		t.Fatalf("unexpected deploy key metadata: %+v", inv.Assets.DeployKeys[0])
	}
	app := inv.Assets.GitHubApps[0]
	if app.Name != "release-bot" || app.RepositorySelection != "selected" || !app.WebhookEnabled || !containsAll(app.Permissions, "contents:write", "metadata:read", "pull_requests:write") || !containsAll(app.Events, "pull_request", "workflow_run") {
		t.Fatalf("unexpected GitHub App metadata: %+v", app)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "evidence", "github-enterprise", "raw-api-collection.json")); err != nil {
		t.Fatalf("expected raw API collection evidence: %v", err)
	}
}

func TestLiveCollectorWarnsWhenGitHubAppsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/orgs/acme/repos":
			writeJSON(t, w, []map[string]any{})
		case "/orgs/acme/teams":
			writeJSON(t, w, []map[string]any{})
		case "/orgs/acme/actions/runners":
			writeJSON(t, w, map[string]any{"total_count": 0, "runners": []map[string]any{}})
		case "/orgs/acme/actions/secrets":
			writeJSON(t, w, map[string]any{"total_count": 0, "secrets": []map[string]any{}})
		case "/orgs/acme/installations":
			w.WriteHeader(http.StatusForbidden)
			writeJSON(t, w, map[string]any{"message": "Resource not accessible by token"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")
	inv, err := LiveCollector{}.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: t.TempDir(),
		Project:    "demo",
		Source:     "github-enterprise",
		Options: map[string]string{
			"owner":      "acme",
			"owner-type": "org",
			"base-url":   server.URL,
			"token-env":  "GITHUB_TOKEN",
		},
	})
	if err != nil {
		t.Fatalf("collect live github inventory: %v", err)
	}
	if len(inv.Assets.GitHubApps) != 0 {
		t.Fatalf("expected app inventory to be skipped: %+v", inv.Assets.GitHubApps)
	}
	if len(inv.Warnings) == 0 || !strings.Contains(strings.Join(inv.Warnings, "\n"), "collect organization GitHub Apps") {
		t.Fatalf("expected GitHub Apps warning, got %+v", inv.Warnings)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json response: %v", err)
	}
}

func writeGitHubContent(t *testing.T, w http.ResponseWriter, value string) {
	t.Helper()
	writeJSON(t, w, map[string]any{
		"type":     "file",
		"encoding": "base64",
		"content":  base64.StdEncoding.EncodeToString([]byte(value)),
	})
}

func containsAll(values []string, expected ...string) bool {
	joined := "\n" + strings.Join(values, "\n") + "\n"
	for _, value := range expected {
		if !strings.Contains(joined, "\n"+value+"\n") {
			return false
		}
	}
	return true
}
