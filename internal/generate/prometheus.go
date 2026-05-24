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
		conversion := convertDatadogMonitorQuery(monitor.Query)
		rules = append(rules, alertRule{
			Alert: promAlertName(monitor.Name),
			Expr:  conversion.Expr,
			For:   "5m",
			Labels: map[string]string{
				"severity":           "warning",
				"openexit_candidate": "true",
				"source":             "datadog",
				"source_monitor_id":  monitor.ID,
				"production_ready":   "false",
				"conversion":         conversion.Level,
			},
			Annotations: map[string]string{
				"summary":                monitor.Name,
				"openexit_manual_review": conversion.Review,
				"openexit_source_query":  monitor.Query,
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

type promConversion struct {
	Expr   string
	Review string
	Level  string
}

var simpleDatadogThresholdPattern = regexp.MustCompile(`(?i)^\s*(sum|avg|min|max)\(last_([0-9]+)([smhd])\):\s*(sum|avg|min|max|count):([a-zA-Z_:][a-zA-Z0-9_.:-]*)(\{([^}]*)\})?(\.(as_count|as_rate)\(\))?\s*([<>]=?|==|!=)\s*([0-9]+(?:\.[0-9]+)?)\s*$`)

func convertDatadogMonitorQuery(query string) promConversion {
	sourceComment := fmt.Sprintf("# Source Datadog query:\n# %s", sanitizeComment(query))
	lower := strings.ToLower(query)
	for _, unsupported := range []string{"anomal", "outlier", "forecast", "change(", "pct_change", "timeshift(", "default_zero(", "exclude_null("} {
		if strings.Contains(lower, unsupported) {
			return placeholderConversion(sourceComment, "complex Datadog function requires PromQL redesign")
		}
	}
	match := simpleDatadogThresholdPattern.FindStringSubmatch(query)
	if len(match) == 0 {
		return placeholderConversion(sourceComment, "Datadog query requires PromQL conversion and alert shadowing")
	}
	outerAgg := strings.ToLower(match[1])
	window := match[2] + strings.ToLower(match[3])
	metricAgg := strings.ToLower(match[4])
	metric := promMetricName(match[5])
	labels, ok := promLabels(match[7])
	if !ok || metric == "" {
		return placeholderConversion(sourceComment, "Datadog labels or metric name require manual PromQL conversion")
	}
	converter := strings.ToLower(match[9])
	operator := match[10]
	threshold := match[11]
	rangeExpr := metric + labels + "[" + window + "]"
	var vectorExpr string
	switch {
	case converter == "as_count":
		vectorExpr = "increase(" + rangeExpr + ")"
	case converter == "as_rate":
		vectorExpr = "rate(" + rangeExpr + ")"
	default:
		switch metricAgg {
		case "sum", "count":
			vectorExpr = "sum_over_time(" + rangeExpr + ")"
		case "min":
			vectorExpr = "min_over_time(" + rangeExpr + ")"
		case "max":
			vectorExpr = "max_over_time(" + rangeExpr + ")"
		default:
			vectorExpr = "avg_over_time(" + rangeExpr + ")"
		}
	}
	expr := fmt.Sprintf("# TODO(openexit): Review candidate PromQL before production use.\n%s\n%s(%s) %s %s", sourceComment, outerAgg, vectorExpr, operator, threshold)
	return promConversion{
		Expr:   expr,
		Review: "Simple Datadog threshold converted to a PromQL candidate. Review metric naming, label mapping, aggregation semantics, and alert shadowing before production use.",
		Level:  "simple-threshold",
	}
}

func placeholderConversion(sourceComment, reason string) promConversion {
	return promConversion{
		Expr:   fmt.Sprintf("# TODO(openexit): Manual PromQL review required.\n%s\nvector(0)", sourceComment),
		Review: reason + ".",
		Level:  "manual-review",
	}
}

func promMetricName(metric string) string {
	var b strings.Builder
	for _, r := range metric {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if ok {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	first := out[0]
	if (first >= '0' && first <= '9') || first == '_' {
		out = "datadog_" + out
	}
	return out
}

func promLabels(datadogTags string) (string, bool) {
	datadogTags = strings.TrimSpace(datadogTags)
	if datadogTags == "" {
		return "", true
	}
	parts := strings.Split(datadogTags, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, ":")
		if !ok || key == "" || value == "" || strings.ContainsAny(value, "*!") {
			return "", false
		}
		labels = append(labels, fmt.Sprintf(`%s="%s"`, promLabelName(key), escapePromLabelValue(value)))
	}
	if len(labels) == 0 {
		return "", true
	}
	return "{" + strings.Join(labels, ",") + "}", true
}

func promLabelName(label string) string {
	var b strings.Builder
	for _, r := range label {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if ok {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "label"
	}
	first := out[0]
	if first >= '0' && first <= '9' {
		out = "label_" + out
	}
	return out
}

func escapePromLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
