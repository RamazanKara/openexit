package datadog

import "sort"

type Fixture struct {
	Site         string               `json:"site"`
	Dashboards   []FixtureDashboard   `json:"dashboards"`
	Monitors     []FixtureMonitor     `json:"monitors"`
	SLOs         []FixtureSLO         `json:"slos"`
	Integrations []FixtureIntegration `json:"integrations"`
	Metrics      []FixtureMetric      `json:"metrics"`
}

type FixtureDashboard struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	URL     string          `json:"url"`
	Tags    []string        `json:"tags"`
	Widgets []FixtureWidget `json:"widgets"`
	Queries []string        `json:"queries"`
}

type FixtureWidget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Query string `json:"query"`
}

func (d FixtureDashboard) WidgetTypes() []string {
	seen := map[string]struct{}{}
	for _, widget := range d.Widgets {
		if widget.Type != "" {
			seen[widget.Type] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}
