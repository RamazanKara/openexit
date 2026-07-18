package datadogplan

type endpointSpec struct {
	Family        string
	Kind          string
	Path          string
	ArrayKeys     []string
	Pagination    string
	PageSize      int
	Singleton     bool
	InstalledOnly bool
	DetailPath    string
	RelatedPath   string
	RelatedKey    string
}

var catalogEndpointSpecs = []endpointSpec{
	{Family: "dashboards", Kind: "dashboard", Path: "/api/v1/dashboard", ArrayKeys: []string{"dashboards"}, Pagination: "start", PageSize: 100, DetailPath: "/api/v1/dashboard/%s"},
	{Family: "dashboards", Kind: "dashboard_list", Path: "/api/v1/dashboard/lists/manual", ArrayKeys: []string{"dashboard_lists"}, DetailPath: "/api/v1/dashboard/lists/manual/%s", RelatedPath: "/api/v1/dashboard/lists/manual/%s/dashboards", RelatedKey: "dashboards"},
	{Family: "dashboards", Kind: "powerpack", Path: "/api/v2/powerpacks", ArrayKeys: []string{"data"}, Pagination: "page-offset", PageSize: 25, DetailPath: "/api/v2/powerpacks/%s"},
	{Family: "alerting", Kind: "monitor", Path: "/api/v1/monitor", Pagination: "monitor", PageSize: 100},
	{Family: "alerting", Kind: "monitor_policy", Path: "/api/v2/monitor/policy", ArrayKeys: []string{"data"}},
	{Family: "alerting", Kind: "downtime", Path: "/api/v2/downtime", ArrayKeys: []string{"data"}, Pagination: "page-offset", PageSize: 30},
	{Family: "slos", Kind: "slo", Path: "/api/v1/slo", ArrayKeys: []string{"data"}, Pagination: "offset", PageSize: 1000},
	{Family: "slos", Kind: "slo_correction", Path: "/api/v1/slo/correction", ArrayKeys: []string{"data"}, Pagination: "offset", PageSize: 25},
	{Family: "notebooks", Kind: "notebook", Path: "/api/v1/notebooks", ArrayKeys: []string{"data"}, Pagination: "notebook", PageSize: 100},
	{Family: "synthetics", Kind: "synthetic_test", Path: "/api/v1/synthetics/tests", ArrayKeys: []string{"tests"}, Pagination: "page-number", PageSize: 100, DetailPath: "/api/v1/synthetics/tests/%s"},
	{Family: "synthetics", Kind: "synthetic_variable", Path: "/api/v1/synthetics/variables", ArrayKeys: []string{"variables"}},
	{Family: "synthetics", Kind: "synthetic_location", Path: "/api/v1/synthetics/locations", ArrayKeys: []string{"locations"}},
	{Family: "metrics", Kind: "metric", Path: "/api/v2/metrics", ArrayKeys: []string{"data"}, Pagination: "cursor", PageSize: 1000},
	{Family: "logs", Kind: "log_pipeline", Path: "/api/v1/logs/config/pipelines"},
	{Family: "logs", Kind: "log_pipeline_order", Path: "/api/v1/logs/config/pipeline-order", Singleton: true},
	{Family: "logs", Kind: "log_index", Path: "/api/v1/logs/config/indexes", ArrayKeys: []string{"indexes"}},
	{Family: "logs", Kind: "log_archive", Path: "/api/v2/logs/config/archives", ArrayKeys: []string{"data"}},
	{Family: "logs", Kind: "log_metric", Path: "/api/v2/logs/config/metrics", ArrayKeys: []string{"data"}},
	{Family: "apm", Kind: "apm_retention_filter", Path: "/api/v2/apm/config/retention-filters", ArrayKeys: []string{"data"}},
	{Family: "apm", Kind: "span_metric", Path: "/api/v2/apm/config/metrics", ArrayKeys: []string{"data"}},
	{Family: "services", Kind: "service_definition", Path: "/api/v2/services/definitions", ArrayKeys: []string{"data"}, Pagination: "page-bracket-number", PageSize: 100},
	{Family: "integrations", Kind: "integration", Path: "/api/v2/integrations", ArrayKeys: []string{"data"}, InstalledOnly: true},
	{Family: "integrations", Kind: "aws_integration", Path: "/api/v2/integration/aws/accounts", ArrayKeys: []string{"data"}},
	{Family: "integrations", Kind: "azure_integration", Path: "/api/v1/integration/azure"},
	{Family: "integrations", Kind: "gcp_integration", Path: "/api/v2/integration/gcp/accounts", ArrayKeys: []string{"data"}},
	{Family: "integrations", Kind: "gcp_legacy_integration", Path: "/api/v1/integration/gcp"},
}

var catalogFamilies = []string{
	"dashboards",
	"alerting",
	"slos",
	"notebooks",
	"synthetics",
	"metrics",
	"logs",
	"apm",
	"services",
	"integrations",
}
