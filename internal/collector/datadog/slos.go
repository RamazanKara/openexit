package datadog

type FixtureSLO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Target    float64 `json:"target"`
	Timeframe string  `json:"timeframe"`
}
