package datadogplan

import (
	"fmt"
	"regexp"
	"strings"
)

type queryConversion struct {
	Expr       string
	Window     string
	Operator   string
	Threshold  string
	ReasonCode string
	Review     string
	OK         bool
}

var (
	simpleMetricPattern    = regexp.MustCompile(`(?i)^\s*(avg|sum|min|max|count):([a-zA-Z_:][a-zA-Z0-9_.:-]*)(\{([^}]*)\})?(?:\s+by\s+\{([^}]*)\})?\s*$`)
	simpleThresholdPattern = regexp.MustCompile(`(?i)^\s*(sum|avg|min|max)\(last_([0-9]+)([smhd])\):\s*(sum|avg|min|max|count):([a-zA-Z_:][a-zA-Z0-9_.:-]*)(\{([^}]*)\})?(\.(as_count|as_rate)\(\))?\s*([<>]=?|==|!=)\s*([0-9]+(?:\.[0-9]+)?)\s*$`)
	nonIdentifierPattern   = regexp.MustCompile(`[^A-Za-z0-9]+`)
	promIdentifierPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	datadogTagKeyPattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	datadogTagValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.:/-]+$`)
)

func convertDashboardMetricQuery(query string) queryConversion {
	match := simpleMetricPattern.FindStringSubmatch(query)
	if len(match) == 0 {
		return queryConversion{ReasonCode: "query.datadog-syntax", Review: "Datadog query is outside the deterministic metric subset and requires manual PromQL or LogQL conversion."}
	}
	agg := strings.ToLower(match[1])
	metric := promMetricName(match[2])
	labels, ok := promLabels(match[4])
	if !ok || metric == "" {
		return queryConversion{ReasonCode: "query.label-mapping", Review: "Metric name or tag filters require manual label mapping."}
	}
	groups, ok := promGroupLabels(match[5])
	if !ok {
		return queryConversion{ReasonCode: "query.group-mapping", Review: "Datadog group-by tags require manual label mapping."}
	}
	expr := agg + "(" + metric + labels + ")"
	if len(groups) > 0 {
		expr = agg + " by (" + strings.Join(groups, ",") + ") (" + metric + labels + ")"
	}
	return queryConversion{
		Expr:       expr,
		ReasonCode: "query.promql-approximation",
		Review:     "Review metric naming, tag-to-label mapping, aggregation, rollup, and missing-data semantics.",
		OK:         true,
	}
}

func convertMonitorQuery(query string) queryConversion {
	lower := strings.ToLower(query)
	for _, unsupported := range []string{"anomal", "outlier", "forecast", "change(", "pct_change", "timeshift(", "default_zero(", "exclude_null(", "composite"} {
		if strings.Contains(lower, unsupported) {
			return queryConversion{ReasonCode: "monitor.complex-function", Review: "Complex Datadog monitor behavior requires a manual Prometheus alert design."}
		}
	}
	match := simpleThresholdPattern.FindStringSubmatch(query)
	if len(match) == 0 {
		return queryConversion{ReasonCode: "monitor.query-unsupported", Review: "Monitor query is outside the deterministic static-threshold subset."}
	}
	outerAgg := strings.ToLower(match[1])
	window := match[2] + strings.ToLower(match[3])
	metricAgg := strings.ToLower(match[4])
	metric := promMetricName(match[5])
	labels, ok := promLabels(match[7])
	if !ok || metric == "" {
		return queryConversion{ReasonCode: "monitor.label-mapping", Review: "Metric name or Datadog tag filters require manual Prometheus label mapping."}
	}
	converter := strings.ToLower(match[9])
	operator := match[10]
	threshold := match[11]
	rangeExpr := metric + labels + "[" + window + "]"
	var vectorExpr string
	switch converter {
	case "as_count":
		vectorExpr = "increase(" + rangeExpr + ")"
	case "as_rate":
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
	return queryConversion{
		Expr:       fmt.Sprintf("%s(%s) %s %s", outerAgg, vectorExpr, operator, threshold),
		Window:     window,
		Operator:   operator,
		Threshold:  threshold,
		ReasonCode: "monitor.promql-approximation",
		Review:     "Review metric naming, label mapping, aggregation, evaluation delay, no-data behavior, notification routing, and alert shadowing.",
		OK:         true,
	}
}

func promMetricName(metric string) string {
	var builder strings.Builder
	for _, r := range metric {
		valid := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if valid {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return ""
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = "datadog_" + result
	}
	return result
}

func promLabels(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return "", true
	}
	var labels []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, ":")
		if !ok || !datadogTagKeyPattern.MatchString(key) || !datadogTagValuePattern.MatchString(value) {
			return "", false
		}
		labels = append(labels, fmt.Sprintf(`%s="%s"`, promLabelName(key), escapePromLabelValue(value)))
	}
	if len(labels) == 0 {
		return "", true
	}
	labels = SortedUnique(labels)
	return "{" + strings.Join(labels, ",") + "}", true
}

func promGroupLabels(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	var labels []string
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if !datadogTagKeyPattern.MatchString(value) {
			return nil, false
		}
		labels = append(labels, promLabelName(value))
	}
	return SortedUnique(labels), true
}

func promLabelName(value string) string {
	value = promMetricName(value)
	if value == "" {
		return "label"
	}
	return value
}

func escapePromLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func promAlertName(name, sourceRef string) string {
	parts := nonIdentifierPattern.Split(name, -1)
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			builder.WriteString(part[1:])
		}
	}
	if builder.Len() == 0 {
		builder.WriteString("DatadogMonitor")
	}
	if first := builder.String()[0]; first >= '0' && first <= '9' {
		return "DatadogMonitor" + builder.String() + "Candidate_" + DigestBytes([]byte(sourceRef))[:8]
	}
	return builder.String() + "Candidate_" + DigestBytes([]byte(sourceRef))[:8]
}
