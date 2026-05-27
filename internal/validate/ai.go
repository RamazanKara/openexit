package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

type liteLLMConfigCandidate struct {
	APIVersion             string                    `yaml:"apiVersion"`
	Kind                   string                    `yaml:"kind"`
	Metadata               aiCandidateMetadata       `yaml:"metadata"`
	ModelList              []liteLLMModelRoute       `yaml:"model_list"`
	Router                 liteLLMRouterCandidate    `yaml:"router"`
	VLLMSizingAssumptions  []liteLLMSizingAssumption `yaml:"vllmSizingAssumptions"`
	HumanReviewRequired    bool                      `yaml:"humanReviewRequired"`
	CredentialsIncluded    bool                      `yaml:"credentialsIncluded"`
	ProductionReady        bool                      `yaml:"productionReady"`
	CandidateGeneratedFrom string                    `yaml:"candidateGeneratedFrom"`
	rawText                string                    `yaml:"-"`
}

type aiCandidateMetadata struct {
	Project string `yaml:"project"`
	Source  string `yaml:"source"`
	Target  string `yaml:"target"`
}

type liteLLMModelRoute struct {
	ModelName           string         `yaml:"model_name"`
	SourceModels        []string       `yaml:"sourceModels"`
	Owners              []string       `yaml:"owners"`
	LiteLLMParams       liteLLMParams  `yaml:"litellm_params"`
	EvidenceRef         string         `yaml:"evidenceRef"`
	PeakTokensPerMinute int            `yaml:"peakTokensPerMinute"`
	P95Ms               int            `yaml:"p95Ms"`
	StreamingRequired   bool           `yaml:"streamingRequired"`
	FallbackStrategy    string         `yaml:"fallbackStrategy"`
	ManualQueue         bool           `yaml:"manualQueue"`
	raw                 map[string]any `yaml:"-"`
}

type liteLLMParams struct {
	Model         string `yaml:"model"`
	APIBaseRef    string `yaml:"apiBaseRef"`
	CredentialRef string `yaml:"credentialRef"`
}

type liteLLMRouterCandidate struct {
	RoutingStrategy string `yaml:"routingStrategy"`
	Gateway         string `yaml:"gateway"`
	TargetRuntime   string `yaml:"targetRuntime"`
	ToolPolicy      string `yaml:"toolPolicy"`
	EvidenceBacked  bool   `yaml:"evidenceBacked"`
}

type liteLLMSizingAssumption struct {
	UsageClassID        string `yaml:"usageClassId"`
	Route               string `yaml:"route"`
	MonthlyModelTokens  int64  `yaml:"monthlyModelTokens"`
	PeakTokensPerMinute int    `yaml:"peakTokensPerMinute"`
	BaselineReplicas    int    `yaml:"baselineReplicas"`
	EvidenceRef         string `yaml:"evidenceRef"`
}

func addLiteLLMConfigCandidateChecks(projectDir string, inv *inventory.Inventory, a *assessment.Assessment, add func(string, string, string, bool)) {
	if inv == nil || inv.Source.Type != "ai-provider" {
		return
	}
	path := filepath.Join(projectDir, "generated-config", "ai", "litellm", "config.candidate.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		add("litellm-config-candidate", "failed", err.Error(), true)
		return
	}
	candidate, err := readLiteLLMConfigCandidate(path)
	if err != nil {
		add("litellm-config-candidate", "failed", err.Error(), true)
		return
	}
	var problems []string
	problems = append(problems, validateLiteLLMShape(candidate, inv, a)...)
	problems = append(problems, validateLiteLLMModelRoutes(candidate.ModelList, inv)...)
	problems = append(problems, validateLiteLLMSizingAssumptions(candidate.VLLMSizingAssumptions, inv)...)
	problems = append(problems, validateLiteLLMReadme(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("litellm-config-candidate", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("litellm-config-candidate", "passed", "", true)
}

func readLiteLLMConfigCandidate(path string) (*liteLLMConfigCandidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		ModelList []map[string]any `yaml:"model_list"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var candidate liteLLMConfigCandidate
	if err := yaml.Unmarshal(data, &candidate); err != nil {
		return nil, err
	}
	for i := range candidate.ModelList {
		if i < len(raw.ModelList) {
			candidate.ModelList[i].raw = raw.ModelList[i]
		}
	}
	candidate.rawText = string(data)
	return &candidate, nil
}

func validateLiteLLMShape(candidate *liteLLMConfigCandidate, inv *inventory.Inventory, a *assessment.Assessment) []string {
	if candidate == nil {
		return []string{"LiteLLM config candidate is empty"}
	}
	var problems []string
	if inventory.ContainsSecret(candidate.rawText) {
		problems = append(problems, "config.candidate.yaml contains secret-like content")
	}
	if candidate.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q, got %q", APIVersion, candidate.APIVersion))
	}
	if candidate.Kind != "LiteLLMConfigCandidate" {
		problems = append(problems, fmt.Sprintf("kind must be LiteLLMConfigCandidate, got %q", candidate.Kind))
	}
	if candidate.Metadata.Project != inv.Metadata.Project {
		problems = append(problems, fmt.Sprintf("metadata.project must be %q, got %q", inv.Metadata.Project, candidate.Metadata.Project))
	}
	if candidate.Metadata.Source != inv.Source.Type {
		problems = append(problems, fmt.Sprintf("metadata.source must be %q, got %q", inv.Source.Type, candidate.Metadata.Source))
	}
	if a != nil && candidate.Metadata.Target != a.Target.Type {
		problems = append(problems, fmt.Sprintf("metadata.target must be %q, got %q", a.Target.Type, candidate.Metadata.Target))
	}
	for _, field := range []string{"humanReviewRequired:", "credentialsIncluded:", "productionReady:", "candidateGeneratedFrom:"} {
		if !strings.Contains(candidate.rawText, field) {
			problems = append(problems, "missing safety field "+strings.TrimSuffix(field, ":"))
		}
	}
	if !candidate.HumanReviewRequired {
		problems = append(problems, "humanReviewRequired must be true")
	}
	if candidate.CredentialsIncluded {
		problems = append(problems, "credentialsIncluded must be false")
	}
	if candidate.ProductionReady {
		problems = append(problems, "productionReady must be false")
	}
	if candidate.CandidateGeneratedFrom != "redacted-openexit-inventory" {
		problems = append(problems, fmt.Sprintf("candidateGeneratedFrom must be redacted-openexit-inventory, got %q", candidate.CandidateGeneratedFrom))
	}
	expectedRouter := liteLLMRouterCandidate{
		RoutingStrategy: "explicit-model-aliases",
		Gateway:         "litellm",
		TargetRuntime:   "vllm",
		ToolPolicy:      "deny-write-tools-until-reviewed",
		EvidenceBacked:  inv.Summary.AIModelUsageClasses > 0,
	}
	if candidate.Router != expectedRouter {
		problems = append(problems, "router must preserve explicit LiteLLM/vLLM routing policy and evidenceBacked state")
	}
	return problems
}

func validateLiteLLMModelRoutes(routes []liteLLMModelRoute, inv *inventory.Inventory) []string {
	var problems []string
	byRoute := map[string]liteLLMModelRoute{}
	for _, route := range routes {
		byRoute[route.ModelName] = route
	}
	if len(routes) != len(inv.Assets.AIModelUsageClasses) {
		problems = append(problems, fmt.Sprintf("model_list count %d does not match model usage class count %d", len(routes), len(inv.Assets.AIModelUsageClasses)))
	}
	volumes := aiTokenVolumesByUsageClass(inv.Assets.AITokenVolumes)
	latencies := aiLatenciesByUsageClass(inv.Assets.AILatencyExpectations)
	fallbacks := aiFallbacksByUsageClass(inv.Assets.AIFallbackBehaviors)
	for _, usageClass := range inv.Assets.AIModelUsageClasses {
		routeName := inventory.Slug(usageClass.Name)
		route, ok := byRoute[routeName]
		if !ok {
			problems = append(problems, fmt.Sprintf("model_list missing route for usage class %s", usageClass.ID))
			continue
		}
		if route.EvidenceRef != usageClass.EvidenceRef {
			problems = append(problems, fmt.Sprintf("model_list.%s.evidenceRef must be %q", routeName, usageClass.EvidenceRef))
		}
		if !sameStringSet(route.SourceModels, usageClass.Models) {
			problems = append(problems, fmt.Sprintf("model_list.%s.sourceModels must match inventory", routeName))
		}
		if !sameStringSet(route.Owners, usageClass.Owners) {
			problems = append(problems, fmt.Sprintf("model_list.%s.owners must match inventory", routeName))
		}
		expectedParams := liteLLMParams{
			Model:         "vllm/" + aiTargetModelPlaceholder(usageClass),
			APIBaseRef:    "vllm-service-" + inventory.Slug(usageClass.ID),
			CredentialRef: "runtime-managed-credential",
		}
		if route.LiteLLMParams != expectedParams {
			problems = append(problems, fmt.Sprintf("model_list.%s.litellm_params must preserve vLLM model, apiBaseRef, and runtime credential placeholder", routeName))
		}
		if volume, ok := volumes[usageClass.ID]; ok {
			if _, present := route.raw["peakTokensPerMinute"]; !present {
				problems = append(problems, fmt.Sprintf("model_list.%s.peakTokensPerMinute is required when token volume exists", routeName))
			}
			if route.PeakTokensPerMinute != volume.PeakTokensPerMinute {
				problems = append(problems, fmt.Sprintf("model_list.%s.peakTokensPerMinute must be %d", routeName, volume.PeakTokensPerMinute))
			}
		}
		if latency, ok := latencies[usageClass.ID]; ok {
			if _, present := route.raw["p95Ms"]; !present {
				problems = append(problems, fmt.Sprintf("model_list.%s.p95Ms is required when latency expectation exists", routeName))
			}
			if route.P95Ms != latency.P95Ms || route.StreamingRequired != latency.StreamingRequired {
				problems = append(problems, fmt.Sprintf("model_list.%s must preserve p95Ms and streamingRequired", routeName))
			}
		}
		if fallback, ok := fallbacks[usageClass.ID]; ok {
			if route.FallbackStrategy != fallback.Strategy || route.ManualQueue != fallback.ManualQueue {
				problems = append(problems, fmt.Sprintf("model_list.%s must preserve fallback strategy and manualQueue", routeName))
			}
		}
	}
	return problems
}

func validateLiteLLMSizingAssumptions(assumptions []liteLLMSizingAssumption, inv *inventory.Inventory) []string {
	var problems []string
	byUsageClass := map[string]liteLLMSizingAssumption{}
	for _, assumption := range assumptions {
		byUsageClass[assumption.UsageClassID] = assumption
	}
	usageClasses := aiUsageClassesByID(inv.Assets.AIModelUsageClasses)
	expectedCount := 0
	for _, volume := range inv.Assets.AITokenVolumes {
		if _, ok := usageClasses[volume.UsageClassID]; ok {
			expectedCount++
		}
	}
	if len(assumptions) != expectedCount {
		problems = append(problems, fmt.Sprintf("vllmSizingAssumptions count %d does not match token-volume-backed usage class count %d", len(assumptions), expectedCount))
	}
	for _, volume := range inv.Assets.AITokenVolumes {
		usageClass, ok := usageClasses[volume.UsageClassID]
		if !ok {
			continue
		}
		assumption, ok := byUsageClass[volume.UsageClassID]
		if !ok {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions missing usage class %s", volume.UsageClassID))
			continue
		}
		if assumption.Route != inventory.Slug(usageClass.Name) {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions.%s.route must be %q", volume.UsageClassID, inventory.Slug(usageClass.Name)))
		}
		if assumption.MonthlyModelTokens != volume.MonthlyInputTokens+volume.MonthlyOutputTokens {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions.%s.monthlyModelTokens must match token totals", volume.UsageClassID))
		}
		if assumption.PeakTokensPerMinute != volume.PeakTokensPerMinute {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions.%s.peakTokensPerMinute must be %d", volume.UsageClassID, volume.PeakTokensPerMinute))
		}
		if assumption.BaselineReplicas != aiRecommendedVLLMReplicas(volume.PeakTokensPerMinute) {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions.%s.baselineReplicas must be %d", volume.UsageClassID, aiRecommendedVLLMReplicas(volume.PeakTokensPerMinute)))
		}
		if assumption.EvidenceRef != volume.EvidenceRef {
			problems = append(problems, fmt.Sprintf("vllmSizingAssumptions.%s.evidenceRef must be %q", volume.UsageClassID, volume.EvidenceRef))
		}
	}
	return problems
}

func validateLiteLLMReadme(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "ai", "litellm", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/ai/litellm/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"deterministic LiteLLM routing candidate", "vLLM-backed models", "no provider credentials", "not production-ready", "model quality", "tool controls"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/ai/litellm/README.md: missing %q", required)}
		}
	}
	return nil
}

func aiTokenVolumesByUsageClass(volumes []inventory.AITokenVolume) map[string]inventory.AITokenVolume {
	out := map[string]inventory.AITokenVolume{}
	for _, volume := range volumes {
		out[volume.UsageClassID] = volume
	}
	return out
}

func aiLatenciesByUsageClass(latencies []inventory.AILatencyExpectation) map[string]inventory.AILatencyExpectation {
	out := map[string]inventory.AILatencyExpectation{}
	for _, latency := range latencies {
		out[latency.UsageClassID] = latency
	}
	return out
}

func aiFallbacksByUsageClass(fallbacks []inventory.AIFallbackBehavior) map[string]inventory.AIFallbackBehavior {
	out := map[string]inventory.AIFallbackBehavior{}
	for _, fallback := range fallbacks {
		out[fallback.UsageClassID] = fallback
	}
	return out
}

func aiUsageClassesByID(classes []inventory.AIModelUsageClass) map[string]inventory.AIModelUsageClass {
	out := map[string]inventory.AIModelUsageClass{}
	for _, usageClass := range classes {
		out[usageClass.ID] = usageClass
	}
	return out
}

func aiRecommendedVLLMReplicas(peakTokensPerMinute int) int {
	if peakTokensPerMinute <= 0 {
		return 1
	}
	replicas := (peakTokensPerMinute + 11999) / 12000
	if replicas < 1 {
		return 1
	}
	return replicas
}

func aiTargetModelPlaceholder(usageClass inventory.AIModelUsageClass) string {
	if len(usageClass.Models) == 0 {
		return inventory.Slug(usageClass.ID) + "-candidate"
	}
	return inventory.Slug(usageClass.ID) + "-replacement-for-" + inventory.Slug(usageClass.Models[0])
}
