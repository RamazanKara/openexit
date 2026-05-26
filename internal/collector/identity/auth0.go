package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type Auth0Collector struct{}

func (Auth0Collector) Name() string { return "auth0" }

func (Auth0Collector) Collect(ctx context.Context, req collector.CollectRequest) (*inventory.Inventory, error) {
	domain := strings.TrimSpace(req.Options["domain"])
	tokenEnv := option(req.Options, "token-env", "AUTH0_MANAGEMENT_TOKEN")
	token, err := envSecret(tokenEnv)
	if err != nil {
		return nil, err
	}
	client, err := NewAuth0Client(domain, token)
	if err != nil {
		return nil, err
	}

	var warnings []string
	fixture := Fixture{
		Provider: "auth0",
		Tenant:   strings.TrimPrefix(strings.TrimPrefix(client.baseURL, "https://"), "http://"),
	}

	clients, err := collectAuth0Clients(ctx, client)
	if err != nil {
		return nil, err
	}
	for _, app := range clients {
		fixture.Applications = append(fixture.Applications, auth0ClientToFixture(app))
	}

	roles, err := collectAuth0Roles(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect Auth0 roles", err))
	}
	for _, role := range roles {
		members, err := countAuth0RoleMembers(ctx, client, role.ID)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect Auth0 role members for "+role.ID, err))
		}
		fixture.Groups = append(fixture.Groups, auth0RoleToFixture(role, members))
	}

	policies, err := collectAuth0Policies(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect Auth0 actions and rules", err))
	}
	fixture.Policies = append(fixture.Policies, policies...)

	factors, err := collectAuth0GuardianFactors(ctx, client)
	if err != nil {
		warnings = append(warnings, optionalWarning("collect Auth0 Guardian factors", err))
	}
	if len(factors) > 0 || len(policies) > 0 {
		fixture.MFASettings = append(fixture.MFASettings, FixtureMFASetting{
			Name:     "auth0-guardian",
			Required: policiesRequireMFA(policies),
			Factors:  factors,
		})
	}

	for _, username := range splitOptionList(req.Options["break-glass-users"]) {
		account, err := collectAuth0BreakGlassAccount(ctx, client, username)
		if err != nil {
			warnings = append(warnings, optionalWarning("collect Auth0 break-glass user "+username, err))
			continue
		}
		fixture.BreakGlassAccounts = append(fixture.BreakGlassAccounts, account)
	}

	raw := inventory.RedactBytes(prettyJSON(fixture))
	inv := inventory.New(req.Project, "identity", client.baseURL, version.Version, time.Now().UTC())
	inv.Warnings = compactWarnings(warnings)
	if err := normalizeFixtureWithRawPath(req.ProjectDir, &fixture, inv, raw, "identity/raw-auth0-api-collection.json"); err != nil {
		return nil, err
	}
	return inv, writeInventory(req.ProjectDir, inv)
}

type auth0Client struct {
	ClientID       string         `json:"client_id"`
	Name           string         `json:"name"`
	AppType        string         `json:"app_type"`
	Callbacks      []string       `json:"callbacks"`
	AllowedOrigins []string       `json:"allowed_origins"`
	WebOrigins     []string       `json:"web_origins"`
	GrantTypes     []string       `json:"grant_types"`
	Addons         map[string]any `json:"addons"`
	ClientMetadata map[string]any `json:"client_metadata"`
	Owners         any            `json:"owners"`
}

type auth0Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type auth0Action struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Runtime            string `json:"runtime"`
	Status             string `json:"status"`
	AllChangesDeployed bool   `json:"all_changes_deployed"`
	SupportedTriggers  []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		Status  string `json:"status"`
	} `json:"supported_triggers"`
}

type auth0Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Order   int    `json:"order"`
	Stage   string `json:"stage"`
}

type auth0GuardianFactor struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type auth0User struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Multifactor []string `json:"multifactor"`
}

type auth0AuthenticationMethod struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func collectAuth0Clients(ctx context.Context, client *Client) ([]auth0Client, error) {
	query := url.Values{
		"fields":         {"client_id,name,app_type,callbacks,allowed_origins,web_origins,grant_types,addons,client_metadata,owners"},
		"include_fields": {"true"},
	}
	return getAuth0Collection[auth0Client](ctx, client, "/api/v2/clients", query, "clients")
}

func collectAuth0Roles(ctx context.Context, client *Client) ([]auth0Role, error) {
	return getAuth0Collection[auth0Role](ctx, client, "/api/v2/roles", nil, "roles")
}

func countAuth0RoleMembers(ctx context.Context, client *Client, roleID string) (int, error) {
	if strings.TrimSpace(roleID) == "" {
		return 0, nil
	}
	users, err := getAuth0Collection[auth0User](ctx, client, "/api/v2/roles/"+url.PathEscape(roleID)+"/users", nil, "users")
	if err != nil {
		return 0, err
	}
	return len(users), nil
}

func collectAuth0Policies(ctx context.Context, client *Client) ([]FixturePolicy, error) {
	var out []FixturePolicy
	actions, actionsErr := getAuth0Collection[auth0Action](ctx, client, "/api/v2/actions/actions", nil, "actions")
	if actionsErr == nil {
		for _, action := range actions {
			out = append(out, auth0ActionToPolicy(action))
		}
	}

	rules, rulesErr := getAuth0Collection[auth0Rule](ctx, client, "/api/v2/rules", url.Values{
		"fields":         {"id,name,enabled,order,stage"},
		"include_fields": {"true"},
	}, "rules")
	if rulesErr == nil {
		for _, rule := range rules {
			out = append(out, auth0RuleToPolicy(rule))
		}
	}

	switch {
	case actionsErr != nil && rulesErr != nil:
		return out, fmt.Errorf("%v; %v", actionsErr, rulesErr)
	case actionsErr != nil && apiStatus(actionsErr) != http.StatusForbidden && apiStatus(actionsErr) != http.StatusNotFound:
		return out, actionsErr
	case rulesErr != nil && apiStatus(rulesErr) != http.StatusForbidden && apiStatus(rulesErr) != http.StatusNotFound:
		return out, rulesErr
	default:
		return out, nil
	}
}

func collectAuth0GuardianFactors(ctx context.Context, client *Client) ([]string, error) {
	factors, err := getAuth0CollectionOnce[auth0GuardianFactor](ctx, client, "/api/v2/guardian/factors", nil, "factors")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, factor := range factors {
		if !factor.Enabled {
			continue
		}
		out = append(out, factor.Name)
	}
	return uniqueSortedStrings(out), nil
}

func collectAuth0BreakGlassAccount(ctx context.Context, client *Client, username string) (FixtureBreakGlassAccount, error) {
	user, err := findAuth0User(ctx, client, username)
	if err != nil {
		return FixtureBreakGlassAccount{}, err
	}
	methods, err := getAuth0CollectionOnce[auth0AuthenticationMethod](ctx, client, "/api/v2/users/"+url.PathEscape(user.UserID)+"/authentication-methods", nil, "authentication_methods")
	if err != nil && apiStatus(err) != http.StatusForbidden && apiStatus(err) != http.StatusNotFound {
		return FixtureBreakGlassAccount{}, err
	}
	login := firstNonEmpty(user.Email, user.Username, user.Name, user.UserID, username)
	return FixtureBreakGlassAccount{
		Username:   login,
		MFAEnabled: len(user.Multifactor) > 0 || len(methods) > 0,
		ManagedBy:  "auth0",
	}, nil
}

func findAuth0User(ctx context.Context, client *Client, username string) (auth0User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return auth0User{}, fmt.Errorf("username is required")
	}
	if strings.Contains(username, "|") {
		var user auth0User
		if _, _, err := client.get(ctx, "/api/v2/users/"+url.PathEscape(username), nil, &user); err != nil {
			return auth0User{}, err
		}
		return user, nil
	}
	query := url.Values{
		"q":             {fmt.Sprintf(`email:"%s" OR username:"%s"`, escapeAuth0QueryValue(username), escapeAuth0QueryValue(username))},
		"search_engine": {"v3"},
	}
	users, err := getAuth0Collection[auth0User](ctx, client, "/api/v2/users", query, "users")
	if err != nil {
		return auth0User{}, err
	}
	if len(users) == 0 {
		return auth0User{}, fmt.Errorf("user not found")
	}
	return users[0], nil
}

func auth0ClientToFixture(app auth0Client) FixtureApplication {
	return FixtureApplication{
		ID:           firstNonEmpty(app.ClientID, app.Name),
		Name:         firstNonEmpty(app.Name, app.ClientID),
		Protocol:     auth0ClientProtocol(app),
		ClientID:     app.ClientID,
		Owners:       uniqueSortedStrings(append(valuesFromAny(app.Owners), ownersFromMaps(app.ClientMetadata)...)),
		RedirectURIs: uniqueSortedStrings(auth0RedirectURIs(app)),
		GrantTypes:   uniqueSortedStrings(app.GrantTypes),
	}
}

func auth0RoleToFixture(role auth0Role, members int) FixtureGroup {
	return FixtureGroup{
		ID:      role.ID,
		Name:    firstNonEmpty(role.Name, role.ID),
		Members: members,
	}
}

func auth0ActionToPolicy(action auth0Action) FixturePolicy {
	triggers := auth0TriggerIDs(action.SupportedTriggers)
	policyType := "action"
	if len(triggers) > 0 {
		policyType = "action:" + strings.Join(triggers, ",")
	}
	return FixturePolicy{
		ID:          firstNonEmpty(action.ID, safeID(action.Name)),
		Name:        "Auth0 Action: " + firstNonEmpty(action.Name, action.ID),
		Type:        policyType,
		EnforcesMFA: auth0LooksLikeMFA(action.Name, strings.Join(triggers, " "), action.Status),
	}
}

func auth0RuleToPolicy(rule auth0Rule) FixturePolicy {
	policyType := "rule"
	if rule.Stage != "" {
		policyType = "rule:" + rule.Stage
	}
	return FixturePolicy{
		ID:          firstNonEmpty(rule.ID, safeID(rule.Name)),
		Name:        "Auth0 Rule: " + firstNonEmpty(rule.Name, rule.ID),
		Type:        policyType,
		EnforcesMFA: auth0LooksLikeMFA(rule.Name, rule.Stage),
		Priority:    rule.Order,
	}
}

func getAuth0Collection[T any](ctx context.Context, client *Client, path string, query url.Values, field string) ([]T, error) {
	const perPage = 100
	var out []T
	for page := 0; ; page++ {
		q := cloneValues(query)
		q.Set("page", fmt.Sprint(page))
		q.Set("per_page", fmt.Sprint(perPage))

		var raw json.RawMessage
		if _, _, err := client.get(ctx, path, q, &raw); err != nil {
			return nil, err
		}
		items, total, err := decodeAuth0Collection[T](raw, field)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if total > 0 {
			if len(out) >= total {
				break
			}
			continue
		}
		if len(items) < perPage {
			break
		}
	}
	return out, nil
}

func getAuth0CollectionOnce[T any](ctx context.Context, client *Client, path string, query url.Values, field string) ([]T, error) {
	var raw json.RawMessage
	if _, _, err := client.get(ctx, path, query, &raw); err != nil {
		return nil, err
	}
	items, _, err := decodeAuth0Collection[T](raw, field)
	return items, err
}

func decodeAuth0Collection[T any](raw json.RawMessage, field string) ([]T, int, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, 0, nil
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		var items []T
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, 0, err
		}
		return items, 0, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, 0, err
	}
	var total int
	_ = json.Unmarshal(envelope["total"], &total)
	payload := envelope[field]
	if len(payload) == 0 {
		payload = envelope["items"]
	}
	var items []T
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &items); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func cloneValues(input url.Values) url.Values {
	out := url.Values{}
	for key, values := range input {
		out[key] = append([]string{}, values...)
	}
	return out
}

func auth0ClientProtocol(app auth0Client) string {
	if _, ok := app.Addons["samlp"]; ok {
		return "saml"
	}
	if _, ok := app.Addons["wsfed"]; ok {
		return "wsfed"
	}
	switch strings.ToLower(strings.TrimSpace(app.AppType)) {
	case "non_interactive":
		return "m2m"
	default:
		return "oidc"
	}
}

func auth0RedirectURIs(app auth0Client) []string {
	out := append([]string{}, app.Callbacks...)
	out = append(out, app.AllowedOrigins...)
	out = append(out, app.WebOrigins...)
	return out
}

func auth0TriggerIDs(triggers []struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Status  string `json:"status"`
}) []string {
	var out []string
	for _, trigger := range triggers {
		if trigger.ID != "" {
			out = append(out, trigger.ID)
		}
	}
	return uniqueSortedStrings(out)
}

func auth0LooksLikeMFA(values ...string) bool {
	text := strings.ToLower(strings.Join(values, " "))
	return strings.Contains(text, "mfa") || strings.Contains(text, "multi-factor") || strings.Contains(text, "multifactor")
}

func escapeAuth0QueryValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
}
