package evidence

import (
	"path/filepath"
	"testing"
)

func TestPathForRef(t *testing.T) {
	tests := []struct {
		kind string
		dir  string
	}{
		{"dashboard", "dashboards"},
		{"monitor", "monitors"},
		{"slo", "slos"},
		{"integration", "integrations"},
		{"repository", "repositories"},
		{"team", "teams"},
		{"branch-protection", "branch-protections"},
		{"actions-workflow", "actions-workflows"},
		{"secret", "secrets"},
		{"runner", "runners"},
		{"deploy-key", "deploy-keys"},
		{"github-app", "github-apps"},
		{"application", "applications"},
		{"group", "groups"},
		{"policy", "policies"},
		{"mfa-setting", "mfa-settings"},
		{"break-glass-account", "break-glass-accounts"},
		{"dns-record", "dns-records"},
		{"waf-rule", "waf-rules"},
		{"cache-rule", "cache-rules"},
		{"redirect", "redirects"},
		{"origin", "origins"},
		{"tls-setting", "tls-settings"},
		{"bot-rule", "bot-rules"},
		{"page-rule", "page-rules"},
		{"model-usage-class", "model-usage-classes"},
		{"token-volume", "token-volumes"},
		{"latency-expectation", "latency-expectations"},
		{"sensitive-prompt-category", "sensitive-prompt-categories"},
		{"ai-tool-usage", "tool-usages"},
		{"fallback-behavior", "fallback-behaviors"},
		{"custom-kind", "custom-kind"},
	}
	project := t.TempDir()
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got, err := PathForRef(project, "evidence://source/"+tt.kind+"/My Resource:123")
			want := filepath.Join(project, "evidence", "source", tt.dir, "my-resource-123.json")
			if err != nil || got != want {
				t.Fatalf("PathForRef = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestPathForRefRejectsMalformedPaths(t *testing.T) {
	project := t.TempDir()
	for _, ref := range []string{
		"", "file://datadog/monitor/id", "evidence://datadog/monitor",
		"evidence://datadog/monitor/", "evidence:///monitor/id",
		"evidence://datadog//id", "evidence://datadog/monitor/id/extra",
		"evidence://../monitor/id", "evidence://./monitor/id",
		"evidence://datadog/../id", "evidence://datadog/./id",
		"evidence://.. /monitor/id", "evidence://datadog/.../id",
		`evidence://..\../monitor/id`, `evidence://datadog/..\../id`,
		"evidence://C:/monitor/id", "evidence://datadog/C:/id",
	} {
		t.Run(ref, func(t *testing.T) {
			if path, err := PathForRef(project, ref); err == nil || path != "" {
				t.Fatalf("accepted invalid ref %q: %q, %v", ref, path, err)
			}
		})
	}
}
