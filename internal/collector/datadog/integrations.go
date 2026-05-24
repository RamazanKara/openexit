package datadog

type FixtureIntegration struct {
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Tags    []string `json:"tags"`
}
