package inventory

import (
	"regexp"
	"sort"
	"strings"
)

var metricPattern = regexp.MustCompile(`(?:avg|sum|min|max|count|p[0-9]+|pct|median|rate):([a-zA-Z_][a-zA-Z0-9_.-]*)`)

func ExtractMetricNames(query string) []string {
	seen := map[string]struct{}{}
	for _, match := range metricPattern.FindAllStringSubmatch(query, -1) {
		if len(match) > 1 {
			seen[match[1]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for metric := range seen {
		out = append(out, metric)
	}
	sort.Strings(out)
	return out
}

func HasOwnerTag(tags []string) bool {
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if strings.HasPrefix(tag, "team:") || strings.HasPrefix(tag, "owner:") || strings.HasPrefix(tag, "service:") {
			return true
		}
	}
	return false
}

func Slug(value string) string {
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
