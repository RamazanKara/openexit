package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
	openmapping "github.com/RamazanKara/openexit/internal/mapping"
	"gopkg.in/yaml.v3"
)

type prometheusRuleFile struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

type prometheusRuleGroup struct {
	Name  string                `yaml:"name"`
	Rules []prometheusAlertRule `yaml:"rules"`
}

type prometheusAlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func addPrometheusRuleCandidateChecks(projectDir string, inv *inventory.Inventory, result *openmapping.MappingResult, add func(string, string, string, bool)) {
	if inv == nil || result == nil {
		return
	}
	if len(result.AlertRuleDrafts) == 0 {
		return
	}
	path := filepath.Join(projectDir, "generated-config", "prometheus", "rules", "datadog-monitor-candidates.yaml")
	rules, err := readPrometheusRuleCandidates(path)
	if err != nil {
		add("prometheus-rule-candidates", "failed", err.Error(), true)
		return
	}
	monitorsByID := map[string]inventory.Monitor{}
	for _, monitor := range inv.Assets.Monitors {
		monitorsByID[monitor.ID] = monitor
	}
	var problems []string
	problems = append(problems, validatePrometheusRuleSetShape(rules)...)
	problems = append(problems, validatePrometheusRuleMapping(result, monitorsByID, rules)...)
	problems = append(problems, validatePrometheusRuleREADME(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("prometheus-rule-candidates", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("prometheus-rule-candidates", "passed", "", true)
}

func readPrometheusRuleCandidates(path string) (*prometheusRuleFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules prometheusRuleFile
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return &rules, nil
}

func validatePrometheusRuleSetShape(rules *prometheusRuleFile) []string {
	var problems []string
	if rules == nil {
		return []string{"Prometheus rule file is empty"}
	}
	if len(rules.Groups) == 0 {
		return []string{"Prometheus rule file requires at least one group"}
	}
	for i, group := range rules.Groups {
		groupName := fmt.Sprintf("group %d", i+1)
		if group.Name == "" {
			problems = append(problems, groupName+": name is required")
		}
		if len(group.Rules) == 0 {
			problems = append(problems, groupName+": at least one alert rule is required")
		}
		for j, rule := range group.Rules {
			ruleName := fmt.Sprintf("%s rule %d", groupName, j+1)
			problems = append(problems, validatePrometheusAlertRuleShape(ruleName, rule)...)
		}
	}
	return problems
}

func validatePrometheusAlertRuleShape(ruleName string, rule prometheusAlertRule) []string {
	var problems []string
	if rule.Alert == "" {
		problems = append(problems, ruleName+": alert is required")
	}
	if strings.TrimSpace(rule.Expr) == "" {
		problems = append(problems, ruleName+": expr is required")
	}
	if rule.For == "" {
		problems = append(problems, ruleName+": for is required")
	}
	if rule.Labels == nil {
		problems = append(problems, ruleName+": labels are required")
	}
	if rule.Annotations == nil {
		problems = append(problems, ruleName+": annotations are required")
	}
	return problems
}

func validatePrometheusRuleMapping(result *openmapping.MappingResult, monitorsByID map[string]inventory.Monitor, rules *prometheusRuleFile) []string {
	rulesBySourceMonitor := map[string]prometheusAlertRule{}
	var problems []string
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			sourceID := rule.Labels["source_monitor_id"]
			if sourceID == "" {
				problems = append(problems, fmt.Sprintf("%s: labels.source_monitor_id is required", rule.Alert))
				continue
			}
			if _, exists := rulesBySourceMonitor[sourceID]; exists {
				problems = append(problems, fmt.Sprintf("%s: duplicate rule for source monitor %s", rule.Alert, sourceID))
			}
			rulesBySourceMonitor[sourceID] = rule
			monitor, ok := monitorsByID[sourceID]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: source monitor %s is not present in inventory", rule.Alert, sourceID))
				continue
			}
			problems = append(problems, validatePrometheusAlertRuleSemantics(monitor, rule)...)
		}
	}
	for _, draft := range result.AlertRuleDrafts {
		if _, ok := monitorsByID[draft.SourceID]; !ok {
			problems = append(problems, fmt.Sprintf("%s: mapping references missing monitor %s", draft.Alert, draft.SourceID))
			continue
		}
		rule, ok := rulesBySourceMonitor[draft.SourceID]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: no generated Prometheus rule for monitor %s", draft.Alert, draft.SourceID))
			continue
		}
		if rule.Alert != draft.Alert {
			problems = append(problems, fmt.Sprintf("%s: generated alert %q does not match mapping alert %q", draft.SourceID, rule.Alert, draft.Alert))
		}
		if filepath.ToSlash(draft.Path) != "generated-config/prometheus/rules/datadog-monitor-candidates.yaml" {
			problems = append(problems, fmt.Sprintf("%s: unexpected mapping path %s", draft.Alert, draft.Path))
		}
	}
	return problems
}

func validatePrometheusAlertRuleSemantics(monitor inventory.Monitor, rule prometheusAlertRule) []string {
	var problems []string
	prefix := rule.Alert
	expectedLabels := map[string]string{
		"openexit_candidate": "true",
		"source":             "datadog",
		"source_monitor_id":  monitor.ID,
		"production_ready":   "false",
	}
	for key, want := range expectedLabels {
		if got := rule.Labels[key]; got != want {
			problems = append(problems, fmt.Sprintf("%s: labels.%s must be %q, got %q", prefix, key, want, got))
		}
	}
	if conversion := rule.Labels["conversion"]; conversion != "simple-threshold" && conversion != "manual-review" {
		problems = append(problems, fmt.Sprintf("%s: labels.conversion must be simple-threshold or manual-review, got %q", prefix, conversion))
	}
	expectedAnnotations := map[string]string{
		"summary":               monitor.Name,
		"openexit_source_query": monitor.Query,
	}
	for key, want := range expectedAnnotations {
		if got := rule.Annotations[key]; got != want {
			problems = append(problems, fmt.Sprintf("%s: annotations.%s must preserve source value", prefix, key))
		}
	}
	if strings.TrimSpace(rule.Annotations["openexit_manual_review"]) == "" {
		problems = append(problems, fmt.Sprintf("%s: annotations.openexit_manual_review is required", prefix))
	}
	if !strings.Contains(rule.Expr, "# Source Datadog query:") {
		problems = append(problems, fmt.Sprintf("%s: expr must preserve source query as a comment", prefix))
	}
	if !strings.Contains(rule.Expr, monitor.Query) {
		problems = append(problems, fmt.Sprintf("%s: expr must include the source Datadog query", prefix))
	}
	if rule.Labels["conversion"] == "manual-review" {
		if !strings.Contains(rule.Expr, "vector(0)") {
			problems = append(problems, fmt.Sprintf("%s: manual-review conversion must use vector(0) placeholder", prefix))
		}
		if !strings.Contains(strings.ToLower(rule.Expr), "manual promql review") {
			problems = append(problems, fmt.Sprintf("%s: manual-review conversion must include manual PromQL review guidance", prefix))
		}
	}
	if rule.Labels["conversion"] == "simple-threshold" {
		if !strings.Contains(strings.ToLower(rule.Expr), "review candidate promql") {
			problems = append(problems, fmt.Sprintf("%s: simple-threshold conversion must include candidate review guidance", prefix))
		}
	}
	return problems
}

func validatePrometheusRuleREADME(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "prometheus", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/prometheus/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"generated candidates", "source monitor IDs", "manual PromQL review"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/prometheus/README.md: missing %q", required)}
		}
	}
	return nil
}
