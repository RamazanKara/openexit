package datadog

type FixtureSLO struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Target             float64  `json:"target"`
	Timeframe          string   `json:"timeframe"`
	SLI                string   `json:"sli"`
	BurnRateMonitorIDs []string `json:"burnRateMonitorIds"`
	DashboardRefs      []string `json:"dashboardRefs"`
}
