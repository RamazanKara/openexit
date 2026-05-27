package edge

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type AkamaiCollector struct{}

func (AkamaiCollector) Name() string { return "akamai" }

func (AkamaiCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	credentials, err := akamaiCredentialsFromOptions(req.Options)
	if err != nil {
		return nil, err
	}
	accountSwitchKey := firstNonEmpty(req.Options["account-switch-key"], credentials.AccountSwitchKey)
	client, err := NewAkamaiClient(req.Options["base-url"], credentials, accountSwitchKey)
	if err != nil {
		return nil, err
	}

	zones := splitListOption(req.Options["zones"])
	propertyIDs := splitListOption(req.Options["property-ids"])
	securityConfigs := splitListOption(req.Options["security-config-ids"])
	discoverProperties := boolOption(req.Options, "discover-properties", false)
	discoverSecurity := boolOption(req.Options, "discover-security-configs", false)
	contractID := strings.TrimSpace(req.Options["contract-id"])
	groupID := strings.TrimSpace(req.Options["group-id"])
	securityVersion := intOption(req.Options, "security-version", 0)

	if discoverProperties && len(propertyIDs) == 0 {
		properties, err := collectAkamaiProperties(ctx, client, contractID, groupID)
		if err != nil {
			return nil, err
		}
		for _, property := range properties {
			propertyIDs = append(propertyIDs, property.PropertyID)
		}
	}

	if len(zones) == 0 && len(propertyIDs) == 0 && len(securityConfigs) == 0 && !discoverSecurity {
		return nil, fmt.Errorf("at least one --zone, --property-id, --security-config-id, --discover-properties, or --discover-security-configs value is required")
	}

	fixture := Fixture{Provider: "akamai", Zone: akamaiInventoryLabel(zones, propertyIDs, securityConfigs)}
	var warnings []string

	for _, zone := range zones {
		records, err := collectAkamaiDNSRecordsets(ctx, client, zone)
		if err != nil {
			return nil, err
		}
		fixture.DNSRecords = append(fixture.DNSRecords, records...)
	}

	for _, propertyID := range propertyIDs {
		collected, err := collectAkamaiProperty(ctx, client, propertyID, contractID, groupID)
		if err != nil {
			return nil, err
		}
		fixture.DNSRecords = append(fixture.DNSRecords, collected.DNSRecords...)
		fixture.WAFRules = append(fixture.WAFRules, collected.WAFRules...)
		fixture.CacheRules = append(fixture.CacheRules, collected.CacheRules...)
		fixture.Redirects = append(fixture.Redirects, collected.Redirects...)
		fixture.Origins = append(fixture.Origins, collected.Origins...)
		fixture.TLSSettings = append(fixture.TLSSettings, collected.TLSSettings...)
		fixture.BotRules = append(fixture.BotRules, collected.BotRules...)
		fixture.PageRules = append(fixture.PageRules, collected.PageRules...)
	}

	if discoverSecurity {
		configs, err := collectAkamaiSecurityConfigs(ctx, client)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect Akamai security configurations", err))
		} else {
			for _, config := range configs {
				if config.ID != 0 {
					securityConfigs = append(securityConfigs, strconv.Itoa(config.ID)+":"+strconv.Itoa(config.LatestVersion))
				}
			}
		}
	}

	if len(securityConfigs) > 0 {
		knownConfigs := map[int]akamaiSecurityConfig{}
		configs, err := collectAkamaiSecurityConfigs(ctx, client)
		if err == nil {
			for _, config := range configs {
				knownConfigs[config.ID] = config
			}
		} else {
			warnings = append(warnings, optionalWarning("collect Akamai security configuration metadata", err))
		}
		for _, value := range securityConfigs {
			ref, err := parseAkamaiSecurityRef(value, securityVersion, knownConfigs)
			if err != nil {
				warnings = append(warnings, optionalWarning("parse Akamai security config "+value, err))
				continue
			}
			rules, bots, err := collectAkamaiSecurityPolicyMetadata(ctx, client, ref)
			if err != nil {
				warnings = append(warnings, optionalWarning("collect Akamai security config "+strconv.Itoa(ref.ConfigID), err))
				continue
			}
			fixture.WAFRules = append(fixture.WAFRules, rules...)
			fixture.BotRules = append(fixture.BotRules, bots...)
		}
	}

	dedupeFixture(&fixture)
	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "edge", "akamai/"+fixture.Zone, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "edge/raw-akamai-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type AkamaiCredentials struct {
	Host             string
	ClientToken      string
	AccessToken      string
	ClientSecret     string
	AccountSwitchKey string
}

type AkamaiClient struct {
	baseURL          string
	credentials      AkamaiCredentials
	accountSwitchKey string
	http             *http.Client
}

type akamaiAPIError struct {
	Path       string
	Status     string
	StatusCode int
	Body       string
}

func (e *akamaiAPIError) Error() string {
	return fmt.Sprintf("akamai API %s returned %s: %s", e.Path, e.Status, e.Body)
}

func NewAkamaiClient(baseURL string, credentials AkamaiCredentials, accountSwitchKey string) (*AkamaiClient, error) {
	if strings.TrimSpace(credentials.ClientToken) == "" {
		return nil, fmt.Errorf("akamai client token is required")
	}
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return nil, fmt.Errorf("akamai access token is required")
	}
	if strings.TrimSpace(credentials.ClientSecret) == "" {
		return nil, fmt.Errorf("akamai client secret is required")
	}
	normalized, err := akamaiBaseURL(baseURL, credentials.Host)
	if err != nil {
		return nil, err
	}
	return &AkamaiClient{
		baseURL:          normalized,
		credentials:      credentials,
		accountSwitchKey: strings.TrimSpace(accountSwitchKey),
		http:             &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func akamaiBaseURL(raw, host string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		host = strings.TrimSpace(host)
		if host == "" {
			return "", fmt.Errorf("akamai host is required when --base-url is not set")
		}
		host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
		raw = "https://" + host
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("akamai base URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("akamai base URL requires a host")
	}
	return strings.TrimRight(raw, "/"), nil
}

func (c *AkamaiClient) get(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	if query == nil {
		query = url.Values{}
	} else {
		query = cloneValues(query)
	}
	if c.accountSwitchKey != "" && query.Get("accountSwitchKey") == "" {
		query.Set("accountSwitchKey", c.accountSwitchKey)
	}
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
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
		if strings.HasPrefix(path, "/papi/") {
			req.Header.Set("PAPI-Use-Prefixes", "true")
		}
		if err := c.sign(req); err != nil {
			return nil, err
		}

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
			lastErr = &akamaiAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
			time.Sleep(retryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &akamaiAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
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

func (c *AkamaiClient) sign(req *http.Request) error {
	timestamp := time.Now().UTC().Format("20060102T15:04:05+0000")
	nonce := randomNonce()
	authData := fmt.Sprintf("EG1-HMAC-SHA256 client_token=%s;access_token=%s;timestamp=%s;nonce=%s;", c.credentials.ClientToken, c.credentials.AccessToken, timestamp, nonce)
	dataToSign := strings.Join([]string{
		req.Method,
		req.URL.Scheme,
		req.URL.Host,
		req.URL.RequestURI(),
		authData,
	}, "\t")
	signingKey := base64.StdEncoding.EncodeToString(hmacSHA256([]byte(c.credentials.ClientSecret), []byte(timestamp)))
	signature := base64.StdEncoding.EncodeToString(hmacSHA256([]byte(signingKey), []byte(dataToSign)))
	req.Header.Set("Authorization", authData+"signature="+signature)
	return nil
}

func hmacSHA256(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(value)
	return mac.Sum(nil)
}

func randomNonce() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

type akamaiDNSRecordsetsResponse struct {
	Recordsets []akamaiDNSRecordset `json:"recordsets"`
}

type akamaiDNSRecordset struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	TTL   int      `json:"ttl"`
	RData []string `json:"rdata"`
}

func collectAkamaiDNSRecordsets(ctx context.Context, client *AkamaiClient, zone string) ([]FixtureDNSRecord, error) {
	var response akamaiDNSRecordsetsResponse
	path := "/config-dns/v2/zones/" + url.PathEscape(zone) + "/recordsets"
	if _, err := client.get(ctx, path, nil, &response); err != nil {
		return nil, err
	}
	out := make([]FixtureDNSRecord, 0, len(response.Recordsets))
	for _, record := range response.Recordsets {
		if record.Name == "" || record.Type == "" {
			continue
		}
		value := strings.Join(redactStrings(record.RData), ", ")
		out = append(out, FixtureDNSRecord{
			ID:      safeID("akamai-dns-" + zone + "-" + record.Name + "-" + record.Type),
			Name:    record.Name,
			Type:    strings.ToUpper(record.Type),
			Value:   value,
			TTL:     record.TTL,
			Proxied: akamaiProxiedRecord(record.RData),
		})
	}
	return out, nil
}

type akamaiProperty struct {
	PropertyID        string `json:"propertyId"`
	PropertyName      string `json:"propertyName"`
	LatestVersion     int    `json:"latestVersion"`
	ProductionVersion int    `json:"productionVersion"`
	ContractID        string `json:"contractId"`
	GroupID           string `json:"groupId"`
}

type akamaiPropertiesResponse struct {
	Properties struct {
		Items []akamaiProperty `json:"items"`
	} `json:"properties"`
	Items []akamaiProperty `json:"items"`
}

type akamaiPropertyVersionResponse struct {
	PropertyVersion int `json:"propertyVersion"`
	Versions        struct {
		Items []akamaiProperty `json:"items"`
	} `json:"versions"`
}

type akamaiHostnamesResponse struct {
	Hostnames struct {
		Items []akamaiPropertyHostname `json:"items"`
	} `json:"hostnames"`
	Items []akamaiPropertyHostname `json:"items"`
}

type akamaiPropertyHostname struct {
	CNameFrom             string `json:"cnameFrom"`
	CNameTo               string `json:"cnameTo"`
	CertProvisioningType  string `json:"certProvisioningType"`
	EdgeHostnameID        string `json:"edgeHostnameId"`
	CertStatus            any    `json:"certStatus"`
	CertificateStatus     string `json:"certificateStatus"`
	CertificateNetwork    string `json:"certificateNetwork"`
	StagingCertStatus     string `json:"stagingCertStatus"`
	ProductionCertStatus  string `json:"productionCertStatus"`
	StagingCnameTo        string `json:"stagingCnameTo"`
	ProductionCnameTo     string `json:"productionCnameTo"`
	StagingEdgeHostnameID string `json:"stagingEdgeHostnameId"`
}

type akamaiRulesResponse struct {
	PropertyID      string     `json:"propertyId"`
	PropertyName    string     `json:"propertyName"`
	PropertyVersion int        `json:"propertyVersion"`
	Rules           akamaiRule `json:"rules"`
}

type akamaiRule struct {
	Name      string           `json:"name"`
	Criteria  []akamaiBehavior `json:"criteria"`
	Behaviors []akamaiBehavior `json:"behaviors"`
	Children  []akamaiRule     `json:"children"`
}

type akamaiBehavior struct {
	Name    string         `json:"name"`
	Options map[string]any `json:"options"`
}

func collectAkamaiProperties(ctx context.Context, client *AkamaiClient, contractID, groupID string) ([]akamaiProperty, error) {
	query := akamaiPAPIQuery(contractID, groupID)
	var response akamaiPropertiesResponse
	if _, err := client.get(ctx, "/papi/v1/properties", query, &response); err != nil {
		return nil, err
	}
	if len(response.Properties.Items) > 0 {
		return response.Properties.Items, nil
	}
	return response.Items, nil
}

func collectAkamaiProperty(ctx context.Context, client *AkamaiClient, propertyID, contractID, groupID string) (*Fixture, error) {
	query := akamaiPAPIQuery(contractID, groupID)
	versionNumber, err := collectAkamaiPropertyVersion(ctx, client, propertyID, query)
	if err != nil {
		return nil, err
	}

	fixture := &Fixture{Provider: "akamai", Zone: propertyID}
	hostnames, err := collectAkamaiPropertyHostnames(ctx, client, propertyID, versionNumber, query)
	if err != nil {
		return nil, err
	}
	for _, hostname := range hostnames {
		fixture.DNSRecords = append(fixture.DNSRecords, hostname.toDNSRecord(propertyID))
		fixture.TLSSettings = append(fixture.TLSSettings, hostname.toTLSSetting(propertyID))
	}

	var rules akamaiRulesResponse
	path := fmt.Sprintf("/papi/v1/properties/%s/versions/%d/rules", url.PathEscape(propertyID), versionNumber)
	if _, err := client.get(ctx, path, query, &rules); err != nil {
		return nil, err
	}
	rules.Rules.walk(propertyID, firstNonEmpty(rules.PropertyName, propertyID), fixture)
	return fixture, nil
}

func collectAkamaiPropertyVersion(ctx context.Context, client *AkamaiClient, propertyID string, query url.Values) (int, error) {
	var response akamaiPropertyVersionResponse
	path := "/papi/v1/properties/" + url.PathEscape(propertyID) + "/versions/latest"
	if _, err := client.get(ctx, path, query, &response); err != nil {
		return 0, err
	}
	if response.PropertyVersion > 0 {
		return response.PropertyVersion, nil
	}
	for _, item := range response.Versions.Items {
		if item.ProductionVersion > 0 {
			return item.ProductionVersion, nil
		}
		if item.LatestVersion > 0 {
			return item.LatestVersion, nil
		}
	}
	return 0, fmt.Errorf("akamai property %s latest version response did not include propertyVersion", propertyID)
}

func collectAkamaiPropertyHostnames(ctx context.Context, client *AkamaiClient, propertyID string, versionNumber int, query url.Values) ([]akamaiPropertyHostname, error) {
	var response akamaiHostnamesResponse
	path := fmt.Sprintf("/papi/v1/properties/%s/versions/%d/hostnames", url.PathEscape(propertyID), versionNumber)
	if _, err := client.get(ctx, path, query, &response); err != nil {
		return nil, err
	}
	if len(response.Hostnames.Items) > 0 {
		return response.Hostnames.Items, nil
	}
	return response.Items, nil
}

func (hostname akamaiPropertyHostname) toDNSRecord(propertyID string) FixtureDNSRecord {
	name := firstNonEmpty(hostname.CNameFrom, hostname.EdgeHostnameID)
	value := firstNonEmpty(hostname.CNameTo, hostname.ProductionCnameTo, hostname.StagingCnameTo)
	return FixtureDNSRecord{
		ID:      safeID("akamai-hostname-" + propertyID + "-" + name),
		Name:    name,
		Type:    "CNAME",
		Value:   inventory.RedactString(value),
		TTL:     0,
		Proxied: true,
	}
}

func (hostname akamaiPropertyHostname) toTLSSetting(propertyID string) FixtureTLSSetting {
	name := firstNonEmpty(hostname.CNameFrom, hostname.EdgeHostnameID)
	mode := firstNonEmpty(hostname.CertProvisioningType, hostname.CertificateStatus, hostname.ProductionCertStatus, "edge-certificate")
	return FixtureTLSSetting{
		ID:          safeID("akamai-tls-" + propertyID + "-" + name),
		Hostname:    name,
		Mode:        mode,
		MinVersion:  "unknown",
		HSTSEnabled: false,
	}
}

func (rule akamaiRule) walk(propertyID, propertyName string, fixture *Fixture) {
	pattern := rule.pattern(propertyName)
	for _, behavior := range rule.Behaviors {
		switch strings.ToLower(behavior.Name) {
		case "origin":
			if origin, ok := akamaiOriginFromBehavior(propertyID, rule.Name, behavior); ok {
				fixture.Origins = append(fixture.Origins, origin)
			}
		case "caching", "cachekeyqueryparams", "cachekeyincludequeryparams", "cachekeyignorecase", "cachekeyqueryparameterbehavior":
			if cacheRule, ok := akamaiCacheRuleFromBehavior(propertyID, rule.Name, pattern, behavior); ok {
				fixture.CacheRules = append(fixture.CacheRules, cacheRule)
			}
		case "redirect", "redirectplus", "edgeserverredirect", "responsecode":
			if redirect, ok := akamaiRedirectFromBehavior(propertyID, rule.Name, pattern, behavior); ok {
				fixture.Redirects = append(fixture.Redirects, redirect)
			}
		case "httpstricttransportsecurity", "hsts":
			if tlsSetting, ok := akamaiHSTSFromBehavior(propertyID, rule.Name, propertyName, behavior); ok {
				fixture.TLSSettings = append(fixture.TLSSettings, tlsSetting)
			}
		case "applicationsecurity", "webapplicationfirewall", "firewallrules", "slowpostprotection":
			fixture.WAFRules = append(fixture.WAFRules, akamaiWAFRuleFromBehavior(propertyID, rule.Name, pattern, behavior))
		case "botman", "botmanager", "clientreputation":
			fixture.BotRules = append(fixture.BotRules, akamaiBotRuleFromBehavior(propertyID, rule.Name, pattern, behavior))
		}
	}
	for _, child := range rule.Children {
		child.walk(propertyID, propertyName, fixture)
	}
}

func (rule akamaiRule) pattern(propertyName string) string {
	if len(rule.Criteria) == 0 {
		return "property:" + propertyName
	}
	var parts []string
	for _, criterion := range rule.Criteria {
		if criterion.Name == "" {
			continue
		}
		if len(criterion.Options) == 0 {
			parts = append(parts, criterion.Name)
			continue
		}
		parts = append(parts, criterion.Name+"("+strings.Join(flattenParameterKeys(criterion.Options), ",")+")")
	}
	if len(parts) == 0 {
		return "property:" + propertyName
	}
	sort.Strings(parts)
	return strings.Join(parts, " && ")
}

func akamaiOriginFromBehavior(propertyID, ruleName string, behavior akamaiBehavior) (FixtureOriginConfig, bool) {
	hostname := stringAt(behavior.Options, "hostname")
	if hostname == "" {
		return FixtureOriginConfig{}, false
	}
	scheme := "https"
	port := intAt(behavior.Options, "httpsPort")
	if port == 0 {
		port = intAt(behavior.Options, "httpPort")
		if port > 0 {
			scheme = "http"
		}
	}
	if port == 0 {
		port = 443
	}
	hostHeader := hostname
	if strings.EqualFold(stringAt(behavior.Options, "forwardHostHeader"), "REQUEST_HOST_HEADER") {
		hostHeader = "request-host-header"
	}
	verificationMode := strings.ToLower(stringAt(behavior.Options, "verificationMode"))
	tlsVerify := scheme == "https" && !strings.Contains(verificationMode, "none")
	return FixtureOriginConfig{
		ID:          safeID("akamai-origin-" + propertyID + "-" + ruleName + "-" + hostname),
		Hostname:    inventory.RedactString(hostname),
		Scheme:      scheme,
		Port:        port,
		HostHeader:  hostHeader,
		TLSVerify:   tlsVerify,
		HealthCheck: boolAtDefault(behavior.Options, false, "originHealthCheck", "enabled") || boolAtDefault(behavior.Options, false, "healthCheck", "enabled"),
	}, true
}

func akamaiCacheRuleFromBehavior(propertyID, ruleName, pattern string, behavior akamaiBehavior) (FixtureCacheRule, bool) {
	name := firstNonEmpty(ruleName, behavior.Name)
	action := "cache"
	text := strings.ToLower(strings.Join(flattenParameterKeys(behavior.Options), " "))
	if strings.Contains(text, "no-store") || strings.Contains(text, "bypass") || strings.Contains(text, "never") {
		action = "bypass"
	} else if strings.Contains(text, "honor") || strings.Contains(text, "cache_control") {
		action = "honor-origin"
	}
	ttl := firstPositiveInt(
		intAt(behavior.Options, "ttl"),
		intAt(behavior.Options, "defaultTtl"),
		intAt(behavior.Options, "maxAge"),
		intAt(behavior.Options, "max_age"),
	)
	return FixtureCacheRule{
		ID:         safeID("akamai-cache-" + propertyID + "-" + name + "-" + behavior.Name),
		Name:       firstNonEmpty(name, "Akamai cache behavior"),
		Pattern:    firstNonEmpty(pattern, "property:"+propertyID),
		Action:     action,
		EdgeTTL:    ttl,
		BrowserTTL: firstPositiveInt(intAt(behavior.Options, "browserTtl"), intAt(behavior.Options, "clientTtl")),
		CacheKey:   uniqueSortedStrings(flattenParameterKeys(behavior.Options)),
	}, true
}

func akamaiRedirectFromBehavior(propertyID, ruleName, pattern string, behavior akamaiBehavior) (FixtureRedirectRule, bool) {
	target := firstNonEmpty(
		stringAt(behavior.Options, "destinationHostname"),
		stringAt(behavior.Options, "destinationPath"),
		stringAt(behavior.Options, "targetUrl"),
		stringAt(behavior.Options, "url"),
	)
	if target == "" && strings.EqualFold(stringAt(behavior.Options, "protocol"), "HTTPS") {
		target = "https://{host}{path}"
	}
	if target == "" {
		return FixtureRedirectRule{}, false
	}
	status := firstPositiveInt(intAt(behavior.Options, "responseCode"), intAt(behavior.Options, "statusCode"))
	if status == 0 {
		status = 301
	}
	return FixtureRedirectRule{
		ID:            safeID("akamai-redirect-" + propertyID + "-" + ruleName + "-" + behavior.Name),
		Name:          firstNonEmpty(ruleName, behavior.Name),
		Source:        firstNonEmpty(pattern, "property:"+propertyID),
		Target:        target,
		StatusCode:    status,
		PreserveQuery: !boolAtDefault(behavior.Options, false, "discardQueryString"),
	}, true
}

func akamaiHSTSFromBehavior(propertyID, ruleName, hostname string, behavior akamaiBehavior) (FixtureTLSSetting, bool) {
	enabled := boolAtDefault(behavior.Options, true, "enabled")
	return FixtureTLSSetting{
		ID:          safeID("akamai-hsts-" + propertyID + "-" + ruleName),
		Hostname:    firstNonEmpty(hostname, propertyID),
		Mode:        "hsts",
		MinVersion:  "unknown",
		HSTSEnabled: enabled,
	}, true
}

func akamaiWAFRuleFromBehavior(propertyID, ruleName, pattern string, behavior akamaiBehavior) FixtureWAFRule {
	action := "enabled"
	if !boolAtDefault(behavior.Options, true, "enabled") {
		action = "disabled"
	}
	if mode := firstNonEmpty(stringAt(behavior.Options, "mode"), stringAt(behavior.Options, "ruleSet"), stringAt(behavior.Options, "policy")); mode != "" {
		action = action + ":" + mode
	}
	return FixtureWAFRule{
		ID:         safeID("akamai-waf-" + propertyID + "-" + ruleName + "-" + behavior.Name),
		Name:       firstNonEmpty(ruleName, behavior.Name, "Akamai WAF behavior"),
		Expression: firstNonEmpty(pattern, "property:"+propertyID),
		Action:     action,
		Enabled:    boolAtDefault(behavior.Options, true, "enabled"),
		Managed:    true,
	}
}

func akamaiBotRuleFromBehavior(propertyID, ruleName, pattern string, behavior akamaiBehavior) FixtureBotRule {
	action := "enabled"
	if !boolAtDefault(behavior.Options, true, "enabled") {
		action = "disabled"
	}
	if mode := firstNonEmpty(stringAt(behavior.Options, "mode"), stringAt(behavior.Options, "action")); mode != "" {
		action = mode
	}
	return FixtureBotRule{
		ID:         safeID("akamai-bot-" + propertyID + "-" + ruleName + "-" + behavior.Name),
		Name:       firstNonEmpty(ruleName, behavior.Name, "Akamai bot behavior"),
		Expression: firstNonEmpty(pattern, "property:"+propertyID),
		Action:     action,
		Enabled:    boolAtDefault(behavior.Options, true, "enabled"),
	}
}

type akamaiSecurityConfig struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LatestVersion int    `json:"latestVersion"`
}

type akamaiSecurityConfigsResponse struct {
	Configurations []akamaiSecurityConfig `json:"configurations"`
}

type akamaiSecurityRef struct {
	ConfigID int
	Version  int
}

type akamaiPoliciesResponse struct {
	Policies         []akamaiPolicy `json:"policies"`
	SecurityPolicies []akamaiPolicy `json:"securityPolicies"`
}

type akamaiPolicy struct {
	PolicyID   string `json:"policyId"`
	ID         string `json:"id"`
	PolicyName string `json:"policyName"`
	Name       string `json:"name"`
}

type akamaiCustomRulesResponse struct {
	CustomRules []akamaiCustomRule `json:"customRules"`
	Rules       []akamaiCustomRule `json:"rules"`
}

type akamaiCustomRule struct {
	ID         any            `json:"id"`
	RuleID     any            `json:"ruleId"`
	Name       string         `json:"name"`
	Action     string         `json:"action"`
	Enabled    *bool          `json:"enabled"`
	Conditions []akamaiFilter `json:"conditions"`
	Tags       []string       `json:"tags"`
	Options    map[string]any `json:"options"`
}

type akamaiFilter struct {
	Name    string         `json:"name"`
	Options map[string]any `json:"options"`
}

type akamaiProtectionsResponse struct {
	Protections map[string]any `json:"protections"`
}

func collectAkamaiSecurityConfigs(ctx context.Context, client *AkamaiClient) ([]akamaiSecurityConfig, error) {
	var response akamaiSecurityConfigsResponse
	if _, err := client.get(ctx, "/appsec/v1/configs", nil, &response); err != nil {
		return nil, err
	}
	return response.Configurations, nil
}

func parseAkamaiSecurityRef(value string, fallbackVersion int, known map[int]akamaiSecurityConfig) (akamaiSecurityRef, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	configID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || configID <= 0 {
		return akamaiSecurityRef{}, fmt.Errorf("security config ID must be a positive integer")
	}
	versionNumber := fallbackVersion
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		versionNumber, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || versionNumber <= 0 {
			return akamaiSecurityRef{}, fmt.Errorf("security config version must be a positive integer")
		}
	}
	if versionNumber == 0 {
		if config, ok := known[configID]; ok {
			versionNumber = config.LatestVersion
		}
	}
	if versionNumber == 0 {
		return akamaiSecurityRef{}, fmt.Errorf("security config version is required when latestVersion is unavailable")
	}
	return akamaiSecurityRef{ConfigID: configID, Version: versionNumber}, nil
}

func collectAkamaiSecurityPolicyMetadata(ctx context.Context, client *AkamaiClient, ref akamaiSecurityRef) ([]FixtureWAFRule, []FixtureBotRule, error) {
	policies, err := collectAkamaiSecurityPolicies(ctx, client, ref)
	if err != nil {
		return nil, nil, err
	}
	var wafRules []FixtureWAFRule
	var botRules []FixtureBotRule
	for _, policy := range policies {
		policyID := policy.id()
		if policyID == "" {
			continue
		}
		customRules, err := collectAkamaiCustomRules(ctx, client, ref, policyID)
		if err != nil {
			return nil, nil, err
		}
		for _, rule := range customRules {
			wafRules = append(wafRules, rule.toWAFRule(ref, policy))
		}
		if bot, ok := collectAkamaiPolicyProtections(ctx, client, ref, policy); ok {
			botRules = append(botRules, bot)
		}
	}
	return wafRules, botRules, nil
}

func collectAkamaiSecurityPolicies(ctx context.Context, client *AkamaiClient, ref akamaiSecurityRef) ([]akamaiPolicy, error) {
	query := url.Values{"detail": []string{"true"}}
	var response akamaiPoliciesResponse
	path := fmt.Sprintf("/appsec/v1/configs/%d/versions/%d/security-policies", ref.ConfigID, ref.Version)
	if _, err := client.get(ctx, path, query, &response); err != nil {
		return nil, err
	}
	if len(response.Policies) > 0 {
		return response.Policies, nil
	}
	return response.SecurityPolicies, nil
}

func collectAkamaiCustomRules(ctx context.Context, client *AkamaiClient, ref akamaiSecurityRef, policyID string) ([]akamaiCustomRule, error) {
	var response akamaiCustomRulesResponse
	path := fmt.Sprintf("/appsec/v1/configs/%d/versions/%d/security-policies/%s/custom-rules", ref.ConfigID, ref.Version, url.PathEscape(policyID))
	if _, err := client.get(ctx, path, nil, &response); err != nil {
		return nil, err
	}
	if len(response.CustomRules) > 0 {
		return response.CustomRules, nil
	}
	return response.Rules, nil
}

func collectAkamaiPolicyProtections(ctx context.Context, client *AkamaiClient, ref akamaiSecurityRef, policy akamaiPolicy) (FixtureBotRule, bool) {
	var response akamaiProtectionsResponse
	path := fmt.Sprintf("/appsec/v1/configs/%d/versions/%d/security-policies/%s/protections", ref.ConfigID, ref.Version, url.PathEscape(policy.id()))
	if _, err := client.get(ctx, path, nil, &response); err != nil {
		return FixtureBotRule{}, false
	}
	for key, value := range response.Protections {
		if !strings.Contains(strings.ToLower(key), "bot") {
			continue
		}
		return FixtureBotRule{
			ID:         safeID(fmt.Sprintf("akamai-appsec-bot-%d-%d-%s-%s", ref.ConfigID, ref.Version, policy.id(), key)),
			Name:       firstNonEmpty(policy.name(), key),
			Expression: "appsec-policy:" + policy.id(),
			Action:     valueString(value),
			Enabled:    true,
		}, true
	}
	return FixtureBotRule{}, false
}

func (policy akamaiPolicy) id() string {
	return firstNonEmpty(policy.PolicyID, policy.ID)
}

func (policy akamaiPolicy) name() string {
	return firstNonEmpty(policy.PolicyName, policy.Name, policy.id())
}

func (rule akamaiCustomRule) toWAFRule(ref akamaiSecurityRef, policy akamaiPolicy) FixtureWAFRule {
	id := firstNonEmpty(valueString(rule.RuleID), valueString(rule.ID), rule.Name)
	enabled := true
	if rule.Enabled != nil {
		enabled = *rule.Enabled
	}
	action := firstNonEmpty(rule.Action, stringAt(rule.Options, "action"), "unknown")
	var expressions []string
	for _, condition := range rule.Conditions {
		if condition.Name == "" {
			continue
		}
		expressions = append(expressions, condition.Name+"("+strings.Join(flattenParameterKeys(condition.Options), ",")+")")
	}
	expression := firstNonEmpty(strings.Join(expressions, " && "), "appsec-policy:"+policy.id())
	return FixtureWAFRule{
		ID:         safeID(fmt.Sprintf("akamai-appsec-waf-%d-%d-%s-%s", ref.ConfigID, ref.Version, policy.id(), id)),
		Name:       firstNonEmpty(rule.Name, policy.name(), "Akamai custom rule"),
		Expression: expression,
		Action:     action,
		Enabled:    enabled,
		Managed:    false,
	}
}

func akamaiCredentialsFromOptions(options map[string]string) (AkamaiCredentials, error) {
	hostEnv := option(options, "host-env", "AKAMAI_HOST")
	clientTokenEnv := option(options, "client-token-env", "AKAMAI_CLIENT_TOKEN")
	accessTokenEnv := option(options, "access-token-env", "AKAMAI_ACCESS_TOKEN")
	clientSecretEnv := option(options, "client-secret-env", "AKAMAI_CLIENT_SECRET")
	accountKeyEnv := option(options, "account-switch-key-env", "AKAMAI_ACCOUNT_KEY")

	credentials := AkamaiCredentials{
		Host:             os.Getenv(hostEnv),
		ClientToken:      os.Getenv(clientTokenEnv),
		AccessToken:      os.Getenv(accessTokenEnv),
		ClientSecret:     os.Getenv(clientSecretEnv),
		AccountSwitchKey: os.Getenv(accountKeyEnv),
	}
	if credentials.valid() {
		return credentials, nil
	}

	edgercPath := option(options, "edgerc", "~/.edgerc")
	section := option(options, "edgerc-section", "default")
	fileCredentials, err := parseEdgeRC(edgercPath, section)
	if err != nil {
		return AkamaiCredentials{}, err
	}
	if credentials.Host == "" {
		credentials.Host = fileCredentials.Host
	}
	if credentials.ClientToken == "" {
		credentials.ClientToken = fileCredentials.ClientToken
	}
	if credentials.AccessToken == "" {
		credentials.AccessToken = fileCredentials.AccessToken
	}
	if credentials.ClientSecret == "" {
		credentials.ClientSecret = fileCredentials.ClientSecret
	}
	if credentials.AccountSwitchKey == "" {
		credentials.AccountSwitchKey = fileCredentials.AccountSwitchKey
	}
	if !credentials.valid() {
		return AkamaiCredentials{}, fmt.Errorf("akamai credentials require host, client_token, access_token, and client_secret from environment or %s [%s]", edgercPath, section)
	}
	return credentials, nil
}

func (credentials AkamaiCredentials) valid() bool {
	return strings.TrimSpace(credentials.Host) != "" &&
		strings.TrimSpace(credentials.ClientToken) != "" &&
		strings.TrimSpace(credentials.AccessToken) != "" &&
		strings.TrimSpace(credentials.ClientSecret) != ""
}

func parseEdgeRC(path, section string) (AkamaiCredentials, error) {
	path = expandHome(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return AkamaiCredentials{}, fmt.Errorf("read edgerc: %w", err)
	}
	current := ""
	values := map[string]string{}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			current = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if current != section {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return AkamaiCredentials{
		Host:             values["host"],
		ClientToken:      values["client_token"],
		AccessToken:      values["access_token"],
		ClientSecret:     values["client_secret"],
		AccountSwitchKey: values["account_key"],
	}, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func akamaiPAPIQuery(contractID, groupID string) url.Values {
	query := url.Values{}
	if strings.TrimSpace(contractID) != "" {
		query.Set("contractId", strings.TrimSpace(contractID))
	}
	if strings.TrimSpace(groupID) != "" {
		query.Set("groupId", strings.TrimSpace(groupID))
	}
	return query
}

func splitListOption(value string) []string {
	value = strings.ReplaceAll(value, ",", "\n")
	var out []string
	for _, part := range strings.Split(value, "\n") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func boolOption(options map[string]string, key string, fallback bool) bool {
	value := strings.TrimSpace(options[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
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

func boolAtDefault(values map[string]any, fallback bool, path ...string) bool {
	value, ok := boolAt(values, path...)
	if !ok {
		return fallback
	}
	return value
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func redactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, inventory.RedactString(value))
	}
	return out
}

func akamaiProxiedRecord(values []string) bool {
	for _, value := range values {
		value = strings.ToLower(value)
		if strings.Contains(value, "akamai") || strings.Contains(value, "edgekey.net") || strings.Contains(value, "edgesuite.net") || strings.Contains(value, "akamaiedge.net") {
			return true
		}
	}
	return false
}

func akamaiInventoryLabel(zones, properties, securityConfigs []string) string {
	if len(zones) > 0 {
		return zones[0]
	}
	if len(properties) > 0 {
		return properties[0]
	}
	if len(securityConfigs) > 0 {
		return "appsec-" + securityConfigs[0]
	}
	return "account"
}

func requestPath(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Path == "" {
		return endpoint
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}

func dedupeFixture(fixture *Fixture) {
	fixture.DNSRecords = dedupeByID(fixture.DNSRecords, func(value FixtureDNSRecord) string { return value.ID })
	fixture.WAFRules = dedupeByID(fixture.WAFRules, func(value FixtureWAFRule) string { return value.ID })
	fixture.CacheRules = dedupeByID(fixture.CacheRules, func(value FixtureCacheRule) string { return value.ID })
	fixture.Redirects = dedupeByID(fixture.Redirects, func(value FixtureRedirectRule) string { return value.ID })
	fixture.Origins = dedupeByID(fixture.Origins, func(value FixtureOriginConfig) string { return value.ID })
	fixture.TLSSettings = dedupeByID(fixture.TLSSettings, func(value FixtureTLSSetting) string { return value.ID })
	fixture.BotRules = dedupeByID(fixture.BotRules, func(value FixtureBotRule) string { return value.ID })
	fixture.PageRules = dedupeByID(fixture.PageRules, func(value FixturePageRule) string { return value.ID })
}

func dedupeByID[T any](values []T, id func(T) string) []T {
	seen := map[string]struct{}{}
	out := make([]T, 0, len(values))
	for _, value := range values {
		key := id(value)
		if key == "" {
			out = append(out, value)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
