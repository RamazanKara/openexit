package assessment

import (
	"context"
	"fmt"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type AIProviderAnalyzer struct{}

func (AIProviderAnalyzer) Name() string { return "ai-provider-vllm-litellm" }

func (AIProviderAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	if inv.Source.Type != "ai-provider" && len(inv.Assets.AIModelUsageClasses) == 0 {
		return nil, nil
	}
	var findings []Finding
	usageClasses := map[string]inventory.AIModelUsageClass{}
	for _, usageClass := range inv.Assets.AIModelUsageClasses {
		usageClasses[usageClass.ID] = usageClass
		if len(usageClass.Owners) == 0 {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.ownership.missing.001", "medium", "Model usage class is missing an owner", fmt.Sprintf("%s has no captured owner.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Assign an accountable owner before evaluating or cutting over the target route."))
		}
		if len(usageClass.Models) == 0 {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.model.mapping-missing.001", "high", "Model usage class has no source model", fmt.Sprintf("%s has no captured source model.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Capture source model names and map them to candidate self-hosted models before sizing."))
		}
		if isProprietaryAIProvider(usageClass.Provider) || containsProprietaryModel(usageClass.Models) {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.model.provider-specific.001", "medium", "Provider-specific model behavior needs evaluation", fmt.Sprintf("%s uses provider/model metadata that may not transfer directly to self-hosted models.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Build a task-specific evaluation set and compare target quality before changing production routing."))
		}
	}
	volumeSeen := map[string]struct{}{}
	for _, volume := range inv.Assets.AITokenVolumes {
		volumeSeen[volume.UsageClassID] = struct{}{}
		total := volume.MonthlyInputTokens + volume.MonthlyOutputTokens
		if total > 500_000_000 {
			findings = append(findings, aiFinding(volume.EvidenceRef, "ai.capacity.high-token-volume.001", "high", "High model volume needs capacity planning", fmt.Sprintf("%s processes %d monthly model tokens.", usageName(usageClasses, volume.UsageClassID), total), []string{"ai-token-volume:" + volume.ID, "ai-model-usage-class:" + volume.UsageClassID}, "Run a load test against candidate vLLM deployments and reserve capacity for representative peak traffic."))
		} else if total > 100_000_000 {
			findings = append(findings, aiFinding(volume.EvidenceRef, "ai.capacity.high-token-volume.001", "medium", "Model volume needs capacity planning", fmt.Sprintf("%s processes %d monthly model tokens.", usageName(usageClasses, volume.UsageClassID), total), []string{"ai-token-volume:" + volume.ID, "ai-model-usage-class:" + volume.UsageClassID}, "Estimate target GPU and concurrency needs before routing traffic through LiteLLM."))
		}
		if volume.PeakTokensPerMinute > 50_000 {
			findings = append(findings, aiFinding(volume.EvidenceRef, "ai.capacity.peak-throughput.001", "high", "Peak model throughput is high", fmt.Sprintf("%s peaks at %d tokens per minute.", usageName(usageClasses, volume.UsageClassID), volume.PeakTokensPerMinute), []string{"ai-token-volume:" + volume.ID, "ai-model-usage-class:" + volume.UsageClassID}, "Size vLLM replicas and queue limits around peak traffic, not monthly averages."))
		} else if volume.PeakTokensPerMinute > 30_000 {
			findings = append(findings, aiFinding(volume.EvidenceRef, "ai.capacity.peak-throughput.001", "medium", "Peak model throughput needs review", fmt.Sprintf("%s peaks at %d tokens per minute.", usageName(usageClasses, volume.UsageClassID), volume.PeakTokensPerMinute), []string{"ai-token-volume:" + volume.ID, "ai-model-usage-class:" + volume.UsageClassID}, "Validate target concurrency, batching, and queue behavior before production traffic."))
		}
	}
	latencySeen := map[string]struct{}{}
	for _, latency := range inv.Assets.AILatencyExpectations {
		latencySeen[latency.UsageClassID] = struct{}{}
		if latency.P95Ms > 0 && latency.P95Ms <= 1500 {
			findings = append(findings, aiFinding(latency.EvidenceRef, "ai.latency.tight-slo.001", "high", "Latency expectation is tight for self-hosting", fmt.Sprintf("%s expects p95 latency at or below %d ms.", usageName(usageClasses, latency.UsageClassID), latency.P95Ms), []string{"ai-latency-expectation:" + latency.ID, "ai-model-usage-class:" + latency.UsageClassID}, "Benchmark candidate models with production-like prompts, context sizes, and streaming behavior."))
		}
		if latency.StreamingRequired {
			findings = append(findings, aiFinding(latency.EvidenceRef, "ai.latency.streaming-required.001", "medium", "Streaming behavior must be preserved", fmt.Sprintf("%s requires streaming responses.", usageName(usageClasses, latency.UsageClassID)), []string{"ai-latency-expectation:" + latency.ID, "ai-model-usage-class:" + latency.UsageClassID}, "Verify LiteLLM and target model streaming semantics with application clients before cutover."))
		}
	}
	for _, category := range inv.Assets.AISensitivePromptCategories {
		if category.ContainsPII || category.RequiresDataResidency || strings.EqualFold(category.DataClass, "restricted") {
			findings = append(findings, aiFinding(category.EvidenceRef, "ai.data.sensitive-prompts.001", "high", "Sensitive prompt class needs target controls", fmt.Sprintf("%s carries %s data for %s.", category.Category, category.DataClass, usageName(usageClasses, category.UsageClassID)), []string{"ai-sensitive-prompt-category:" + category.ID, "ai-model-usage-class:" + category.UsageClassID}, "Define redaction, retention, residency, and logging controls before routing prompts to a self-hosted stack."))
		}
		if strings.EqualFold(category.Retention, "redacted-analytics") {
			findings = append(findings, aiFinding(category.EvidenceRef, "ai.data.retention-review.001", "medium", "Prompt retention policy needs review", fmt.Sprintf("%s retains redacted analytics.", category.Category), []string{"ai-sensitive-prompt-category:" + category.ID, "ai-model-usage-class:" + category.UsageClassID}, "Confirm that target observability, evaluation, and prompt logging preserve the approved retention behavior."))
		}
	}
	for _, tool := range inv.Assets.AIToolUsages {
		if tool.NetworkAccess {
			findings = append(findings, aiFinding(tool.EvidenceRef, "ai.tool.network-access.001", "medium", "Tool uses network access", fmt.Sprintf("%s performs network access for %s.", tool.Name, usageName(usageClasses, tool.UsageClassID)), []string{"ai-tool-usage:" + tool.ID, "ai-model-usage-class:" + tool.UsageClassID}, "Proxy and audit tool calls explicitly when moving behind LiteLLM or a self-hosted model gateway."))
		}
		if tool.WritesExternalState {
			findings = append(findings, aiFinding(tool.EvidenceRef, "ai.tool.external-write.001", "high", "Tool can write external state", fmt.Sprintf("%s can write external state.", tool.Name), []string{"ai-tool-usage:" + tool.ID, "ai-model-usage-class:" + tool.UsageClassID}, "Require scoped credentials, idempotency, audit logs, and rollback procedures for write-capable tools."))
		}
		if tool.WritesExternalState && !tool.HumanApprovalRequired {
			findings = append(findings, aiFinding(tool.EvidenceRef, "ai.tool.approval-missing.001", "high", "Write-capable tool lacks human approval", fmt.Sprintf("%s writes external state without captured human approval.", tool.Name), []string{"ai-tool-usage:" + tool.ID, "ai-model-usage-class:" + tool.UsageClassID}, "Add an approval gate or narrow the tool scope before enabling target routes."))
		}
	}
	fallbackSeen := map[string]struct{}{}
	for _, fallback := range inv.Assets.AIFallbackBehaviors {
		fallbackSeen[fallback.UsageClassID] = struct{}{}
		if strings.Contains(strings.ToLower(fallback.Strategy), "source-provider") || containsProprietaryModel(fallback.Models) {
			findings = append(findings, aiFinding(fallback.EvidenceRef, "ai.fallback.proprietary-provider.001", "medium", "Fallback still depends on proprietary provider", fmt.Sprintf("%s fallback strategy is %s.", usageName(usageClasses, fallback.UsageClassID), fallback.Strategy), []string{"ai-fallback-behavior:" + fallback.ID, "ai-model-usage-class:" + fallback.UsageClassID}, "Decide whether source-provider fallback is temporary, remove it from steady-state architecture, and document the retirement trigger."))
		}
		if fallback.MaxRetries > 2 && !fallback.ManualQueue {
			findings = append(findings, aiFinding(fallback.EvidenceRef, "ai.fallback.retry-loop.001", "low", "Fallback retry behavior needs bounds", fmt.Sprintf("%s allows %d retries without a manual queue.", usageName(usageClasses, fallback.UsageClassID), fallback.MaxRetries), []string{"ai-fallback-behavior:" + fallback.ID, "ai-model-usage-class:" + fallback.UsageClassID}, "Set explicit retry budgets and queue behavior for model outages or target overload."))
		}
	}
	for _, usageClass := range inv.Assets.AIModelUsageClasses {
		if _, ok := volumeSeen[usageClass.ID]; !ok {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.capacity.volume-missing.001", "medium", "Token volume is missing", fmt.Sprintf("%s has no captured token volume.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Capture monthly and peak token volume before sizing vLLM or LiteLLM routing."))
		}
		if _, ok := latencySeen[usageClass.ID]; !ok {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.latency.expectation-missing.001", "medium", "Latency expectation is missing", fmt.Sprintf("%s has no captured latency expectation.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Capture p95 and timeout expectations before declaring the target route ready."))
		}
		if _, ok := fallbackSeen[usageClass.ID]; !ok {
			findings = append(findings, aiFinding(usageClass.EvidenceRef, "ai.fallback.missing.001", "high", "Fallback behavior is missing", fmt.Sprintf("%s has no captured fallback behavior.", usageClass.Name), []string{"ai-model-usage-class:" + usageClass.ID}, "Define manual queue, retry, or degraded-mode behavior before production cutover."))
		}
	}
	return findings, nil
}

func aiFinding(evidenceRef, id, severity, title, description string, affected []string, recommendation string) Finding {
	return Finding{
		ID:             id,
		Severity:       severity,
		Title:          title,
		Description:    description,
		AffectedAssets: affected,
		EvidenceRefs:   []string{evidenceRef},
		Recommendation: recommendation,
	}
}

func usageName(usageClasses map[string]inventory.AIModelUsageClass, id string) string {
	if usageClass, ok := usageClasses[id]; ok && usageClass.Name != "" {
		return usageClass.Name
	}
	return id
}

func isProprietaryAIProvider(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return provider == "openai" || provider == "anthropic"
}

func containsProprietaryModel(models []string) bool {
	for _, model := range models {
		model = strings.ToLower(strings.TrimSpace(model))
		if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "claude-") {
			return true
		}
	}
	return false
}
