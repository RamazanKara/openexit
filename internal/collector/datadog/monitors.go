package datadog

type FixtureMonitor struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Query               string   `json:"query"`
	Message             string   `json:"message"`
	NotificationTargets []string `json:"notificationTargets"`
	Tags                []string `json:"tags"`
	RunbookURL          string   `json:"runbookUrl"`
}
