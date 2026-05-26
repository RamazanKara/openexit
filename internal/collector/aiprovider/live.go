package aiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type LiveCollector struct{}

func (LiveCollector) Name() string { return "openai" }

func (LiveCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	tokenEnv := option(req.Options, "admin-key-env", "OPENAI_ADMIN_KEY")
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
	client, err := NewOpenAIClient(req.Options["base-url"], token, req.Options["organization-id"], req.Options["project-id"])
	if err != nil {
		return nil, err
	}

	var warnings []string
	now := time.Now().UTC().Truncate(time.Hour)
	usageStart := now.AddDate(0, 0, -days)
	usageBuckets, err := collectOpenAICompletionsUsage(ctx, client, usageStart, now, "1d")
	if err != nil {
		return nil, err
	}
	peakStart := now.AddDate(0, 0, -peakDays)
	peakBuckets, err := collectOpenAICompletionsUsage(ctx, client, peakStart, now, "1h")
	if err != nil {
		warnings = append(warnings, optionalWarning("collect hourly OpenAI usage for peak estimate", err))
	}

	models, err := collectOpenAIModels(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect OpenAI model list", err))
	}
	if days != 30 {
		warnings = append(warnings, fmt.Sprintf("OpenAI token volume fields reflect a %d-day collection window, not a normalized calendar month.", days))
	}
	warnings = append(warnings, "OpenAI usage APIs expose aggregate operational metadata only; latency, prompt sensitivity, tool use, and fallback behavior should be supplied from application owners when not passed as collector flags.")

	workspace := option(req.Options, "workspace", "organization")
	fixture := fixtureFromOpenAIUsage(openAIUsageOptions{
		Workspace:           workspace,
		Owners:              splitOptionList(req.Options["owners"]),
		AvailableModels:     models,
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
		warnings = append(warnings, "OpenAI usage API returned no model-grouped completions usage for the selected window.")
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "ai-provider", "openai/"+workspace, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "ai-provider/raw-openai-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type openAIUsageOptions struct {
	Workspace           string
	Owners              []string
	AvailableModels     []string
	UsageBuckets        []openAIUsageBucket
	PeakBuckets         []openAIUsageBucket
	DefaultP95Ms        int
	DefaultTimeoutMs    int
	StreamingRequired   bool
	FallbackStrategy    string
	FallbackManualQueue bool
	FallbackMaxRetries  int
	PeakDays            int
}

type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

type openAIUsageResponse struct {
	Data     []openAIUsageBucket `json:"data"`
	HasMore  bool                `json:"has_more"`
	NextPage string              `json:"next_page"`
}

type openAIUsageBucket struct {
	StartTime int64               `json:"start_time"`
	EndTime   int64               `json:"end_time"`
	Object    string              `json:"object"`
	Results   []openAIUsageResult `json:"results"`
}

func (b *openAIUsageBucket) UnmarshalJSON(data []byte) error {
	type bucketAlias openAIUsageBucket
	var raw struct {
		bucketAlias
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*b = openAIUsageBucket(raw.bucketAlias)
	if len(b.Results) == 0 && len(raw.Result) > 0 && string(raw.Result) != "null" {
		var single openAIUsageResult
		if err := json.Unmarshal(raw.Result, &single); err == nil {
			b.Results = []openAIUsageResult{single}
		}
	}
	return nil
}

type openAIUsageResult struct {
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	InputAudioTokens  int64  `json:"input_audio_tokens"`
	OutputAudioTokens int64  `json:"output_audio_tokens"`
	NumModelRequests  int64  `json:"num_model_requests"`
	Model             string `json:"model"`
	ProjectID         string `json:"project_id"`
	UserID            string `json:"user_id"`
	APIKeyID          string `json:"api_key_id"`
	ServiceTier       string `json:"service_tier"`
}

type openAITokenTotals struct {
	InputTokens  int64
	OutputTokens int64
	Requests     int64
}

func collectOpenAIModels(ctx context.Context, client *OpenAIClient) ([]string, error) {
	var response openAIModelsResponse
	if _, err := client.get(ctx, "/models", nil, &response); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		models = append(models, model.ID)
	}
	return uniqueSortedStrings(models), nil
}

func collectOpenAICompletionsUsage(ctx context.Context, client *OpenAIClient, start, end time.Time, bucketWidth string) ([]openAIUsageBucket, error) {
	query := url.Values{}
	query.Set("start_time", strconv.FormatInt(start.Unix(), 10))
	query.Set("end_time", strconv.FormatInt(end.Unix(), 10))
	query.Set("bucket_width", bucketWidth)
	query.Set("group_by", "model")
	query.Set("limit", openAIUsageLimit(bucketWidth))

	var buckets []openAIUsageBucket
	for {
		var response openAIUsageResponse
		if _, err := client.get(ctx, "/organization/usage/completions", query, &response); err != nil {
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

func openAIUsageLimit(bucketWidth string) string {
	switch bucketWidth {
	case "1d":
		return "31"
	case "1h":
		return "168"
	default:
		return "100"
	}
}

func fixtureFromOpenAIUsage(opts openAIUsageOptions) Fixture {
	totals := aggregateOpenAITokens(opts.UsageBuckets)
	peaks := peakOpenAITokensPerMinute(opts.PeakBuckets)
	models := sortedKeys(totals)
	fixture := Fixture{
		Provider:        "openai",
		Workspace:       firstNonEmpty(opts.Workspace, "organization"),
		AvailableModels: uniqueSortedStrings(opts.AvailableModels),
	}
	for _, model := range models {
		total := totals[model]
		id := "openai-" + safeID(model)
		usageClass := FixtureModelUsageClass{
			ID:       id,
			Name:     "OpenAI " + model,
			Provider: "openai",
			Models:   []string{model},
			Purpose:  "Observed completions usage from OpenAI organization usage APIs.",
			Owners:   append([]string{}, opts.Owners...),
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

func aggregateOpenAITokens(buckets []openAIUsageBucket) map[string]openAITokenTotals {
	out := map[string]openAITokenTotals{}
	for _, bucket := range buckets {
		for _, result := range bucket.Results {
			model := openAIModelName(result.Model)
			total := out[model]
			total.InputTokens += result.InputTokens + result.InputAudioTokens
			total.OutputTokens += result.OutputTokens + result.OutputAudioTokens
			total.Requests += result.NumModelRequests
			out[model] = total
		}
	}
	return out
}

func peakOpenAITokensPerMinute(buckets []openAIUsageBucket) map[string]int {
	out := map[string]int{}
	for _, bucket := range buckets {
		minutes := int((bucket.EndTime - bucket.StartTime) / 60)
		if minutes <= 0 {
			minutes = 60
		}
		for _, result := range bucket.Results {
			model := openAIModelName(result.Model)
			tokens := result.InputTokens + result.OutputTokens + result.InputAudioTokens + result.OutputAudioTokens
			peak := int((tokens + int64(minutes) - 1) / int64(minutes))
			if peak > out[model] {
				out[model] = peak
			}
		}
	}
	return out
}

func openAIModelName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown-model"
	}
	return model
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func positiveIntOption(options map[string]string, key string, fallback int) (int, error) {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("--%s must be a positive integer", key)
	}
	return parsed, nil
}

func intOption(options map[string]string, key string, fallback int) int {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolOption(options map[string]string, key string) bool {
	value := strings.ToLower(strings.TrimSpace(options[key]))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func splitOptionList(raw string) []string {
	var out []string
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' }) {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return uniqueSortedStrings(out)
}

func optionalWarning(label string, err error) string {
	return fmt.Sprintf("%s: %v", label, err)
}

func compactWarnings(warnings []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		out = append(out, warning)
	}
	sort.Strings(out)
	return out
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func option(options map[string]string, key, fallback string) string {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	return value
}

func prettyJSON(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return append(data, '\n')
}
