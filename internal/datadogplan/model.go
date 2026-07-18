package datadogplan

import "time"

const (
	APIVersion      = "openexit.dev/v1alpha1"
	InventoryKind   = "DatadogInventory"
	PlanKind        = "DatadogMigrationPlan"
	ValidationKind  = "DatadogValidation"
	BundleKind      = "MigrationBundle"
	CatalogVersion  = "datadog-observability/v1"
	RulesetVersion  = "datadog-grafana-lgtm/v1"
	DefaultTarget   = "grafana-lgtm"
	DefaultWorkDir  = ".openexit"
	InventoryRel    = "inventory/datadog.inventory.json"
	PlanRel         = "plan/openexit.plan.json"
	ValidationRel   = "validation/validation.json"
	ReportRel       = "index.html"
	BundleReadmeRel = "README.md"
)

type Inventory struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   InventoryMetadata `json:"metadata"`
	Catalog    Catalog           `json:"catalog"`
	Resources  []Resource        `json:"resources"`
}

type InventoryMetadata struct {
	Source           string    `json:"source"`
	Site             string    `json:"site"`
	CollectedAt      time.Time `json:"collectedAt"`
	CollectorVersion string    `json:"collectorVersion"`
	SnapshotDigest   string    `json:"snapshotDigest"`
}

type Catalog struct {
	Version  string          `json:"version"`
	Complete bool            `json:"complete"`
	Coverage []CatalogFamily `json:"coverage"`
}

type CatalogFamily struct {
	Family    string            `json:"family"`
	Status    string            `json:"status"`
	Count     int               `json:"count"`
	Endpoints []CatalogEndpoint `json:"endpoints"`
	Message   string            `json:"message,omitempty"`
}

type CatalogEndpoint struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Count   int    `json:"count"`
	Message string `json:"message,omitempty"`
}

const (
	CoverageComplete         = "complete"
	CoverageEmpty            = "empty"
	CoverageNotAvailable     = "not_available"
	CoveragePartial          = "partial"
	CoveragePermissionDenied = "permission_denied"
	CoverageError            = "error"
)

type Resource struct {
	Ref          string         `json:"ref"`
	Kind         string         `json:"kind"`
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	SourceURL    string         `json:"sourceUrl,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Evidence     Evidence       `json:"evidence"`
	Spec         map[string]any `json:"spec"`
}

type Evidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type MigrationPlan struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   PlanMetadata `json:"metadata"`
	Target     string       `json:"target"`
	Summary    PlanSummary  `json:"summary"`
	Readiness  Readiness    `json:"readiness"`
	Resources  []Conversion `json:"resources"`
}

type PlanMetadata struct {
	PlanID          string    `json:"planId"`
	InventoryDigest string    `json:"inventoryDigest"`
	GeneratedAt     time.Time `json:"generatedAt"`
	RulesetVersion  string    `json:"rulesetVersion"`
}

type PlanSummary struct {
	Total       int `json:"total"`
	Exact       int `json:"exact"`
	Approximate int `json:"approximate"`
	Manual      int `json:"manual"`
	Unsupported int `json:"unsupported"`
	OutputFiles int `json:"outputFiles"`
}

const (
	StatusExact       = "exact"
	StatusApproximate = "approximate"
	StatusManual      = "manual"
	StatusUnsupported = "unsupported"
)

type Conversion struct {
	SourceRef       string           `json:"sourceRef"`
	SourceKind      string           `json:"sourceKind"`
	SourceName      string           `json:"sourceName"`
	SourceURL       string           `json:"sourceUrl,omitempty"`
	EvidencePath    string           `json:"evidencePath"`
	Status          string           `json:"status"`
	ReasonCodes     []string         `json:"reasonCodes"`
	Summary         string           `json:"summary"`
	SemanticChanges []SemanticChange `json:"semanticChanges,omitempty"`
	Components      []Component      `json:"components,omitempty"`
	Outputs         []OutputRef      `json:"outputs"`
}

type Component struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Status      string   `json:"status"`
	ReasonCodes []string `json:"reasonCodes,omitempty"`
	SourceQuery string   `json:"sourceQuery,omitempty"`
	TargetQuery string   `json:"targetQuery,omitempty"`
	Review      string   `json:"review,omitempty"`
}

type SemanticChange struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
}

type OutputRef struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type Readiness struct {
	Score          int              `json:"score"`
	Level          string           `json:"level"`
	Formula        string           `json:"formula"`
	Collection     ReadinessFactor  `json:"collection"`
	Translation    ReadinessFactor  `json:"translation"`
	Validation     ReadinessFactor  `json:"validation"`
	Deductions     []ScoreDeduction `json:"deductions"`
	Interpretation string           `json:"interpretation"`
}

type ReadinessFactor struct {
	Value       float64 `json:"value"`
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Description string  `json:"description"`
}

type ScoreDeduction struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Points      int    `json:"points"`
}

type ValidationReport struct {
	APIVersion  string            `json:"apiVersion"`
	Kind        string            `json:"kind"`
	Status      string            `json:"status"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Checks      []ValidationCheck `json:"checks"`
}

type ValidationCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Critical bool   `json:"critical"`
}

type BundleManifest struct {
	APIVersion      string       `json:"apiVersion"`
	Kind            string       `json:"kind"`
	PlanID          string       `json:"planId"`
	InventoryDigest string       `json:"inventoryDigest"`
	Build           BuildInfo    `json:"build"`
	Files           []BundleFile `json:"files"`
}

type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type BundleFile struct {
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	SHA256     string   `json:"sha256"`
	SourceRefs []string `json:"sourceRefs,omitempty"`
}
