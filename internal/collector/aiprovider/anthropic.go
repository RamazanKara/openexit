package aiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type AnthropicCollector struct{}

func (AnthropicCollector) Name() string { return "anthropic" }

func (AnthropicCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	tokenEnv := option(req.Options, "admin-key-env", "ANTHROPIC_ADMIN_KEY")
	token, err := envSecret(tokenEnv)
	if err != nil {
		return nil, err
	}
	days, err := positiveIntOption(req.Options, "days", 30)
	if err != nil {
		return nil, err
	}
	peakDays, err := positiveIntOption(req.Options, "peak-days", 7)
	if err != nil {
		return nil, err
	}
	client, err := NewAnthropicClient(req.Options["base-url"], token, option(req.Options, "anthropic-version", "2023-06-01"))
	if err != nil {
		return nil, err
	}

	var warnings []string
	now := time.Now().UTC().Truncate(time.Hour)
	usageStart := now.AddDate(0, 0, -days)
	usageBuckets, err := collectAnthropicMessagesUsage(ctx, client, anthropicUsageRequest{
		Start:        usageStart,
		End:          now,
		BucketWidth:  "1d",
		WorkspaceIDs: splitOptionList(req.Options["workspace-ids"]),
		APIKeyIDs:    splitOptionList(req.Options["api-key-ids"]),
		Models:       splitOptionList(req.Options["models"]),
	})
	if err != nil {
		return nil, err
	}
	peakStart := now.AddDate(0, 0, -peakDays)
	peakBuckets, err := collectAnthropicMessagesUsage(ctx, client, anthropicUsageRequest{
		Start:        peakStart,
		End:          now,
		BucketWidth:  "1h",
		WorkspaceIDs: splitOptionList(req.Options["workspace-ids"]),
		APIKeyIDs:    splitOptionList(req.Options["api-key-ids"]),
		Models:       splitOptionList(req.Options["models"]),
	})
	if err != nil {
		warnings = append(warnings, optionalWarning("collect hourly Anthropic usage for peak estimate", err))
	}
	if days != 30 {
		warnings = append(warnings, fmt.Sprintf("Anthropic token volume fields reflect a %d-day collection window, not a normalized calendar month.", days))
	}
	warnings = append(warnings, "Anthropic Usage and Cost Admin APIs expose aggregate usage metadata only; latency, prompt sensitivity, tool use, and fallback behavior should be supplied from application owners when not passed as collector flags.")

	workspace := option(req.Options, "workspace", "organization")
	fixture := fixtureFromAnthropicUsage(anthropicUsageOptions{
		Workspace:           workspace,
		Owners:              splitOptionList(req.Options["owners"]),
		UsageBuckets:        usageBuckets,
		PeakBuckets:         peakBuckets,
		DefaultP95Ms:        intOption(req.Options, "default-p95-ms", 0),
		DefaultTimeoutMs:    intOption(req.Options, "default-timeout-ms", 0),
		StreamingRequired:   boolOption(req.Options, "streaming-required"),
		FallbackStrategy:    strings.TrimSpace(req.Options["fallback-strategy"]),
		FallbackManualQueue: boolOption(req.Options, "fallback-manual-queue"),
		FallbackMaxRetries:  intOption(req.Options, "fallback-max-retries", 0),
		PeakDays:            peakDays,
	})
	if len(fixture.ModelUsageClasses) == 0 {
		warnings = append(warnings, "Anthropic usage API returned no model-grouped messages usage for the selected window.")
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "ai-provider", "anthropic/"+workspace, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "ai-provider/raw-anthropic-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type AnthropicClient struct {
	baseURL string
	token   string
	version string
	http    *http.Client
}

type anthropicAPIError struct {
	Path       string
	Status     string
	StatusCode int
	Body       string
}

func (e *anthropicAPIError) Error() string {
	return fmt.Sprintf("anthropic API %s returned %s: %s", e.Path, e.Status, e.Body)
}

func NewAnthropicClient(baseURL, token, apiVersion string) (*AnthropicClient, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("anthropic admin key is required")
	}
	normalized, err := anthropicBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	apiVersion = strings.TrimSpace(apiVersion)
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}
	return &AnthropicClient{
		baseURL: normalized,
		token:   token,
		version: apiVersion,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func anthropicBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "https://api.anthropic.com"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !anthropicLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("anthropic base URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("anthropic base URL requires a host")
	}
	return strings.TrimRight(raw, "/"), nil
}

func anthropicLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *AnthropicClient) get(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	endpoint := c.baseURL + path
	if query != nil {
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", version.Name+"/"+version.Version)
		req.Header.Set("anthropic-version", c.version)
		req.Header.Set("x-api-key", c.token)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &anthropicAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
			time.Sleep(retryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &anthropicAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
		}
		if out != nil {
			if err := json.Unmarshal(body, out); err != nil {
				return nil, err
			}
		}
		return body, nil
	}
	return nil, lastErr
}

type anthropicUsageRequest struct {
	Start        time.Time
	End          time.Time
	BucketWidth  string
	WorkspaceIDs []string
	APIKeyIDs    []string
	Models       []string
}

type anthropicUsageResponse struct {
	Data     []anthropicUsageBucket `json:"data"`
	HasMore  bool                   `json:"has_more"`
	NextPage string                 `json:"next_page"`
}

type anthropicUsageBucket struct {
	StartingAt string                 `json:"starting_at"`
	EndingAt   string                 `json:"ending_at"`
	Results    []anthropicUsageResult `json:"results"`
}

type anthropicUsageResult struct {
	UncachedInputTokens int64 `json:"uncached_input_tokens"`
	CacheCreation       struct {
		Ephemeral1HInputTokens int64 `json:"ephemeral_1h_input_tokens"`
		Ephemeral5MInputTokens int64 `json:"ephemeral_5m_input_tokens"`
	} `json:"cache_creation"`
	CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	ServerToolUse        struct {
		WebSearchRequests int64 `json:"web_search_requests"`
	} `json:"server_tool_use"`
	APIKeyID      string `json:"api_key_id"`
	WorkspaceID   string `json:"workspace_id"`
	Model         string `json:"model"`
	ServiceTier   string `json:"service_tier"`
	ContextWindow string `json:"context_window"`
	InferenceGeo  string `json:"inference_geo"`
}

type anthropicTokenTotals struct {
	InputTokens       int64
	OutputTokens      int64
	WebSearchRequests int64
}

type anthropicUsageOptions struct {
	Workspace           string
	Owners              []string
	UsageBuckets        []anthropicUsageBucket
	PeakBuckets         []anthropicUsageBucket
	DefaultP95Ms        int
	DefaultTimeoutMs    int
	StreamingRequired   bool
	FallbackStrategy    string
	FallbackManualQueue bool
	FallbackMaxRetries  int
	PeakDays            int
}

func collectAnthropicMessagesUsage(ctx context.Context, client *AnthropicClient, req anthropicUsageRequest) ([]anthropicUsageBucket, error) {
	query := url.Values{}
	query.Set("starting_at", req.Start.Format(time.RFC3339))
	query.Set("ending_at", req.End.Format(time.RFC3339))
	query.Set("bucket_width", req.BucketWidth)
	query.Set("limit", anthropicUsageLimit(req.BucketWidth))
	query.Add("group_by[]", "model")
	for _, value := range req.WorkspaceIDs {
		query.Add("workspace_ids[]", value)
	}
	for _, value := range req.APIKeyIDs {
		query.Add("api_key_ids[]", value)
	}
	for _, value := range req.Models {
		query.Add("models[]", value)
	}

	var buckets []anthropicUsageBucket
	for {
		var response anthropicUsageResponse
		if _, err := client.get(ctx, "/v1/organizations/usage_report/messages", query, &response); err != nil {
			return nil, err
		}
		buckets = append(buckets, response.Data...)
		if !response.HasMore || strings.TrimSpace(response.NextPage) == "" {
			break
		}
		query.Set("page", response.NextPage)
	}
	return buckets, nil
}

func anthropicUsageLimit(bucketWidth string) string {
	switch bucketWidth {
	case "1d":
		return "31"
	case "1h":
		return "168"
	case "1m":
		return "1440"
	default:
		return "100"
	}
}

func fixtureFromAnthropicUsage(opts anthropicUsageOptions) Fixture {
	totals := aggregateAnthropicTokens(opts.UsageBuckets)
	peaks := peakAnthropicTokensPerMinute(opts.PeakBuckets)
	models := sortedKeys(totals)
	fixture := Fixture{
		Provider:        "anthropic",
		Workspace:       firstNonEmpty(opts.Workspace, "organization"),
		AvailableModels: uniqueSortedStrings(models),
	}
	for _, model := range models {
		total := totals[model]
		id := "anthropic-" + safeID(model)
		usageClass := FixtureModelUsageClass{
			ID:       id,
			Name:     "Anthropic " + model,
			Provider: "anthropic",
			Models:   []string{model},
			Purpose:  "Observed Messages API usage from Anthropic Usage and Cost Admin APIs.",
			Owners:   append([]string{}, opts.Owners...),
		}
		if total.WebSearchRequests > 0 {
			toolID := id + "-web-search"
			usageClass.ToolRefs = []string{toolID}
			fixture.ToolUsages = append(fixture.ToolUsages, FixtureToolUsage{
				ID:                  toolID,
				UsageClassID:        id,
				Name:                "Anthropic server web search",
				Type:                "server-tool",
				NetworkAccess:       true,
				WritesExternalState: false,
			})
		}
		if opts.FallbackStrategy != "" {
			usageClass.FallbackBehavior = id + "-fallback"
		}
		fixture.ModelUsageClasses = append(fixture.ModelUsageClasses, usageClass)
		fixture.TokenVolumes = append(fixture.TokenVolumes, FixtureTokenVolume{
			ID:                  id + "-tokens",
			UsageClassID:        id,
			MonthlyInputTokens:  total.InputTokens,
			MonthlyOutputTokens: total.OutputTokens,
			PeakTokensPerMinute: peaks[model],
			BurstWindow:         fmt.Sprintf("max-1h-bucket-over-%dd-divided-by-60", opts.PeakDays),
		})
		if opts.DefaultP95Ms > 0 || opts.DefaultTimeoutMs > 0 || opts.StreamingRequired {
			fixture.LatencyExpectations = append(fixture.LatencyExpectations, FixtureLatencyExpectation{
				ID:                id + "-latency",
				UsageClassID:      id,
				P95Ms:             opts.DefaultP95Ms,
				TimeoutMs:         opts.DefaultTimeoutMs,
				StreamingRequired: opts.StreamingRequired,
			})
		}
		if opts.FallbackStrategy != "" {
			fixture.FallbackBehaviors = append(fixture.FallbackBehaviors, FixtureFallbackBehavior{
				ID:           id + "-fallback",
				UsageClassID: id,
				Strategy:     opts.FallbackStrategy,
				ManualQueue:  opts.FallbackManualQueue,
				MaxRetries:   opts.FallbackMaxRetries,
			})
		}
	}
	return fixture
}

func aggregateAnthropicTokens(buckets []anthropicUsageBucket) map[string]anthropicTokenTotals {
	out := map[string]anthropicTokenTotals{}
	for _, bucket := range buckets {
		for _, result := range bucket.Results {
			model := anthropicModelName(result.Model)
			total := out[model]
			total.InputTokens += anthropicInputTokens(result)
			total.OutputTokens += result.OutputTokens
			total.WebSearchRequests += result.ServerToolUse.WebSearchRequests
			out[model] = total
		}
	}
	return out
}

func peakAnthropicTokensPerMinute(buckets []anthropicUsageBucket) map[string]int {
	out := map[string]int{}
	for _, bucket := range buckets {
		minutes := anthropicBucketMinutes(bucket)
		for _, result := range bucket.Results {
			model := anthropicModelName(result.Model)
			tokens := anthropicInputTokens(result) + result.OutputTokens
			peak := int((tokens + int64(minutes) - 1) / int64(minutes))
			if peak > out[model] {
				out[model] = peak
			}
		}
	}
	return out
}

func anthropicInputTokens(result anthropicUsageResult) int64 {
	return result.UncachedInputTokens + result.CacheReadInputTokens + result.CacheCreation.Ephemeral1HInputTokens + result.CacheCreation.Ephemeral5MInputTokens
}

func anthropicBucketMinutes(bucket anthropicUsageBucket) int {
	start, startErr := time.Parse(time.RFC3339, bucket.StartingAt)
	end, endErr := time.Parse(time.RFC3339, bucket.EndingAt)
	if startErr != nil || endErr != nil {
		return 60
	}
	minutes := int(end.Sub(start).Minutes())
	if minutes <= 0 {
		return 60
	}
	return minutes
}

func anthropicModelName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown-model"
	}
	return model
}
