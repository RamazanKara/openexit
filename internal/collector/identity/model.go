package identity

type Fixture struct {
	Provider           string                     `json:"provider"`
	Tenant             string                     `json:"tenant"`
	Applications       []FixtureApplication       `json:"applications"`
	Groups             []FixtureGroup             `json:"groups"`
	Policies           []FixturePolicy            `json:"policies"`
	MFASettings        []FixtureMFASetting        `json:"mfaSettings"`
	BreakGlassAccounts []FixtureBreakGlassAccount `json:"breakGlassAccounts"`
}

type FixtureApplication struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Protocol                 string   `json:"protocol"`
	ClientID                 string   `json:"clientId"`
	Owners                   []string `json:"owners"`
	Groups                   []string `json:"groups"`
	RedirectURIs             []string `json:"redirectUris"`
	GrantTypes               []string `json:"grantTypes"`
	SAMLSigningCertPresent   bool     `json:"samlSigningCertPresent"`
	SAMLSigningCertExpiresAt string   `json:"samlSigningCertExpiresAt"`
}

type FixtureGroup struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Owners  []string `json:"owners"`
	Members int      `json:"members"`
}

type FixturePolicy struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Groups       []string `json:"groups"`
	Applications []string `json:"applications"`
	EnforcesMFA  bool     `json:"enforcesMfa"`
	Priority     int      `json:"priority"`
}

type FixtureMFASetting struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	Factors  []string `json:"factors"`
}

type FixtureBreakGlassAccount struct {
	Username   string `json:"username"`
	MFAEnabled bool   `json:"mfaEnabled"`
	ManagedBy  string `json:"managedBy"`
}
