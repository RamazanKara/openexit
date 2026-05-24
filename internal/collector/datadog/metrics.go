package datadog

type FixtureMetric struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
