package aiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
	"gopkg.in/yaml.v3"
)

type FixtureCollector struct{}

func (FixtureCollector) Name() string { return "ai-fixture" }

func (FixtureCollector) Collect(_ context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	input := req.Options["input"]
	if input == "" {
		return nil, fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	data = inventory.RedactBytes(data)
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	inv := inventory.New(req.Project, "ai-provider", fixture.Provider, version.Version, time.Now().UTC())
	if fixture.Workspace != "" {
		inv.Source.Site = fixture.Provider + "/" + fixture.Workspace
	}
	if err := normalizeFixture(req.ProjectDir, &fixture, inv, data); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func normalizeFixture(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte) error {
	return normalizeFixtureWithRawPath(projectDir, fixture, inv, raw, "ai-provider/raw-fixture.json")
}

func normalizeFixtureWithRawPath(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte, rawPath string) error {
	if rawPath != "" && len(raw) > 0 {
		if err := writeEvidence(projectDir, rawPath, raw); err != nil {
			return err
		}
	}
	if rawPath == "" && len(raw) > 0 {
		return fmt.Errorf("raw evidence path is required when raw evidence is provided")
	}
	for _, usageClass := range fixture.ModelUsageClasses {
		evidenceID := safeID(usageClass.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/model-usage-classes/"+evidenceID+".json", usageClass); err != nil {
			return err
		}
		inv.Assets.AIModelUsageClasses = append(inv.Assets.AIModelUsageClasses, inventory.AIModelUsageClass{
			ID:                        usageClass.ID,
			Name:                      usageClass.Name,
			Provider:                  firstNonEmpty(usageClass.Provider, fixture.Provider),
			Models:                    append([]string{}, usageClass.Models...),
			Purpose:                   usageClass.Purpose,
			Owners:                    append([]string{}, usageClass.Owners...),
			SensitivePromptCategories: append([]string{}, usageClass.SensitivePromptCategories...),
			ToolRefs:                  append([]string{}, usageClass.ToolRefs...),
			FallbackBehavior:          usageClass.FallbackBehavior,
			EvidenceRef:               "evidence://ai-provider/model-usage-class/" + evidenceID,
		})
	}
	for _, volume := range fixture.TokenVolumes {
		evidenceID := safeID(volume.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/token-volumes/"+evidenceID+".json", volume); err != nil {
			return err
		}
		inv.Assets.AITokenVolumes = append(inv.Assets.AITokenVolumes, inventory.AITokenVolume{
			ID:                  volume.ID,
			UsageClassID:        volume.UsageClassID,
			MonthlyInputTokens:  volume.MonthlyInputTokens,
			MonthlyOutputTokens: volume.MonthlyOutputTokens,
			PeakTokensPerMinute: volume.PeakTokensPerMinute,
			BurstWindow:         volume.BurstWindow,
			EvidenceRef:         "evidence://ai-provider/token-volume/" + evidenceID,
		})
	}
	for _, latency := range fixture.LatencyExpectations {
		evidenceID := safeID(latency.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/latency-expectations/"+evidenceID+".json", latency); err != nil {
			return err
		}
		inv.Assets.AILatencyExpectations = append(inv.Assets.AILatencyExpectations, inventory.AILatencyExpectation{
			ID:                latency.ID,
			UsageClassID:      latency.UsageClassID,
			P50Ms:             latency.P50Ms,
			P95Ms:             latency.P95Ms,
			TimeoutMs:         latency.TimeoutMs,
			StreamingRequired: latency.StreamingRequired,
			EvidenceRef:       "evidence://ai-provider/latency-expectation/" + evidenceID,
		})
	}
	for _, category := range fixture.SensitivePromptCategories {
		evidenceID := safeID(category.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/sensitive-prompt-categories/"+evidenceID+".json", category); err != nil {
			return err
		}
		inv.Assets.AISensitivePromptCategories = append(inv.Assets.AISensitivePromptCategories, inventory.AISensitivePromptCategory{
			ID:                    category.ID,
			UsageClassID:          category.UsageClassID,
			Category:              category.Category,
			DataClass:             category.DataClass,
			ContainsPII:           category.ContainsPII,
			RequiresDataResidency: category.RequiresDataResidency,
			Retention:             category.Retention,
			EvidenceRef:           "evidence://ai-provider/sensitive-prompt-category/" + evidenceID,
		})
	}
	for _, tool := range fixture.ToolUsages {
		evidenceID := safeID(tool.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/tool-usages/"+evidenceID+".json", tool); err != nil {
			return err
		}
		inv.Assets.AIToolUsages = append(inv.Assets.AIToolUsages, inventory.AIToolUsage{
			ID:                    tool.ID,
			UsageClassID:          tool.UsageClassID,
			Name:                  tool.Name,
			Type:                  tool.Type,
			NetworkAccess:         tool.NetworkAccess,
			WritesExternalState:   tool.WritesExternalState,
			HumanApprovalRequired: tool.HumanApprovalRequired,
			EvidenceRef:           "evidence://ai-provider/ai-tool-usage/" + evidenceID,
		})
	}
	for _, fallback := range fixture.FallbackBehaviors {
		evidenceID := safeID(fallback.ID)
		if err := writeJSONEvidence(projectDir, "ai-provider/fallback-behaviors/"+evidenceID+".json", fallback); err != nil {
			return err
		}
		inv.Assets.AIFallbackBehaviors = append(inv.Assets.AIFallbackBehaviors, inventory.AIFallbackBehavior{
			ID:           fallback.ID,
			UsageClassID: fallback.UsageClassID,
			Strategy:     fallback.Strategy,
			Models:       append([]string{}, fallback.Models...),
			ManualQueue:  fallback.ManualQueue,
			MaxRetries:   fallback.MaxRetries,
			EvidenceRef:  "evidence://ai-provider/fallback-behavior/" + evidenceID,
		})
	}
	sortInventory(inv)
	inv.RecomputeSummary()
	return inventory.Validate(inv)
}

func writeInventory(projectDir string, inv *inventory.Inventory) error {
	invDir := filepath.Join(projectDir, "inventory")
	if err := os.MkdirAll(invDir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(inv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(invDir, "openexit.inventory.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(invDir, "openexit.inventory.json"), jsonData, 0o644)
}

func writeJSONEvidence(projectDir, rel string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeEvidence(projectDir, rel, inventory.RedactBytes(data))
}

func writeEvidence(projectDir, rel string, data []byte) error {
	path := filepath.Join(projectDir, "evidence", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func sortInventory(inv *inventory.Inventory) {
	sort.Slice(inv.Assets.AIModelUsageClasses, func(i, j int) bool {
		return inv.Assets.AIModelUsageClasses[i].ID < inv.Assets.AIModelUsageClasses[j].ID
	})
	sort.Slice(inv.Assets.AITokenVolumes, func(i, j int) bool {
		return inv.Assets.AITokenVolumes[i].ID < inv.Assets.AITokenVolumes[j].ID
	})
	sort.Slice(inv.Assets.AILatencyExpectations, func(i, j int) bool {
		return inv.Assets.AILatencyExpectations[i].ID < inv.Assets.AILatencyExpectations[j].ID
	})
	sort.Slice(inv.Assets.AISensitivePromptCategories, func(i, j int) bool {
		return inv.Assets.AISensitivePromptCategories[i].ID < inv.Assets.AISensitivePromptCategories[j].ID
	})
	sort.Slice(inv.Assets.AIToolUsages, func(i, j int) bool {
		return inv.Assets.AIToolUsages[i].ID < inv.Assets.AIToolUsages[j].ID
	})
	sort.Slice(inv.Assets.AIFallbackBehaviors, func(i, j int) bool {
		return inv.Assets.AIFallbackBehaviors[i].ID < inv.Assets.AIFallbackBehaviors[j].ID
	})
}

func safeID(value string) string {
	return inventory.Slug(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
