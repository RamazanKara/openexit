package edge

type Fixture struct {
	Provider    string                `json:"provider"`
	Zone        string                `json:"zone"`
	DNSRecords  []FixtureDNSRecord    `json:"dnsRecords"`
	WAFRules    []FixtureWAFRule      `json:"wafRules"`
	CacheRules  []FixtureCacheRule    `json:"cacheRules"`
	Redirects   []FixtureRedirectRule `json:"redirects"`
	Origins     []FixtureOriginConfig `json:"origins"`
	TLSSettings []FixtureTLSSetting   `json:"tlsSettings"`
	BotRules    []FixtureBotRule      `json:"botRules"`
	PageRules   []FixturePageRule     `json:"pageRules"`
}

type FixtureDNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type FixtureWAFRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Enabled    bool   `json:"enabled"`
	Managed    bool   `json:"managed"`
}

type FixtureCacheRule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Pattern    string   `json:"pattern"`
	Action     string   `json:"action"`
	EdgeTTL    int      `json:"edgeTtl"`
	BrowserTTL int      `json:"browserTtl"`
	CacheKey   []string `json:"cacheKey"`
}

type FixtureRedirectRule struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Source        string `json:"source"`
	Target        string `json:"target"`
	StatusCode    int    `json:"statusCode"`
	PreserveQuery bool   `json:"preserveQuery"`
}

type FixtureOriginConfig struct {
	ID          string `json:"id"`
	Hostname    string `json:"hostname"`
	Scheme      string `json:"scheme"`
	Port        int    `json:"port"`
	HostHeader  string `json:"hostHeader"`
	TLSVerify   bool   `json:"tlsVerify"`
	HealthCheck bool   `json:"healthCheck"`
}

type FixtureTLSSetting struct {
	ID          string `json:"id"`
	Hostname    string `json:"hostname"`
	Mode        string `json:"mode"`
	MinVersion  string `json:"minVersion"`
	HSTSEnabled bool   `json:"hstsEnabled"`
}

type FixtureBotRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Enabled    bool   `json:"enabled"`
}

type FixturePageRule struct {
	ID       string   `json:"id"`
	Target   string   `json:"target"`
	Actions  []string `json:"actions"`
	Priority int      `json:"priority"`
	Enabled  bool     `json:"enabled"`
}
