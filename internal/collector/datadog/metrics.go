package datadog

type FixtureMetric struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type FixtureVolumes struct {
	LogVolumeKnown   bool `json:"logVolumeKnown"`
	TraceVolumeKnown bool `json:"traceVolumeKnown"`
}
