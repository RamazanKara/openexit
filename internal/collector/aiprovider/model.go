package aiprovider

type Fixture struct {
	Provider                  string                           `json:"provider"`
	Workspace                 string                           `json:"workspace"`
	ModelUsageClasses         []FixtureModelUsageClass         `json:"modelUsageClasses"`
	TokenVolumes              []FixtureTokenVolume             `json:"tokenVolumes"`
	LatencyExpectations       []FixtureLatencyExpectation      `json:"latencyExpectations"`
	SensitivePromptCategories []FixtureSensitivePromptCategory `json:"sensitivePromptCategories"`
	ToolUsages                []FixtureToolUsage               `json:"toolUsages"`
	FallbackBehaviors         []FixtureFallbackBehavior        `json:"fallbackBehaviors"`
}

type FixtureModelUsageClass struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	Provider                  string   `json:"provider"`
	Models                    []string `json:"models"`
	Purpose                   string   `json:"purpose"`
	Owners                    []string `json:"owners"`
	SensitivePromptCategories []string `json:"sensitivePromptCategories"`
	ToolRefs                  []string `json:"toolRefs"`
	FallbackBehavior          string   `json:"fallbackBehavior"`
}

type FixtureTokenVolume struct {
	ID                  string `json:"id"`
	UsageClassID        string `json:"usageClassId"`
	MonthlyInputTokens  int64  `json:"monthlyInputTokens"`
	MonthlyOutputTokens int64  `json:"monthlyOutputTokens"`
	PeakTokensPerMinute int    `json:"peakTokensPerMinute"`
	BurstWindow         string `json:"burstWindow"`
}

type FixtureLatencyExpectation struct {
	ID                string `json:"id"`
	UsageClassID      string `json:"usageClassId"`
	P50Ms             int    `json:"p50Ms"`
	P95Ms             int    `json:"p95Ms"`
	TimeoutMs         int    `json:"timeoutMs"`
	StreamingRequired bool   `json:"streamingRequired"`
}

type FixtureSensitivePromptCategory struct {
	ID                    string `json:"id"`
	UsageClassID          string `json:"usageClassId"`
	Category              string `json:"category"`
	DataClass             string `json:"dataClass"`
	ContainsPII           bool   `json:"containsPii"`
	RequiresDataResidency bool   `json:"requiresDataResidency"`
	Retention             string `json:"retention"`
}

type FixtureToolUsage struct {
	ID                    string `json:"id"`
	UsageClassID          string `json:"usageClassId"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	NetworkAccess         bool   `json:"networkAccess"`
	WritesExternalState   bool   `json:"writesExternalState"`
	HumanApprovalRequired bool   `json:"humanApprovalRequired"`
}

type FixtureFallbackBehavior struct {
	ID           string   `json:"id"`
	UsageClassID string   `json:"usageClassId"`
	Strategy     string   `json:"strategy"`
	Models       []string `json:"models"`
	ManualQueue  bool     `json:"manualQueue"`
	MaxRetries   int      `json:"maxRetries"`
}
