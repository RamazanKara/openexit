package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func (LiveCollector) Name() string { return "cloudflare" }

func (LiveCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	zoneID := strings.TrimSpace(req.Options["zone-id"])
	if zoneID == "" {
		return nil, fmt.Errorf("--zone-id is required")
	}
	tokenEnv := option(req.Options, "token-env", "CLOUDFLARE_API_TOKEN")
	token, err := envSecret(tokenEnv)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(req.Options["base-url"], token)
	if err != nil {
		return nil, err
	}

	var warnings []string
	zoneName := zoneID
	if zone, err := collectZone(ctx, client, zoneID); err == nil && zone.Name != "" {
		zoneName = zone.Name
	} else if err != nil {
		warnings = append(warnings, optionalWarning("collect zone metadata", err))
	}

	dnsRecords, err := collectDNSRecords(ctx, client, zoneID)
	if err != nil {
		return nil, err
	}
	fixture := Fixture{Provider: "cloudflare", Zone: zoneName}
	for _, record := range dnsRecords {
		fixture.DNSRecords = append(fixture.DNSRecords, recordToFixture(record))
	}

	settings := collectZoneSettings(ctx, client, zoneID, &warnings)
	fixture.TLSSettings = append(fixture.TLSSettings, settings.tlsSetting(zoneID, zoneName))
	fixture.Origins = append(fixture.Origins, originsFromDNS(fixture.DNSRecords, settings.SSLMode)...)
	if settings.AlwaysUseHTTPS {
		fixture.Redirects = append(fixture.Redirects, FixtureRedirectRule{
			ID:            "always-use-https",
			Name:          "Always Use HTTPS",
			Source:        "http://" + zoneName + "/*",
			Target:        "https://" + zoneName + "/$1",
			StatusCode:    301,
			PreserveQuery: true,
		})
	}

	collectRulesets(ctx, client, zoneID, &fixture, &warnings)
	pageRules, err := collectPageRules(ctx, client, zoneID)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect page rules", err))
	} else {
		fixture.PageRules = append(fixture.PageRules, pageRules...)
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "edge", "cloudflare/"+zoneName, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "edge/raw-cloudflare-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type cloudflareZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cloudflareDNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type cloudflareRuleset struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Phase       string           `json:"phase"`
	Rules       []cloudflareRule `json:"rules"`
}

type cloudflareRule struct {
	ID               string         `json:"id"`
	Ref              string         `json:"ref"`
	Description      string         `json:"description"`
	Expression       string         `json:"expression"`
	Action           string         `json:"action"`
	Enabled          *bool          `json:"enabled"`
	ActionParameters map[string]any `json:"action_parameters"`
}

type cloudflarePageRule struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Targets  []struct {
		Constraint struct {
			Value string `json:"value"`
		} `json:"constraint"`
	} `json:"targets"`
	Actions []struct {
		ID    string `json:"id"`
		Value any    `json:"value"`
	} `json:"actions"`
}

type cloudflareSetting struct {
	ID    string `json:"id"`
	Value any    `json:"value"`
}

type zoneSettings struct {
	SSLMode        string
	MinTLSVersion  string
	HSTSEnabled    bool
	AlwaysUseHTTPS bool
}

func collectZone(ctx context.Context, client *Client, zoneID string) (*cloudflareZone, error) {
	var zone cloudflareZone
	_, err := client.getResult(ctx, "/zones/"+url.PathEscape(zoneID), nil, &zone)
	return &zone, err
}

func collectDNSRecords(ctx context.Context, client *Client, zoneID string) ([]cloudflareDNSRecord, error) {
	return getPagedResult[cloudflareDNSRecord](ctx, client, "/zones/"+url.PathEscape(zoneID)+"/dns_records", nil)
}

func collectPageRules(ctx context.Context, client *Client, zoneID string) ([]FixturePageRule, error) {
	pageRules, err := getPagedResult[cloudflarePageRule](ctx, client, "/zones/"+url.PathEscape(zoneID)+"/pagerules", nil)
	if err != nil {
		if apiStatus(err) == http.StatusForbidden || apiStatus(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]FixturePageRule, 0, len(pageRules))
	for _, pageRule := range pageRules {
		target := ""
		if len(pageRule.Targets) > 0 {
			target = pageRule.Targets[0].Constraint.Value
		}
		if target == "" {
			target = pageRule.ID
		}
		out = append(out, FixturePageRule{
			ID:       pageRule.ID,
			Target:   target,
			Actions:  pageRuleActions(pageRule.Actions),
			Priority: pageRule.Priority,
			Enabled:  strings.EqualFold(pageRule.Status, "active"),
		})
	}
	return out, nil
}

func collectZoneSettings(ctx context.Context, client *Client, zoneID string, warnings *[]string) zoneSettings {
	out := zoneSettings{SSLMode: "unknown", MinTLSVersion: "unknown"}
	if setting, err := collectZoneSetting(ctx, client, zoneID, "ssl"); err == nil {
		out.SSLMode = valueString(setting.Value)
	} else {
		*warnings = append(*warnings, optionalWarning("collect ssl setting", err))
	}
	if setting, err := collectZoneSetting(ctx, client, zoneID, "min_tls_version"); err == nil {
		out.MinTLSVersion = valueString(setting.Value)
	} else {
		*warnings = append(*warnings, optionalWarning("collect min_tls_version setting", err))
	}
	if setting, err := collectZoneSetting(ctx, client, zoneID, "always_use_https"); err == nil {
		out.AlwaysUseHTTPS = strings.EqualFold(valueString(setting.Value), "on")
	} else {
		*warnings = append(*warnings, optionalWarning("collect always_use_https setting", err))
	}
	if setting, err := collectZoneSetting(ctx, client, zoneID, "security_header"); err == nil {
		out.HSTSEnabled = hstsEnabled(setting.Value)
	} else {
		*warnings = append(*warnings, optionalWarning("collect security_header setting", err))
	}
	return out
}

func collectZoneSetting(ctx context.Context, client *Client, zoneID, settingID string) (*cloudflareSetting, error) {
	var setting cloudflareSetting
	_, err := client.getResult(ctx, "/zones/"+url.PathEscape(zoneID)+"/settings/"+url.PathEscape(settingID), nil, &setting)
	return &setting, err
}

func (settings zoneSettings) tlsSetting(zoneID, zoneName string) FixtureTLSSetting {
	return FixtureTLSSetting{
		ID:          "zone-" + zoneID,
		Hostname:    zoneName,
		Mode:        firstNonEmpty(settings.SSLMode, "unknown"),
		MinVersion:  firstNonEmpty(settings.MinTLSVersion, "unknown"),
		HSTSEnabled: settings.HSTSEnabled,
	}
}

func collectRulesets(ctx context.Context, client *Client, zoneID string, fixture *Fixture, warnings *[]string) {
	for _, phase := range []string{"http_request_firewall_custom", "http_request_firewall_managed"} {
		ruleset, err := collectRulesetPhase(ctx, client, zoneID, phase)
		if err != nil {
			*warnings = append(*warnings, optionalWarning("collect ruleset phase "+phase, err))
			continue
		}
		if ruleset == nil {
			continue
		}
		managed := phase == "http_request_firewall_managed"
		fixture.WAFRules = append(fixture.WAFRules, wafRulesFromRuleset(*ruleset, managed)...)
		fixture.BotRules = append(fixture.BotRules, botRulesFromRuleset(*ruleset)...)
	}
	if ruleset, err := collectRulesetPhase(ctx, client, zoneID, "http_request_cache_settings"); err != nil {
		*warnings = append(*warnings, optionalWarning("collect cache ruleset", err))
	} else if ruleset != nil {
		fixture.CacheRules = append(fixture.CacheRules, cacheRulesFromRuleset(*ruleset)...)
	}
	if ruleset, err := collectRulesetPhase(ctx, client, zoneID, "http_request_dynamic_redirect"); err != nil {
		*warnings = append(*warnings, optionalWarning("collect redirect ruleset", err))
	} else if ruleset != nil {
		fixture.Redirects = append(fixture.Redirects, redirectsFromRuleset(*ruleset)...)
	}
	if ruleset, err := collectRulesetPhase(ctx, client, zoneID, "http_request_sbfm"); err != nil {
		*warnings = append(*warnings, optionalWarning("collect bot ruleset", err))
	} else if ruleset != nil {
		fixture.BotRules = append(fixture.BotRules, botRulesFromRuleset(*ruleset)...)
	}
}

func collectRulesetPhase(ctx context.Context, client *Client, zoneID, phase string) (*cloudflareRuleset, error) {
	var ruleset cloudflareRuleset
	_, err := client.getResult(ctx, "/zones/"+url.PathEscape(zoneID)+"/rulesets/phases/"+url.PathEscape(phase)+"/entrypoint", nil, &ruleset)
	if err != nil {
		if apiStatus(err) == http.StatusForbidden || apiStatus(err) == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ruleset, nil
}

func recordToFixture(record cloudflareDNSRecord) FixtureDNSRecord {
	return FixtureDNSRecord{
		ID:      firstNonEmpty(record.ID, safeID(record.Name+"-"+record.Type)),
		Name:    record.Name,
		Type:    record.Type,
		Value:   inventory.RedactString(record.Content),
		TTL:     record.TTL,
		Proxied: record.Proxied,
	}
}

func wafRulesFromRuleset(ruleset cloudflareRuleset, managed bool) []FixtureWAFRule {
	out := make([]FixtureWAFRule, 0, len(ruleset.Rules))
	for _, rule := range ruleset.Rules {
		out = append(out, FixtureWAFRule{
			ID:         ruleID(rule),
			Name:       firstNonEmpty(rule.Description, rule.Ref, rule.ID, ruleset.Name),
			Expression: rule.Expression,
			Action:     firstNonEmpty(rule.Action, "unknown"),
			Enabled:    ruleEnabled(rule),
			Managed:    managed || strings.EqualFold(rule.Action, "execute"),
		})
	}
	return out
}

func cacheRulesFromRuleset(ruleset cloudflareRuleset) []FixtureCacheRule {
	out := make([]FixtureCacheRule, 0, len(ruleset.Rules))
	for _, rule := range ruleset.Rules {
		action := "cache"
		if value, ok := boolAt(rule.ActionParameters, "cache"); ok && !value {
			action = "bypass"
		}
		out = append(out, FixtureCacheRule{
			ID:         ruleID(rule),
			Name:       firstNonEmpty(rule.Description, rule.Ref, rule.ID, ruleset.Name),
			Pattern:    firstNonEmpty(rule.Expression, "true"),
			Action:     action,
			EdgeTTL:    intAt(rule.ActionParameters, "edge_ttl", "default"),
			BrowserTTL: intAt(rule.ActionParameters, "browser_ttl", "default"),
			CacheKey:   uniqueSortedStrings(flattenParameterKeys(rule.ActionParameters["cache_key"])),
		})
	}
	return out
}

func redirectsFromRuleset(ruleset cloudflareRuleset) []FixtureRedirectRule {
	out := make([]FixtureRedirectRule, 0, len(ruleset.Rules))
	for _, rule := range ruleset.Rules {
		target := stringAt(rule.ActionParameters, "from_value", "target_url", "value")
		if target == "" {
			target = stringAt(rule.ActionParameters, "from_value", "target_url", "expression")
		}
		statusCode := intAt(rule.ActionParameters, "from_value", "status_code")
		if statusCode == 0 {
			statusCode = 302
		}
		preserveQuery, _ := boolAt(rule.ActionParameters, "from_value", "preserve_query_string")
		out = append(out, FixtureRedirectRule{
			ID:            ruleID(rule),
			Name:          firstNonEmpty(rule.Description, rule.Ref, rule.ID, ruleset.Name),
			Source:        firstNonEmpty(rule.Expression, "true"),
			Target:        firstNonEmpty(target, "manual-review-required"),
			StatusCode:    statusCode,
			PreserveQuery: preserveQuery,
		})
	}
	return out
}

func botRulesFromRuleset(ruleset cloudflareRuleset) []FixtureBotRule {
	var out []FixtureBotRule
	for _, rule := range ruleset.Rules {
		text := strings.ToLower(rule.Expression + " " + rule.Description + " " + rule.Action)
		if !strings.Contains(text, "bot") && !strings.Contains(text, "cf.bot_management") && !strings.Contains(text, "sbfm") {
			continue
		}
		out = append(out, FixtureBotRule{
			ID:         ruleID(rule),
			Name:       firstNonEmpty(rule.Description, rule.Ref, rule.ID, ruleset.Name),
			Expression: rule.Expression,
			Action:     firstNonEmpty(rule.Action, "unknown"),
			Enabled:    ruleEnabled(rule),
		})
	}
	return out
}

func originsFromDNS(records []FixtureDNSRecord, sslMode string) []FixtureOriginConfig {
	var out []FixtureOriginConfig
	seen := map[string]struct{}{}
	for _, record := range records {
		switch strings.ToUpper(record.Type) {
		case "A", "AAAA", "CNAME":
		default:
			continue
		}
		if _, ok := seen[record.Name]; ok {
			continue
		}
		seen[record.Name] = struct{}{}
		scheme, port, tlsVerify := originTLSDefaults(sslMode)
		out = append(out, FixtureOriginConfig{
			ID:          "origin-" + record.ID,
			Hostname:    firstNonEmpty(record.Value, record.Name),
			Scheme:      scheme,
			Port:        port,
			HostHeader:  record.Name,
			TLSVerify:   tlsVerify,
			HealthCheck: false,
		})
	}
	return out
}

func originTLSDefaults(sslMode string) (string, int, bool) {
	switch strings.ToLower(strings.TrimSpace(sslMode)) {
	case "off", "flexible":
		return "http", 80, false
	case "strict", "full_strict":
		return "https", 443, true
	default:
		return "https", 443, false
	}
}

func pageRuleActions(actions []struct {
	ID    string `json:"id"`
	Value any    `json:"value"`
}) []string {
	var out []string
	for _, action := range actions {
		if action.ID == "" {
			continue
		}
		if value := valueString(action.Value); value != "" {
			out = append(out, action.ID+":"+value)
			continue
		}
		out = append(out, action.ID)
	}
	return uniqueSortedStrings(out)
}

func hstsEnabled(value any) bool {
	if enabled, ok := boolAt(map[string]any{"value": value}, "value", "strict_transport_security", "enabled"); ok {
		return enabled
	}
	return false
}

func ruleID(rule cloudflareRule) string {
	return firstNonEmpty(rule.ID, rule.Ref, safeID(rule.Description))
}

func ruleEnabled(rule cloudflareRule) bool {
	if rule.Enabled == nil {
		return true
	}
	return *rule.Enabled
}

func stringAt(values map[string]any, path ...string) string {
	current, ok := nestedValue(values, path...)
	if !ok {
		return ""
	}
	return valueString(current)
}

func intAt(values map[string]any, path ...string) int {
	current, ok := nestedValue(values, path...)
	if !ok {
		return 0
	}
	switch typed := current.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		value, _ := typed.Int64()
		return int(value)
	case string:
		value, _ := strconv.Atoi(typed)
		return value
	default:
		return 0
	}
}

func boolAt(values map[string]any, path ...string) (bool, bool) {
	current, ok := nestedValue(values, path...)
	if !ok {
		return false, false
	}
	value, ok := current.(bool)
	return value, ok
}

func nestedValue(values map[string]any, path ...string) (any, bool) {
	var current any = values
	for _, part := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func flattenParameterKeys(value any) []string {
	var out []string
	walkParameterKeys("", value, &out)
	return out
}

func walkParameterKeys(prefix string, value any, out *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			walkParameterKeys(next, child, out)
		}
	case []any:
		for i, child := range typed {
			walkParameterKeys(prefix+"["+strconv.Itoa(i)+"]", child, out)
		}
	case string:
		if typed != "" {
			*out = append(*out, prefix+":"+typed)
		}
	case bool:
		if typed {
			*out = append(*out, prefix)
		}
	case float64:
		*out = append(*out, prefix+":"+strconv.FormatFloat(typed, 'f', -1, 64))
	}
}

func optionalWarning(label string, err error) string {
	return label + ": " + err.Error()
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
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func option(options map[string]string, key, fallback string) string {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	return value
}

func prettyJSON(value any) []byte {
	data, _ := json.MarshalIndent(value, "", "  ")
	return data
}
