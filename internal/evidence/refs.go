package evidence

import (
	"fmt"
	"path/filepath"
	"strings"
)

func PathForRef(projectDir, ref string) (string, error) {
	const prefix = "evidence://"
	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("unsupported evidence ref %q", ref)
	}
	rest := strings.TrimPrefix(ref, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid evidence ref %q", ref)
	}
	source, kind, id := parts[0], parts[1], parts[2]
	var dir string
	switch kind {
	case "dashboard":
		dir = "dashboards"
	case "monitor":
		dir = "monitors"
	case "slo":
		dir = "slos"
	case "integration":
		dir = "integrations"
	case "repository":
		dir = "repositories"
	case "team":
		dir = "teams"
	case "branch-protection":
		dir = "branch-protections"
	case "actions-workflow":
		dir = "actions-workflows"
	case "secret":
		dir = "secrets"
	case "runner":
		dir = "runners"
	case "deploy-key":
		dir = "deploy-keys"
	case "github-app":
		dir = "github-apps"
	case "application":
		dir = "applications"
	case "group":
		dir = "groups"
	case "policy":
		dir = "policies"
	case "mfa-setting":
		dir = "mfa-settings"
	case "break-glass-account":
		dir = "break-glass-accounts"
	case "dns-record":
		dir = "dns-records"
	case "waf-rule":
		dir = "waf-rules"
	case "cache-rule":
		dir = "cache-rules"
	case "redirect":
		dir = "redirects"
	case "origin":
		dir = "origins"
	case "tls-setting":
		dir = "tls-settings"
	case "bot-rule":
		dir = "bot-rules"
	case "page-rule":
		dir = "page-rules"
	case "model-usage-class":
		dir = "model-usage-classes"
	case "token-volume":
		dir = "token-volumes"
	case "latency-expectation":
		dir = "latency-expectations"
	case "sensitive-prompt-category":
		dir = "sensitive-prompt-categories"
	case "ai-tool-usage":
		dir = "tool-usages"
	case "fallback-behavior":
		dir = "fallback-behaviors"
	default:
		dir = kind
	}
	return filepath.Join(projectDir, "evidence", source, dir, slugPath(id)+".json"), nil
}

func slugPath(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unnamed"
	}
	return out
}
