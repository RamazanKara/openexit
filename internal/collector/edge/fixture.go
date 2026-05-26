package edge

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

func (FixtureCollector) Name() string { return "edge-fixture" }

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
	inv := inventory.New(req.Project, "edge", fixture.Provider, version.Version, time.Now().UTC())
	if fixture.Zone != "" {
		inv.Source.Site = fixture.Provider + "/" + fixture.Zone
	}
	if err := normalizeFixture(req.ProjectDir, &fixture, inv, data); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

func normalizeFixture(projectDir string, fixture *Fixture, inv *inventory.Inventory, raw []byte) error {
	return normalizeFixtureWithRawPath(projectDir, fixture, inv, raw, "edge/raw-fixture.json")
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
	for _, record := range fixture.DNSRecords {
		evidenceID := safeID(record.ID)
		if err := writeJSONEvidence(projectDir, "edge/dns-records/"+evidenceID+".json", record); err != nil {
			return err
		}
		inv.Assets.DNSRecords = append(inv.Assets.DNSRecords, inventory.DNSRecord{
			ID:          record.ID,
			Name:        record.Name,
			Type:        record.Type,
			Value:       record.Value,
			TTL:         record.TTL,
			Proxied:     record.Proxied,
			EvidenceRef: "evidence://edge/dns-record/" + evidenceID,
		})
	}
	for _, rule := range fixture.WAFRules {
		evidenceID := safeID(rule.ID)
		if err := writeJSONEvidence(projectDir, "edge/waf-rules/"+evidenceID+".json", rule); err != nil {
			return err
		}
		inv.Assets.WAFRules = append(inv.Assets.WAFRules, inventory.WAFRule{
			ID:          rule.ID,
			Name:        rule.Name,
			Expression:  rule.Expression,
			Action:      rule.Action,
			Enabled:     rule.Enabled,
			Managed:     rule.Managed,
			EvidenceRef: "evidence://edge/waf-rule/" + evidenceID,
		})
	}
	for _, rule := range fixture.CacheRules {
		evidenceID := safeID(rule.ID)
		if err := writeJSONEvidence(projectDir, "edge/cache-rules/"+evidenceID+".json", rule); err != nil {
			return err
		}
		inv.Assets.CacheRules = append(inv.Assets.CacheRules, inventory.CacheRule{
			ID:          rule.ID,
			Name:        rule.Name,
			Pattern:     rule.Pattern,
			Action:      rule.Action,
			EdgeTTL:     rule.EdgeTTL,
			BrowserTTL:  rule.BrowserTTL,
			CacheKey:    append([]string{}, rule.CacheKey...),
			EvidenceRef: "evidence://edge/cache-rule/" + evidenceID,
		})
	}
	for _, redirect := range fixture.Redirects {
		evidenceID := safeID(redirect.ID)
		if err := writeJSONEvidence(projectDir, "edge/redirects/"+evidenceID+".json", redirect); err != nil {
			return err
		}
		inv.Assets.Redirects = append(inv.Assets.Redirects, inventory.RedirectRule{
			ID:            redirect.ID,
			Name:          redirect.Name,
			Source:        redirect.Source,
			Target:        redirect.Target,
			StatusCode:    redirect.StatusCode,
			PreserveQuery: redirect.PreserveQuery,
			EvidenceRef:   "evidence://edge/redirect/" + evidenceID,
		})
	}
	for _, origin := range fixture.Origins {
		evidenceID := safeID(origin.ID)
		if err := writeJSONEvidence(projectDir, "edge/origins/"+evidenceID+".json", origin); err != nil {
			return err
		}
		inv.Assets.Origins = append(inv.Assets.Origins, inventory.OriginConfig{
			ID:          origin.ID,
			Hostname:    origin.Hostname,
			Scheme:      origin.Scheme,
			Port:        origin.Port,
			HostHeader:  origin.HostHeader,
			TLSVerify:   origin.TLSVerify,
			HealthCheck: origin.HealthCheck,
			EvidenceRef: "evidence://edge/origin/" + evidenceID,
		})
	}
	for _, setting := range fixture.TLSSettings {
		evidenceID := safeID(setting.ID)
		if err := writeJSONEvidence(projectDir, "edge/tls-settings/"+evidenceID+".json", setting); err != nil {
			return err
		}
		inv.Assets.TLSSettings = append(inv.Assets.TLSSettings, inventory.TLSSetting{
			ID:          setting.ID,
			Hostname:    setting.Hostname,
			Mode:        setting.Mode,
			MinVersion:  setting.MinVersion,
			HSTSEnabled: setting.HSTSEnabled,
			EvidenceRef: "evidence://edge/tls-setting/" + evidenceID,
		})
	}
	for _, rule := range fixture.BotRules {
		evidenceID := safeID(rule.ID)
		if err := writeJSONEvidence(projectDir, "edge/bot-rules/"+evidenceID+".json", rule); err != nil {
			return err
		}
		inv.Assets.BotRules = append(inv.Assets.BotRules, inventory.BotRule{
			ID:          rule.ID,
			Name:        rule.Name,
			Expression:  rule.Expression,
			Action:      rule.Action,
			Enabled:     rule.Enabled,
			EvidenceRef: "evidence://edge/bot-rule/" + evidenceID,
		})
	}
	for _, rule := range fixture.PageRules {
		evidenceID := safeID(rule.ID)
		if err := writeJSONEvidence(projectDir, "edge/page-rules/"+evidenceID+".json", rule); err != nil {
			return err
		}
		inv.Assets.PageRules = append(inv.Assets.PageRules, inventory.PageRule{
			ID:          rule.ID,
			Target:      rule.Target,
			Actions:     append([]string{}, rule.Actions...),
			Priority:    rule.Priority,
			Enabled:     rule.Enabled,
			EvidenceRef: "evidence://edge/page-rule/" + evidenceID,
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
	sort.Slice(inv.Assets.DNSRecords, func(i, j int) bool { return inv.Assets.DNSRecords[i].ID < inv.Assets.DNSRecords[j].ID })
	sort.Slice(inv.Assets.WAFRules, func(i, j int) bool { return inv.Assets.WAFRules[i].ID < inv.Assets.WAFRules[j].ID })
	sort.Slice(inv.Assets.CacheRules, func(i, j int) bool { return inv.Assets.CacheRules[i].ID < inv.Assets.CacheRules[j].ID })
	sort.Slice(inv.Assets.Redirects, func(i, j int) bool { return inv.Assets.Redirects[i].ID < inv.Assets.Redirects[j].ID })
	sort.Slice(inv.Assets.Origins, func(i, j int) bool { return inv.Assets.Origins[i].ID < inv.Assets.Origins[j].ID })
	sort.Slice(inv.Assets.TLSSettings, func(i, j int) bool { return inv.Assets.TLSSettings[i].ID < inv.Assets.TLSSettings[j].ID })
	sort.Slice(inv.Assets.BotRules, func(i, j int) bool { return inv.Assets.BotRules[i].ID < inv.Assets.BotRules[j].ID })
	sort.Slice(inv.Assets.PageRules, func(i, j int) bool { return inv.Assets.PageRules[i].ID < inv.Assets.PageRules[j].ID })
}

func safeID(value string) string {
	return inventory.Slug(value)
}
