package datadog

import (
	"regexp"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type FixtureMetric struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type FixtureVolumes struct {
	LogVolumeKnown   bool `json:"logVolumeKnown"`
	TraceVolumeKnown bool `json:"traceVolumeKnown"`
}

var datadogTagScopePattern = regexp.MustCompile(`\{([^{}]*)\}`)

func recordQueryMetricRefs(metricNames map[string]inventory.MetricRef, query string) {
	tags := datadogQueryTags(query)
	for _, name := range inventory.ExtractMetricNames(query) {
		metric := metricNames[name]
		if metric.Name == "" {
			metric.Name = name
		}
		metric.Tags = uniqueSorted(append(metric.Tags, tags...))
		metricNames[name] = metric
	}
}

func recordFixtureMetricRef(metricNames map[string]inventory.MetricRef, metric FixtureMetric) {
	name := strings.TrimSpace(metric.Name)
	if name == "" {
		return
	}
	current := metricNames[name]
	if current.Name == "" {
		current.Name = name
	}
	current.Tags = uniqueSorted(append(current.Tags, metric.Tags...))
	metricNames[name] = current
}

func recordInventoryQueryMetricRefs(inv *inventory.Inventory, query string) {
	metricNames := map[string]inventory.MetricRef{}
	for _, metric := range inv.Assets.Metrics {
		recordFixtureMetricRef(metricNames, FixtureMetric{Name: metric.Name, Tags: metric.Tags})
	}
	recordQueryMetricRefs(metricNames, query)
	inv.Assets.Metrics = metricRefsFromMap(metricNames)
}

func metricRefsFromMap(metricNames map[string]inventory.MetricRef) []inventory.MetricRef {
	out := make([]inventory.MetricRef, 0, len(metricNames))
	for _, metric := range metricNames {
		out = append(out, metric)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func datadogQueryTags(query string) []string {
	seen := map[string]struct{}{}
	for _, match := range datadogTagScopePattern.FindAllStringSubmatch(query, -1) {
		if len(match) < 2 {
			continue
		}
		for _, part := range strings.Split(match[1], ",") {
			tag := strings.TrimSpace(part)
			tag = strings.TrimPrefix(tag, "!")
			tag = strings.TrimPrefix(tag, "not ")
			tag = strings.TrimSpace(tag)
			if tag == "" || tag == "*" {
				continue
			}
			if idx := strings.Index(tag, ":"); idx >= 0 {
				tag = tag[:idx]
			}
			tag = strings.TrimSpace(strings.TrimPrefix(tag, "$"))
			if tag != "" {
				seen[tag] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}
