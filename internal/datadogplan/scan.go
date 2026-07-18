package datadogplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ScanOptions struct {
	WorkDir      string
	Site         string
	APIKeyEnv    string
	AppKeyEnv    string
	Fixture      string
	BaseURL      string
	HTTP         *http.Client
	Version      string
	Now          time.Time
	AllowPartial bool
}

type IncompleteScanError struct {
	Families []string
}

func (err *IncompleteScanError) Error() string {
	return "Datadog scan is incomplete: " + strings.Join(err.Families, ", ")
}

type endpointResult struct {
	Path      string
	Resources []Resource
	Status    string
	Message   string
}

func Scan(ctx context.Context, opts ScanOptions) (*Inventory, error) {
	applyScanDefaults(&opts)
	if err := os.MkdirAll(opts.WorkDir, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(opts.WorkDir, ".scan-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	var inv *Inventory
	if opts.Fixture != "" {
		inv, err = scanFixture(stage, opts)
	} else {
		inv, err = scanLive(ctx, stage, opts)
	}
	if err != nil {
		return nil, err
	}
	if err := validateInventoryStructure(inv); err != nil {
		return nil, fmt.Errorf("validate collected inventory: %w", err)
	}
	if err := finalizeInventory(stage, inv); err != nil {
		return nil, err
	}
	if err := validateInventoryDigest(inv); err != nil {
		return nil, fmt.Errorf("validate collected inventory digest: %w", err)
	}
	if err := validateSchemaFile("openexit.datadog-inventory.schema.json", filepath.Join(stage, filepath.FromSlash(InventoryRel))); err != nil {
		return nil, fmt.Errorf("validate collected inventory schema: %w", err)
	}
	if err := replaceScanState(opts.WorkDir, stage); err != nil {
		return nil, err
	}

	if !inv.Catalog.Complete && !opts.AllowPartial {
		return inv, &IncompleteScanError{Families: incompleteFamilies(inv.Catalog)}
	}
	return inv, nil
}

func applyScanDefaults(opts *ScanOptions) {
	if opts.WorkDir == "" {
		opts.WorkDir = DefaultWorkDir
	}
	if opts.Site == "" {
		opts.Site = "datadoghq.com"
	}
	if opts.APIKeyEnv == "" {
		opts.APIKeyEnv = "DATADOG_API_KEY"
	}
	if opts.AppKeyEnv == "" {
		opts.AppKeyEnv = "DATADOG_APP_KEY"
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	} else {
		opts.Now = opts.Now.UTC()
	}
}

func scanLive(ctx context.Context, stage string, opts ScanOptions) (*Inventory, error) {
	apiKey := os.Getenv(opts.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s is not set", opts.APIKeyEnv)
	}
	appKey := os.Getenv(opts.AppKeyEnv)
	if appKey == "" {
		return nil, fmt.Errorf("environment variable %s is not set", opts.AppKeyEnv)
	}
	client, err := newAPIClient(opts.Site, opts.BaseURL, apiKey, appKey, opts.HTTP)
	if err != nil {
		return nil, err
	}

	byFamily := map[string][]endpointResult{}
	for _, spec := range catalogEndpointSpecs {
		result := scanEndpoint(ctx, client, stage, opts.Site, spec)
		byFamily[spec.Family] = append(byFamily[spec.Family], result)
	}

	resources := make([]Resource, 0)
	coverage := make([]CatalogFamily, 0, len(catalogFamilies))
	for _, family := range catalogFamilies {
		results := byFamily[family]
		entry := aggregateFamily(family, results)
		for _, result := range results {
			resources = append(resources, result.Resources...)
		}
		coverage = append(coverage, entry)
	}
	resources = deduplicateResources(resources)
	return newInventory(opts, resources, coverage), nil
}

func scanEndpoint(ctx context.Context, client *apiClient, stage, site string, spec endpointSpec) endpointResult {
	items, status, message := fetchEndpointItems(ctx, client, spec)
	resources := make([]Resource, 0, len(items))
	for _, item := range items {
		var detailErr error
		item, detailErr = enrichEndpointItem(ctx, client, spec, item)
		if detailErr != nil {
			status = CoveragePartial
			message = appendMessage(message, detailErr.Error())
		}
		resource, err := makeResource(stage, site, spec.Kind, item)
		if err != nil {
			status = CoveragePartial
			message = appendMessage(message, err.Error())
			continue
		}
		resources = append(resources, resource)
	}
	if status == CoverageComplete && len(resources) == 0 {
		status = CoverageEmpty
	}
	return endpointResult{Path: spec.Path, Resources: resources, Status: status, Message: message}
}

func enrichEndpointItem(ctx context.Context, client *apiClient, spec endpointSpec, item map[string]any) (map[string]any, error) {
	id := resourceID(item, spec.Kind)
	if id == "" || (spec.DetailPath == "" && spec.RelatedPath == "") {
		return item, nil
	}
	out := cloneMap(item)
	if spec.DetailPath != "" {
		detail, err := fetchObject(ctx, client, fmt.Sprintf(spec.DetailPath, url.PathEscape(id)))
		if err != nil {
			return out, fmt.Errorf("%s %s detail: %w", spec.Kind, id, err)
		}
		for key, value := range detail {
			out[key] = value
		}
	}
	if spec.RelatedPath != "" {
		body, err := client.get(ctx, fmt.Sprintf(spec.RelatedPath, url.PathEscape(id)))
		if err != nil {
			return out, fmt.Errorf("%s %s related resources: %w", spec.Kind, id, err)
		}
		decoded, err := decodeJSON(body)
		if err != nil {
			return out, fmt.Errorf("%s %s related resources: %w", spec.Kind, id, err)
		}
		if spec.RelatedKey == "" {
			spec.RelatedKey = "related"
		}
		out["openexit_related"] = map[string]any{spec.RelatedKey: decoded}
	}
	return out, nil
}

func fetchObject(ctx context.Context, client *apiClient, endpoint string) (map[string]any, error) {
	body, err := client.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	decoded, err := decodeJSON(body)
	if err != nil {
		return nil, err
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response is not an object")
	}
	if data, ok := root["data"].(map[string]any); ok {
		return data, nil
	}
	return root, nil
}

func cloneMap(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, child := range value {
		out[key] = child
	}
	return out
}

func fetchEndpointItems(ctx context.Context, client *apiClient, spec endpointSpec) ([]map[string]any, string, string) {
	endpoint := spec.Path
	var items []map[string]any
	for page := 0; page < 1000; page++ {
		requestEndpoint := pagedEndpoint(endpoint, spec, page)
		body, err := client.get(ctx, requestEndpoint)
		if err != nil {
			return items, classifyAPIError(err, len(items) > 0), err.Error()
		}
		decoded, err := decodeJSON(body)
		if err != nil {
			return items, statusForPartial(len(items) > 0), "decode response: " + err.Error()
		}
		rawPageItems := extractItems(decoded, spec)
		pageItems, filterComplete := filterEndpointItems(rawPageItems, spec)
		items = append(items, pageItems...)
		if !filterComplete {
			return items, CoveragePartial, "installed-integration response omitted the installed field"
		}
		next := nextEndpoint(decoded, spec, endpoint, len(rawPageItems))
		if next == "" {
			return items, CoverageComplete, ""
		}
		endpoint = next
		if spec.Pagination != "links" && spec.Pagination != "cursor" {
			// Page and offset modes derive the next request from the original path.
			endpoint = spec.Path
		}
	}
	return items, CoveragePartial, "pagination exceeded 1000 pages"
}

func pagedEndpoint(endpoint string, spec endpointSpec, page int) string {
	mode := spec.Pagination
	if mode == "links" || mode == "" {
		return endpoint
	}
	pageSize := spec.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	size := strconv.Itoa(pageSize)
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	values := u.Query()
	switch mode {
	case "start":
		values.Set("start", strconv.Itoa(page*pageSize))
		values.Set("count", size)
	case "monitor":
		values.Set("page", strconv.Itoa(page))
		values.Set("page_size", size)
	case "offset":
		values.Set("offset", strconv.Itoa(page*pageSize))
		values.Set("limit", size)
	case "notebook":
		values.Set("start", strconv.Itoa(page*pageSize))
		values.Set("count", size)
	case "page-number":
		values.Set("page_number", strconv.Itoa(page))
		values.Set("page_size", size)
	case "page-offset":
		values.Set("page[offset]", strconv.Itoa(page*pageSize))
		values.Set("page[limit]", size)
	case "page-bracket-number":
		values.Set("page[number]", strconv.Itoa(page))
		values.Set("page[size]", size)
	case "cursor":
		values.Set("page[size]", size)
	default:
		return endpoint
	}
	u.RawQuery = values.Encode()
	return u.String()
}

func nextEndpoint(decoded any, spec endpointSpec, endpoint string, count int) string {
	if spec.Pagination == "links" {
		root, _ := decoded.(map[string]any)
		links, _ := root["links"].(map[string]any)
		if next, _ := links["next"].(string); next != "" {
			return next
		}
		meta, _ := root["meta"].(map[string]any)
		pageMeta, _ := meta["page"].(map[string]any)
		if after := stringValue(pageMeta["after"]); after != "" {
			u, _ := url.Parse(endpoint)
			query := u.Query()
			query.Set("page[cursor]", after)
			u.RawQuery = query.Encode()
			return u.String()
		}
		return ""
	}
	if spec.Pagination == "cursor" {
		root, _ := decoded.(map[string]any)
		cursor := stringValue(nestedValue(root, "meta", "pagination", "next_cursor"))
		if cursor == "" {
			return ""
		}
		u, err := url.Parse(spec.Path)
		if err != nil {
			return ""
		}
		query := u.Query()
		query.Set("page[size]", strconv.Itoa(spec.PageSize))
		query.Set("page[cursor]", cursor)
		u.RawQuery = query.Encode()
		return u.String()
	}
	if spec.Pagination != "" && count >= spec.PageSize {
		return spec.Path
	}
	return ""
}

func filterEndpointItems(items []map[string]any, spec endpointSpec) ([]map[string]any, bool) {
	if !spec.InstalledOnly {
		return items, true
	}
	out := make([]map[string]any, 0, len(items))
	complete := true
	for _, item := range items {
		installed, present := boolField(item, "installed")
		if !present {
			installed, present = boolField(item, "attributes", "installed")
		}
		if !present {
			complete = false
			continue
		}
		if installed {
			out = append(out, item)
		}
	}
	return out, complete
}

func boolField(root map[string]any, keys ...string) (bool, bool) {
	value := nestedValue(root, keys...)
	result, ok := value.(bool)
	return result, ok
}

func extractItems(decoded any, spec endpointSpec) []map[string]any {
	if list, ok := decoded.([]any); ok {
		return mapsFromList(list)
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range append(spec.ArrayKeys, "data") {
		if list, ok := root[key].([]any); ok {
			return mapsFromList(list)
		}
	}
	if spec.Singleton {
		return []map[string]any{root}
	}
	if _, hasID := root["id"]; hasID {
		return []map[string]any{root}
	}
	return nil
}

func mapsFromList(list []any) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if mapped, ok := item.(map[string]any); ok {
			out = append(out, mapped)
		}
	}
	return out
}

func scanFixture(stage string, opts ScanOptions) (*Inventory, error) {
	data, err := os.ReadFile(opts.Fixture)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	decoded, err := decodeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("fixture must be a JSON object")
	}
	if site, _ := root["site"].(string); site != "" {
		opts.Site = site
	}
	redacted, err := RedactJSON(data)
	if err != nil {
		return nil, err
	}
	if err := WriteText(filepath.Join(stage, "evidence", "datadog", "raw-fixture.json"), string(redacted)+"\n"); err != nil {
		return nil, err
	}

	definitions := []struct {
		Key    string
		Family string
		Kind   string
	}{
		{Key: "dashboards", Family: "dashboards", Kind: "dashboard"},
		{Key: "dashboard_lists", Family: "dashboards", Kind: "dashboard_list"},
		{Key: "powerpacks", Family: "dashboards", Kind: "powerpack"},
		{Key: "monitors", Family: "alerting", Kind: "monitor"},
		{Key: "monitor_policies", Family: "alerting", Kind: "monitor_policy"},
		{Key: "downtimes", Family: "alerting", Kind: "downtime"},
		{Key: "slos", Family: "slos", Kind: "slo"},
		{Key: "slo_corrections", Family: "slos", Kind: "slo_correction"},
		{Key: "notebooks", Family: "notebooks", Kind: "notebook"},
		{Key: "synthetic_tests", Family: "synthetics", Kind: "synthetic_test"},
		{Key: "synthetic_variables", Family: "synthetics", Kind: "synthetic_variable"},
		{Key: "synthetic_locations", Family: "synthetics", Kind: "synthetic_location"},
		{Key: "integrations", Family: "integrations", Kind: "integration"},
		{Key: "aws_integrations", Family: "integrations", Kind: "aws_integration"},
		{Key: "azure_integrations", Family: "integrations", Kind: "azure_integration"},
		{Key: "gcp_integrations", Family: "integrations", Kind: "gcp_integration"},
		{Key: "gcp_legacy_integrations", Family: "integrations", Kind: "gcp_legacy_integration"},
		{Key: "metrics", Family: "metrics", Kind: "metric"},
		{Key: "log_pipelines", Family: "logs", Kind: "log_pipeline"},
		{Key: "log_pipeline_orders", Family: "logs", Kind: "log_pipeline_order"},
		{Key: "log_indexes", Family: "logs", Kind: "log_index"},
		{Key: "log_archives", Family: "logs", Kind: "log_archive"},
		{Key: "log_metrics", Family: "logs", Kind: "log_metric"},
		{Key: "apm_retention_filters", Family: "apm", Kind: "apm_retention_filter"},
		{Key: "span_metrics", Family: "apm", Kind: "span_metric"},
		{Key: "service_definitions", Family: "services", Kind: "service_definition"},
	}
	counts := map[string]int{}
	var resources []Resource
	for _, definition := range definitions {
		list, _ := root[definition.Key].([]any)
		for _, raw := range list {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			resource, err := makeResource(stage, opts.Site, definition.Kind, item)
			if err != nil {
				return nil, err
			}
			resources = append(resources, resource)
			counts[definition.Family]++
		}
	}
	coverage := make([]CatalogFamily, 0, len(catalogFamilies))
	for _, family := range catalogFamilies {
		status := CoverageEmpty
		if counts[family] > 0 {
			status = CoverageComplete
		}
		coverage = append(coverage, CatalogFamily{
			Family: family, Status: status, Count: counts[family],
			Endpoints: []CatalogEndpoint{{Path: "fixture", Status: status, Count: counts[family]}},
		})
	}
	return newInventory(opts, deduplicateResources(resources), coverage), nil
}

func newInventory(opts ScanOptions, resources []Resource, coverage []CatalogFamily) *Inventory {
	sort.Slice(resources, func(i, j int) bool { return resources[i].Ref < resources[j].Ref })
	sort.Slice(coverage, func(i, j int) bool { return coverage[i].Family < coverage[j].Family })
	complete := true
	for _, entry := range coverage {
		if !coverageSatisfied(entry.Status) {
			complete = false
			break
		}
	}
	return &Inventory{
		APIVersion: APIVersion,
		Kind:       InventoryKind,
		Metadata: InventoryMetadata{
			Source:           "datadog",
			Site:             opts.Site,
			CollectedAt:      opts.Now,
			CollectorVersion: opts.Version,
		},
		Catalog:   Catalog{Version: CatalogVersion, Complete: complete, Coverage: coverage},
		Resources: resources,
	}
}

func finalizeInventory(stage string, inv *Inventory) error {
	digest, err := CanonicalDigest(struct {
		Catalog   Catalog    `json:"catalog"`
		Resources []Resource `json:"resources"`
	}{Catalog: inv.Catalog, Resources: inv.Resources})
	if err != nil {
		return err
	}
	inv.Metadata.SnapshotDigest = digest
	return WriteJSON(filepath.Join(stage, filepath.FromSlash(InventoryRel)), inv)
}

func replaceScanState(workDir, stage string) error {
	installMoves := make([]stateMove, 0, 2)
	stamp := time.Now().UTC().Format("20060102150405.000000000")
	for _, name := range []string{"inventory", "evidence"} {
		source := filepath.Join(stage, name)
		target := filepath.Join(workDir, name)
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("scan output missing %s: %w", name, err)
		}
		installMoves = append(installMoves, stateMove{source: source, target: target, backup: target + ".previous-" + stamp})
	}
	moves := append([]stateMove{}, installMoves...)
	for _, stale := range []string{"generated", "plan", "validation", ReportRel, BundleReadmeRel} {
		target := filepath.Join(workDir, filepath.FromSlash(stale))
		moves = append(moves, stateMove{target: target, backup: target + ".previous-" + stamp})
	}
	if err := backupStateMoves(moves); err != nil {
		return err
	}
	for index := range installMoves {
		if err := os.Rename(installMoves[index].source, installMoves[index].target); err != nil {
			for rollback := 0; rollback < index; rollback++ {
				_ = os.RemoveAll(installMoves[rollback].target)
			}
			restoreMoves(moves)
			return err
		}
	}
	for _, item := range moves {
		_ = os.RemoveAll(item.backup)
	}
	return nil
}

func makeResource(stage, site, kind string, raw map[string]any) (Resource, error) {
	redactedValue := RedactValue(raw)
	spec, ok := redactedValue.(map[string]any)
	if !ok {
		return Resource{}, fmt.Errorf("%s resource is not an object", kind)
	}
	id := resourceID(spec, kind)
	if id == "" {
		digest, err := CanonicalDigest(spec)
		if err != nil {
			return Resource{}, err
		}
		id = digest[:12]
	}
	name := resourceName(spec, kind, id)
	ref := datadogSourceRef(kind, id)
	evidenceRel := "evidence/datadog/" + kind + "/" + safeFilename(id, ref) + ".json"
	evidencePath := filepath.Join(stage, filepath.FromSlash(evidenceRel))
	if err := WriteJSON(evidencePath, spec); err != nil {
		return Resource{}, err
	}
	digest, _, err := DigestFile(evidencePath)
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		Ref:          ref,
		Kind:         kind,
		ID:           id,
		Name:         name,
		SourceURL:    resourceURL(site, kind, id),
		Tags:         resourceTags(spec),
		Dependencies: resourceDependencies(spec),
		Evidence:     Evidence{Path: evidenceRel, SHA256: digest},
		Spec:         spec,
	}, nil
}

func resourceID(item map[string]any, kind string) string {
	for _, key := range []string{"id", "public_id", "publicId", "monitor_id", "slo_id", "name"} {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	if attributes, ok := item["attributes"].(map[string]any); ok {
		for _, key := range []string{"id", "public_id", "name", "title"} {
			if value := stringValue(attributes[key]); value != "" {
				return value
			}
		}
	}
	switch kind {
	case "azure_integration":
		if value := joinedResourceID(item, "tenant_name", "client_id"); value != "" {
			return value
		}
	case "gcp_integration", "gcp_legacy_integration":
		if value := joinedResourceID(item, "project_id", "client_email"); value != "" {
			return value
		}
	case "aws_integration":
		if value := joinedResourceID(item, "aws_account_id", "account_id", "role_name"); value != "" {
			return value
		}
	}
	if kind == "log_pipeline_order" {
		return "pipeline-order"
	}
	return ""
}

func joinedResourceID(item map[string]any, keys ...string) string {
	var values []string
	for _, key := range keys {
		if value := stringValue(item[key]); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, ":")
}

func datadogSourceRef(kind, id string) string {
	return "datadog:" + kind + ":" + url.PathEscape(strings.TrimSpace(id))
}

func resourceName(item map[string]any, kind, fallback string) string {
	for _, key := range []string{"name", "title", "display_name", "public_id"} {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	if attributes, ok := item["attributes"].(map[string]any); ok {
		for _, key := range []string{"name", "title", "display_name"} {
			if value := stringValue(attributes[key]); value != "" {
				return value
			}
		}
	}
	return strings.ReplaceAll(kind, "_", " ") + " " + fallback
}

func resourceTags(item map[string]any) []string {
	for _, candidate := range []any{item["tags"], nestedValue(item, "attributes", "tags")} {
		if list, ok := candidate.([]any); ok {
			values := make([]string, 0, len(list))
			for _, value := range list {
				values = append(values, stringValue(value))
			}
			return SortedUnique(values)
		}
		if list, ok := candidate.([]string); ok {
			return SortedUnique(list)
		}
	}
	return nil
}

func resourceDependencies(item map[string]any) []string {
	var refs []string
	for _, value := range listValues(item["monitor_ids"]) {
		refs = append(refs, datadogSourceRef("monitor", value))
	}
	for _, value := range listValues(item["dashboardRefs"]) {
		refs = append(refs, datadogSourceRef("dashboard", value))
	}
	for _, value := range listValues(item["burnRateMonitorIds"]) {
		refs = append(refs, datadogSourceRef("monitor", value))
	}
	return SortedUnique(refs)
}

func resourceURL(site, kind, id string) string {
	if _, err := datadogAPIBaseURL(site); err != nil {
		return ""
	}
	base := "https://app." + site
	paths := map[string]string{
		"dashboard":      "/dashboard/",
		"monitor":        "/monitors/",
		"slo":            "/slo/",
		"notebook":       "/notebook/",
		"synthetic_test": "/synthetics/details/",
	}
	if prefix := paths[kind]; prefix != "" {
		return base + prefix + url.PathEscape(id)
	}
	return ""
}

func aggregateFamily(family string, results []endpointResult) CatalogFamily {
	entry := CatalogFamily{Family: family}
	var messages []string
	for _, result := range results {
		entry.Count += len(result.Resources)
		entry.Endpoints = append(entry.Endpoints, CatalogEndpoint{Path: result.Path, Status: result.Status, Count: len(result.Resources), Message: result.Message})
		if result.Message != "" {
			messages = append(messages, result.Message)
		}
	}
	entry.Message = strings.Join(SortedUnique(messages), "; ")
	entry.Status = catalogFamilyStatus(entry.Endpoints, entry.Count)
	return entry
}

func catalogFamilyStatus(endpoints []CatalogEndpoint, count int) string {
	allUnavailable := len(endpoints) > 0
	allDenied := len(endpoints) > 0
	hadFailure := false
	hadSuccess := false
	for _, endpoint := range endpoints {
		if endpoint.Status != CoverageNotAvailable {
			allUnavailable = false
		}
		if endpoint.Status != CoveragePermissionDenied {
			allDenied = false
		}
		if coverageSatisfied(endpoint.Status) {
			hadSuccess = true
		} else {
			hadFailure = true
		}
	}
	switch {
	case allUnavailable:
		return CoverageNotAvailable
	case allDenied:
		return CoveragePermissionDenied
	case hadFailure:
		if hadSuccess || count > 0 {
			return CoveragePartial
		} else {
			return CoverageError
		}
	case count == 0:
		return CoverageEmpty
	default:
		return CoverageComplete
	}
}

func deduplicateResources(resources []Resource) []Resource {
	byRef := map[string]Resource{}
	for _, resource := range resources {
		byRef[resource.Ref] = resource
	}
	out := make([]Resource, 0, len(byRef))
	for _, resource := range byRef {
		out = append(out, resource)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}

func classifyAPIError(err error, partial bool) string {
	if partial {
		return CoveragePartial
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusForbidden, http.StatusUnauthorized:
			return CoveragePermissionDenied
		case http.StatusNotFound:
			return CoverageNotAvailable
		}
	}
	return CoverageError
}

func statusForPartial(partial bool) string {
	if partial {
		return CoveragePartial
	}
	return CoverageError
}

func coverageSatisfied(status string) bool {
	return status == CoverageComplete || status == CoverageEmpty || status == CoverageNotAvailable
}

func incompleteFamilies(catalog Catalog) []string {
	var out []string
	for _, family := range catalog.Coverage {
		if !coverageSatisfied(family.Status) {
			out = append(out, family.Family+"="+family.Status)
		}
	}
	return out
}

func nestedValue(root map[string]any, keys ...string) any {
	var value any = root
	for _, key := range keys {
		mapped, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = mapped[key]
	}
	return value
}

func listValues(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text := stringValue(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func safeFilename(id, ref string) string {
	slug := slugify(id)
	digest := DigestBytes([]byte(ref))[:8]
	return slug + "-" + digest
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unnamed"
	}
	if len(result) > 80 {
		result = result[:80]
	}
	return result
}

func appendMessage(existing, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}
