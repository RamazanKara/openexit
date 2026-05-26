package identity

import (
	"context"
	"encoding/json"
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

func (LiveCollector) Name() string { return "okta" }

func (LiveCollector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	orgURL := strings.TrimSpace(req.Options["org-url"])
	tokenEnv := option(req.Options, "token-env", "OKTA_API_TOKEN")
	token, err := envSecret(tokenEnv)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(orgURL, token, option(req.Options, "auth-scheme", "SSWS"))
	if err != nil {
		return nil, err
	}

	var warnings []string
	fixture := Fixture{
		Provider: "okta",
		Tenant:   strings.TrimPrefix(strings.TrimPrefix(client.baseURL, "https://"), "http://"),
	}

	apps, err := collectApplications(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		groups, err := collectApplicationGroups(ctx, client, app.ID)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect application groups for "+app.ID, err))
		}
		fixture.Applications = append(fixture.Applications, appToFixture(app, groups))
	}

	groups, err := collectGroups(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect groups", err))
	}
	for _, group := range groups {
		members, err := countGroupMembers(ctx, client, group.ID)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect group members for "+group.ID, err))
		}
		fixture.Groups = append(fixture.Groups, groupToFixture(group, members))
	}

	policies, err := collectPolicies(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect policies", err))
	}
	fixture.Policies = append(fixture.Policies, policies...)

	factors, err := collectOrgFactors(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect org factors", err))
	}
	if len(factors) > 0 || len(policies) > 0 {
		fixture.MFASettings = append(fixture.MFASettings, FixtureMFASetting{
			Name:     "okta-org",
			Required: policiesRequireMFA(policies),
			Factors:  factors,
		})
	}

	for _, username := range splitOptionList(req.Options["break-glass-users"]) {
		account, err := collectBreakGlassAccount(ctx, client, username)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect break-glass user "+username, err))
			continue
		}
		fixture.BreakGlassAccounts = append(fixture.BreakGlassAccounts, account)
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "identity", client.baseURL, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "identity/raw-okta-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type oktaApp struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Label       string         `json:"label"`
	SignOnMode  string         `json:"signOnMode"`
	Status      string         `json:"status"`
	Profile     map[string]any `json:"profile"`
	Credentials struct {
		OAuthClient struct {
			ClientID string `json:"client_id"`
		} `json:"oauthClient"`
		Signing struct {
			Kid       string `json:"kid"`
			ExpiresAt string `json:"expiresAt"`
		} `json:"signing"`
	} `json:"credentials"`
	Settings struct {
		OAuthClient struct {
			RedirectURIs []string `json:"redirect_uris"`
			GrantTypes   []string `json:"grant_types"`
		} `json:"oauthClient"`
		SignOn struct {
			SsoAcsURL   string `json:"ssoAcsUrl"`
			Recipient   string `json:"recipient"`
			Destination string `json:"destination"`
			Audience    string `json:"audience"`
		} `json:"signOn"`
		App map[string]any `json:"app"`
	} `json:"settings"`
}

type oktaGroup struct {
	ID      string `json:"id"`
	Profile struct {
		Name   string `json:"name"`
		Owner  any    `json:"owner"`
		Owners any    `json:"owners"`
	} `json:"profile"`
}

type oktaPolicy struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Priority   int             `json:"priority"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
}

type oktaPolicyRule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Priority   int             `json:"priority"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
}

type oktaFactor struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	FactorType string `json:"factorType"`
	Status     string `json:"status"`
}

type oktaUser struct {
	ID      string `json:"id"`
	Profile struct {
		Login string `json:"login"`
		Email string `json:"email"`
	} `json:"profile"`
}

func collectApplications(ctx context.Context, client *Client) ([]oktaApp, error) {
	return getPagedArray[oktaApp](ctx, client, "/api/v1/apps", url.Values{"limit": {"200"}})
}

func collectApplicationGroups(ctx context.Context, client *Client, appID string) ([]oktaGroup, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, nil
	}
	return getPagedArray[oktaGroup](ctx, client, "/api/v1/apps/"+url.PathEscape(appID)+"/groups", url.Values{"limit": {"200"}})
}

func collectGroups(ctx context.Context, client *Client) ([]oktaGroup, error) {
	return getPagedArray[oktaGroup](ctx, client, "/api/v1/groups", url.Values{"limit": {"200"}})
}

func countGroupMembers(ctx context.Context, client *Client, groupID string) (int, error) {
	if strings.TrimSpace(groupID) == "" {
		return 0, nil
	}
	users, err := getPagedArray[oktaUser](ctx, client, "/api/v1/groups/"+url.PathEscape(groupID)+"/users", url.Values{"limit": {"200"}})
	if err != nil {
		return 0, err
	}
	return len(users), nil
}

func collectPolicies(ctx context.Context, client *Client) ([]FixturePolicy, error) {
	var out []FixturePolicy
	for _, policyType := range []string{"OKTA_SIGN_ON", "MFA_ENROLL", "ACCESS_POLICY"} {
		policies, err := getPagedArray[oktaPolicy](ctx, client, "/api/v1/policies", url.Values{"type": {policyType}, "limit": {"200"}})
		if err != nil {
			if apiStatus(err) == http.StatusBadRequest && policyType == "OKTA_SIGN_ON" {
				policies, err = getPagedArray[oktaPolicy](ctx, client, "/api/v1/policies", url.Values{"type": {"SIGN_ON"}, "limit": {"200"}})
			}
			if err != nil {
				switch apiStatus(err) {
				case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound:
					continue
				default:
					return nil, err
				}
			}
		}
		for _, policy := range policies {
			rules, err := collectPolicyRules(ctx, client, policy.ID)
			if err != nil {
				switch apiStatus(err) {
				case http.StatusForbidden, http.StatusNotFound:
					rules = nil
				default:
					return nil, err
				}
			}
			out = append(out, policyToFixture(policy, rules))
		}
	}
	return out, nil
}

func collectPolicyRules(ctx context.Context, client *Client, policyID string) ([]oktaPolicyRule, error) {
	if strings.TrimSpace(policyID) == "" {
		return nil, nil
	}
	return getPagedArray[oktaPolicyRule](ctx, client, "/api/v1/policies/"+url.PathEscape(policyID)+"/rules", url.Values{"limit": {"200"}})
}

func collectOrgFactors(ctx context.Context, client *Client) ([]string, error) {
	factors, err := getPagedArray[oktaFactor](ctx, client, "/api/v1/org/factors", nil)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, factor := range factors {
		if !strings.EqualFold(factor.Status, "ACTIVE") {
			continue
		}
		name := factor.FactorType
		if name == "" {
			name = factor.ID
		}
		if factor.Provider != "" {
			name = strings.ToLower(factor.Provider) + ":" + name
		}
		out = append(out, name)
	}
	return uniqueSortedStrings(out), nil
}

func collectBreakGlassAccount(ctx context.Context, client *Client, username string) (FixtureBreakGlassAccount, error) {
	var user oktaUser
	if _, _, err := client.get(ctx, "/api/v1/users/"+url.PathEscape(username), nil, &user); err != nil {
		return FixtureBreakGlassAccount{}, err
	}
	factors, err := getPagedArray[oktaFactor](ctx, client, "/api/v1/users/"+url.PathEscape(user.ID)+"/factors", nil)
	if err != nil {
		return FixtureBreakGlassAccount{}, err
	}
	login := user.Profile.Login
	if login == "" {
		login = user.Profile.Email
	}
	if login == "" {
		login = username
	}
	return FixtureBreakGlassAccount{
		Username:   login,
		MFAEnabled: hasActiveFactor(factors),
		ManagedBy:  "okta",
	}, nil
}

func appToFixture(app oktaApp, groups []oktaGroup) FixtureApplication {
	return FixtureApplication{
		ID:                       app.ID,
		Name:                     firstNonEmpty(app.Label, app.Name, app.ID),
		Protocol:                 appProtocol(app.SignOnMode),
		ClientID:                 firstNonEmpty(app.Credentials.OAuthClient.ClientID, stringFromMap(app.Settings.App, "client_id"), app.ID),
		Owners:                   uniqueSortedStrings(ownersFromMaps(app.Profile, app.Settings.App)),
		Groups:                   uniqueSortedStrings(groupIDs(groups)),
		RedirectURIs:             uniqueSortedStrings(appRedirectURIs(app)),
		GrantTypes:               uniqueSortedStrings(app.Settings.OAuthClient.GrantTypes),
		SAMLSigningCertPresent:   samlSigningCertPresent(app),
		SAMLSigningCertExpiresAt: strings.TrimSpace(app.Credentials.Signing.ExpiresAt),
	}
}

func groupToFixture(group oktaGroup, members int) FixtureGroup {
	return FixtureGroup{
		ID:      group.ID,
		Name:    firstNonEmpty(group.Profile.Name, group.ID),
		Owners:  uniqueSortedStrings(valuesFromAny(group.Profile.Owner, group.Profile.Owners)),
		Members: members,
	}
}

func policyToFixture(policy oktaPolicy, rules []oktaPolicyRule) FixturePolicy {
	var documents []any
	documents = append(documents, policy)
	for _, rule := range rules {
		documents = append(documents, rule)
	}
	return FixturePolicy{
		ID:           policy.ID,
		Name:         firstNonEmpty(policy.Name, policy.ID),
		Type:         strings.ToLower(firstNonEmpty(policy.Type, "policy")),
		Groups:       uniqueSortedStrings(stringsForNestedKey(documents, "groups")),
		Applications: uniqueSortedStrings(stringsForNestedKey(documents, "apps", "applications", "appInstance")),
		EnforcesMFA:  documentRequiresMFA(documents),
		Priority:     policy.Priority,
	}
}

func getPagedArray[T any](ctx context.Context, client *Client, path string, query url.Values) ([]T, error) {
	endpoint := client.baseURL + path
	if query != nil {
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	}
	var out []T
	for endpoint != "" {
		var batch []T
		if _, header, err := client.getURL(ctx, endpoint, &batch); err != nil {
			return nil, err
		} else {
			out = append(out, batch...)
			endpoint = nextLink(header.Get("Link"))
		}
	}
	return out, nil
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		section := strings.TrimSpace(part)
		if !strings.Contains(section, `rel="next"`) {
			continue
		}
		start := strings.Index(section, "<")
		end := strings.Index(section, ">")
		if start >= 0 && end > start {
			return section[start+1 : end]
		}
	}
	return ""
}

func appProtocol(signOnMode string) string {
	switch strings.ToUpper(strings.TrimSpace(signOnMode)) {
	case "OPENID_CONNECT":
		return "oidc"
	case "SAML_2_0":
		return "saml"
	default:
		if signOnMode == "" {
			return "unknown"
		}
		return strings.ToLower(signOnMode)
	}
}

func appRedirectURIs(app oktaApp) []string {
	out := append([]string{}, app.Settings.OAuthClient.RedirectURIs...)
	out = append(out, app.Settings.SignOn.SsoAcsURL, app.Settings.SignOn.Recipient, app.Settings.SignOn.Destination, app.Settings.SignOn.Audience)
	return out
}

func samlSigningCertPresent(app oktaApp) bool {
	if !strings.EqualFold(appProtocol(app.SignOnMode), "saml") {
		return false
	}
	return strings.TrimSpace(app.Credentials.Signing.Kid) != "" || strings.TrimSpace(app.Credentials.Signing.ExpiresAt) != ""
}

func groupIDs(groups []oktaGroup) []string {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.ID != "" {
			out = append(out, group.ID)
		}
	}
	return out
}

func policiesRequireMFA(policies []FixturePolicy) bool {
	for _, policy := range policies {
		if policy.EnforcesMFA {
			return true
		}
	}
	return false
}

func hasActiveFactor(factors []oktaFactor) bool {
	for _, factor := range factors {
		if strings.EqualFold(factor.Status, "ACTIVE") {
			return true
		}
	}
	return false
}

func documentRequiresMFA(documents []any) bool {
	data := strings.ToLower(string(prettyJSON(documents)))
	data = strings.NewReplacer(" ", "", "\n", "", "\t", "", "\r", "").Replace(data)
	return strings.Contains(data, `"requirefactor":true`) ||
		strings.Contains(data, `"factormode":"2fa"`) ||
		(strings.Contains(data, "mfa") && strings.Contains(data, "required"))
}

func stringsForNestedKey(documents []any, keys ...string) []string {
	keySet := map[string]struct{}{}
	for _, key := range keys {
		keySet[strings.ToLower(key)] = struct{}{}
	}
	var out []string
	for _, document := range documents {
		data, _ := json.Marshal(document)
		var decoded any
		if err := json.Unmarshal(data, &decoded); err == nil {
			out = append(out, walkNestedKeys(decoded, keySet, false)...)
		}
	}
	return out
}

func walkNestedKeys(value any, keySet map[string]struct{}, active bool) []string {
	var out []string
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			_, matched := keySet[strings.ToLower(key)]
			out = append(out, walkNestedKeys(child, keySet, active || matched)...)
		}
	case []any:
		for _, child := range typed {
			out = append(out, walkNestedKeys(child, keySet, active)...)
		}
	case string:
		if active && typed != "" {
			out = append(out, typed)
		}
	case float64:
		if active {
			out = append(out, strconv.FormatFloat(typed, 'f', -1, 64))
		}
	}
	return out
}

func ownersFromMaps(maps ...map[string]any) []string {
	var out []string
	for _, values := range maps {
		for _, key := range []string{"owner", "owners", "appOwner", "businessOwner", "technicalOwner", "adminEmail", "contactEmail"} {
			out = append(out, valuesFromAny(values[key])...)
		}
	}
	return out
}

func valuesFromAny(values ...any) []string {
	var out []string
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			out = append(out, typed)
		case []string:
			out = append(out, typed...)
		case []any:
			for _, item := range typed {
				out = append(out, valuesFromAny(item)...)
			}
		}
	}
	return out
}

func stringFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func splitOptionList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
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
