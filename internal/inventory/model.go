package inventory

import "time"

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "Inventory"
)

type Inventory struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Source     Source   `json:"source" yaml:"source"`
	Assets     Assets   `json:"assets" yaml:"assets"`
	Summary    Summary  `json:"summary" yaml:"summary"`
	Warnings   []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type Metadata struct {
	Project          string    `json:"project" yaml:"project"`
	Source           string    `json:"source" yaml:"source"`
	CollectedAt      time.Time `json:"collectedAt" yaml:"collectedAt"`
	CollectorVersion string    `json:"collectorVersion" yaml:"collectorVersion"`
}

type Source struct {
	Type string `json:"type" yaml:"type"`
	Site string `json:"site,omitempty" yaml:"site,omitempty"`
}

type Assets struct {
	Dashboards   []Dashboard   `json:"dashboards,omitempty" yaml:"dashboards,omitempty"`
	Monitors     []Monitor     `json:"monitors,omitempty" yaml:"monitors,omitempty"`
	SLOs         []SLO         `json:"slos,omitempty" yaml:"slos,omitempty"`
	Integrations []Integration `json:"integrations,omitempty" yaml:"integrations,omitempty"`
	Metrics      []MetricRef   `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

type Dashboard struct {
	ID          string        `json:"id" yaml:"id"`
	Title       string        `json:"title" yaml:"title"`
	URL         string        `json:"url,omitempty" yaml:"url,omitempty"`
	Tags        []string      `json:"tags,omitempty" yaml:"tags,omitempty"`
	Widgets     WidgetSummary `json:"widgets" yaml:"widgets"`
	Queries     []Query       `json:"queries,omitempty" yaml:"queries,omitempty"`
	EvidenceRef string        `json:"evidenceRef" yaml:"evidenceRef"`
}

type WidgetSummary struct {
	Total       int      `json:"total" yaml:"total"`
	Unsupported int      `json:"unsupported" yaml:"unsupported"`
	Types       []string `json:"types,omitempty" yaml:"types,omitempty"`
}

type Query struct {
	ID       string `json:"id" yaml:"id"`
	Language string `json:"language" yaml:"language"`
	Raw      string `json:"raw" yaml:"raw"`
}

type Monitor struct {
	ID                  string   `json:"id" yaml:"id"`
	Name                string   `json:"name" yaml:"name"`
	Type                string   `json:"type" yaml:"type"`
	Query               string   `json:"query" yaml:"query"`
	NotificationTargets []string `json:"notificationTargets,omitempty" yaml:"notificationTargets,omitempty"`
	Tags                []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	RunbookURL          string   `json:"runbookUrl,omitempty" yaml:"runbookUrl,omitempty"`
	EvidenceRef         string   `json:"evidenceRef" yaml:"evidenceRef"`
}

type SLO struct {
	ID          string  `json:"id" yaml:"id"`
	Name        string  `json:"name" yaml:"name"`
	Target      float64 `json:"target" yaml:"target"`
	Timeframe   string  `json:"timeframe" yaml:"timeframe"`
	EvidenceRef string  `json:"evidenceRef" yaml:"evidenceRef"`
}

type Integration struct {
	Name        string   `json:"name" yaml:"name"`
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	EvidenceRef string   `json:"evidenceRef,omitempty" yaml:"evidenceRef,omitempty"`
}

type MetricRef struct {
	Name string   `json:"name" yaml:"name"`
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Summary struct {
	Dashboards          int `json:"dashboards" yaml:"dashboards"`
	Monitors            int `json:"monitors" yaml:"monitors"`
	SLOs                int `json:"slos" yaml:"slos"`
	Integrations        int `json:"integrations" yaml:"integrations"`
	UniqueMetrics       int `json:"uniqueMetrics" yaml:"uniqueMetrics"`
	NotificationTargets int `json:"notificationTargets" yaml:"notificationTargets"`
}

func New(project, source, site, collectorVersion string, collectedAt time.Time) *Inventory {
	return &Inventory{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Project:          project,
			Source:           source,
			CollectedAt:      collectedAt.UTC(),
			CollectorVersion: collectorVersion,
		},
		Source: Source{
			Type: source,
			Site: site,
		},
	}
}

func (inv *Inventory) RecomputeSummary() {
	metricNames := map[string]struct{}{}
	targets := map[string]struct{}{}
	for _, metric := range inv.Assets.Metrics {
		metricNames[metric.Name] = struct{}{}
	}
	for _, dashboard := range inv.Assets.Dashboards {
		for _, query := range dashboard.Queries {
			for _, metric := range ExtractMetricNames(query.Raw) {
				metricNames[metric] = struct{}{}
			}
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		for _, metric := range ExtractMetricNames(monitor.Query) {
			metricNames[metric] = struct{}{}
		}
		for _, target := range monitor.NotificationTargets {
			targets[target] = struct{}{}
		}
	}
	inv.Summary = Summary{
		Dashboards:          len(inv.Assets.Dashboards),
		Monitors:            len(inv.Assets.Monitors),
		SLOs:                len(inv.Assets.SLOs),
		Integrations:        len(inv.Assets.Integrations),
		UniqueMetrics:       len(metricNames),
		NotificationTargets: len(targets),
	}
}
