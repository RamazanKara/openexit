package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type ruleFile struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string      `yaml:"name"`
	Rules []alertRule `yaml:"rules"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func Prometheus(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "prometheus", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var rules []alertRule
	for _, monitor := range ctx.Inventory.Assets.Monitors {
		rules = append(rules, alertRule{
			Alert: promAlertName(monitor.Name),
			Expr:  fmt.Sprintf("# TODO(openexit): Manual PromQL review required.\n# Source Datadog query:\n# %s\nvector(0)", sanitizeComment(monitor.Query)),
			For:   "5m",
			Labels: map[string]string{
				"severity":           "warning",
				"openexit_candidate": "true",
				"source":             "datadog",
				"source_monitor_id":  monitor.ID,
				"production_ready":   "false",
			},
			Annotations: map[string]string{
				"summary":                monitor.Name,
				"openexit_manual_review": "Datadog query requires PromQL conversion and alert shadowing.",
			},
		})
	}
	data, err := yaml.Marshal(ruleFile{Groups: []ruleGroup{{Name: "openexit-datadog-monitor-candidates", Rules: rules}}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "datadog-monitor-candidates.yaml"), data, 0o644); err != nil {
		return err
	}
	readme := "# Prometheus Rule Candidates\n\nThese alerts are generated candidates. They preserve source monitor IDs and require manual PromQL review before production use.\n"
	return os.WriteFile(filepath.Join(ctx.ProjectDir, "generated-config", "prometheus", "README.md"), []byte(readme), 0o644)
}

var alertNamePattern = regexp.MustCompile(`[^A-Za-z0-9]+`)

func promAlertName(name string) string {
	parts := alertNamePattern.Split(name, -1)
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.Title(strings.ToLower(part)))
	}
	if b.Len() == 0 {
		return "DatadogMonitorCandidate"
	}
	return b.String() + "Candidate"
}

func sanitizeComment(input string) string {
	return strings.ReplaceAll(input, "\n", " ")
}
