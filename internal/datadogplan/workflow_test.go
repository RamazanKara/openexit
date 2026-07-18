package datadogplan

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFixtureWorkflowIsDeterministicAndReviewable(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	fixture := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	root := t.TempDir()
	workA := filepath.Join(root, "a", DefaultWorkDir)
	workB := filepath.Join(root, "b", DefaultWorkDir)

	for _, workDir := range []string{workA, workB} {
		inventory, err := Scan(context.Background(), ScanOptions{WorkDir: workDir, Fixture: fixture, Version: "test", Now: fixed})
		if err != nil {
			t.Fatalf("scan %s: %v", workDir, err)
		}
		if !inventory.Catalog.Complete || len(inventory.Resources) != 7 {
			t.Fatalf("unexpected inventory: complete=%t resources=%d", inventory.Catalog.Complete, len(inventory.Resources))
		}
		plan, validation, err := Plan(PlanOptions{WorkDir: workDir, Target: DefaultTarget})
		if err != nil {
			t.Fatalf("plan %s: %v", workDir, err)
		}
		if validation.Status != "passed" {
			t.Fatalf("validation status = %s", validation.Status)
		}
		if plan.Summary.Total != len(inventory.Resources) || len(plan.Resources) != len(inventory.Resources) {
			t.Fatalf("plan does not cover inventory: summary=%d conversions=%d inventory=%d", plan.Summary.Total, len(plan.Resources), len(inventory.Resources))
		}
	}

	assertTreeEqual(t, workA, workB)

	var inventory Inventory
	readTestJSON(t, filepath.Join(workA, filepath.FromSlash(InventoryRel)), &inventory)
	var plan MigrationPlan
	readTestJSON(t, filepath.Join(workA, filepath.FromSlash(PlanRel)), &plan)
	if plan.Readiness.Translation.Denominator != len(plan.Resources)*2 {
		t.Fatalf("translation denominator = %d, want %d", plan.Readiness.Translation.Denominator, len(plan.Resources)*2)
	}
	if plan.Readiness.Translation.Value != float64(plan.Readiness.Translation.Numerator)/float64(plan.Readiness.Translation.Denominator) {
		t.Fatalf("translation factor is not reproducible from numerator/denominator")
	}

	convertedOutputs := map[string][]string{}
	for _, conversion := range plan.Resources {
		if conversion.SourceRef == "" || conversion.EvidencePath == "" || len(conversion.ReasonCodes) == 0 {
			t.Fatalf("incomplete conversion ledger entry: %#v", conversion)
		}
		if _, err := os.Stat(filepath.Join(workA, filepath.FromSlash(conversion.EvidencePath))); err != nil {
			t.Fatalf("missing evidence for %s: %v", conversion.SourceRef, err)
		}
		for _, output := range conversion.Outputs {
			convertedOutputs[output.Path] = append(convertedOutputs[output.Path], conversion.SourceRef)
		}
		if conversion.SourceRef == "datadog:dashboard:abc-123" && len(conversion.Components) != 3 {
			t.Fatalf("dashboard components = %d, want every one of 3 source components", len(conversion.Components))
		}
		if conversion.SourceRef == "datadog:monitor:789012" && (conversion.Status != StatusManual || len(conversion.Outputs) != 0) {
			t.Fatalf("complex monitor must stay manual without executable output: %#v", conversion)
		}
	}

	err := filepath.WalkDir(filepath.Join(workA, "generated"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "vector(0)") {
			t.Fatalf("fake vector(0) placeholder in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	exportDir := filepath.Join(root, "migration")
	manifest, err := Export(ExportOptions{WorkDir: workA, Out: exportDir, Version: "0.1.0-test", Commit: "abc123", Date: "2026-07-18"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	manifestRefs := map[string][]string{}
	for _, file := range manifest.Files {
		manifestRefs[file.Path] = file.SourceRefs
	}
	for path, refs := range convertedOutputs {
		for _, ref := range SortedUnique(refs) {
			if !containsString(manifestRefs[path], ref) {
				t.Fatalf("manifest %s does not link source %s", path, ref)
			}
		}
	}
	for _, resource := range inventory.Resources {
		if !containsString(manifestRefs[resource.Evidence.Path], resource.Ref) {
			t.Fatalf("manifest evidence %s does not link source %s", resource.Evidence.Path, resource.Ref)
		}
	}
	verifyChecksumFile(t, exportDir)
	if err := validateHTMLLinks(exportDir, exportDir); err != nil {
		t.Fatalf("exported report links: %v", err)
	}
	if _, err := Export(ExportOptions{WorkDir: workA, Out: exportDir}); err == nil {
		t.Fatal("export unexpectedly replaced an existing directory without --force")
	}
	validationPath := filepath.Join(workA, filepath.FromSlash(ValidationRel))
	validationBytes, err := os.ReadFile(validationPath)
	if err != nil {
		t.Fatal(err)
	}
	var tamperedValidation ValidationReport
	if err := json.Unmarshal(validationBytes, &tamperedValidation); err != nil {
		t.Fatal(err)
	}
	tamperedValidation.Checks[0].Message = "tampered"
	if err := WriteJSON(validationPath, &tamperedValidation); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(ExportOptions{WorkDir: workA, Out: filepath.Join(root, "validation-tamper")}); err == nil || !strings.Contains(err.Error(), "saved validation report") {
		t.Fatalf("tampered validation report was not rejected: %v", err)
	}
	if err := os.WriteFile(validationPath, validationBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	rulePath := filepath.Join(workA, "generated", "prometheus", "rules", "123456-75784b75.yaml")
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulePath, append(rule, []byte("# tampered\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	tamperedOut := filepath.Join(root, "tampered-export")
	if _, err := Export(ExportOptions{WorkDir: workA, Out: tamperedOut}); err == nil || !strings.Contains(err.Error(), "deterministic-regeneration") {
		t.Fatalf("tampered generated file was not rejected by deterministic regeneration: %v", err)
	}
	if _, err := os.Stat(tamperedOut); !os.IsNotExist(err) {
		t.Fatalf("failed export left output behind: %v", err)
	}
}

func TestLiveScanUsesGETAndPaginatesMetrics(t *testing.T) {
	var mu sync.Mutex
	var methods []string
	metricRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mu.Lock()
		methods = append(methods, request.Method)
		mu.Unlock()
		if request.Header.Get("DD-API-KEY") != "test-api-key" || request.Header.Get("DD-APPLICATION-KEY") != "test-app-key" {
			t.Errorf("missing Datadog authentication headers")
		}
		body, _ := io.ReadAll(request.Body)
		if len(body) != 0 {
			t.Errorf("GET %s unexpectedly had a request body", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v2/metrics":
			mu.Lock()
			metricRequests++
			mu.Unlock()
			if request.URL.Query().Get("page[cursor]") == "next-page" {
				_, _ = io.WriteString(w, `{"data":[{"id":"metric.two","type":"metrics"}],"meta":{"pagination":{}}}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"metric.one","type":"metrics"}],"meta":{"pagination":{"next_cursor":"next-page"}}}`)
		case "/api/v1/logs/config/pipeline-order":
			_, _ = io.WriteString(w, `{"pipeline_ids":[]}`)
		default:
			_, _ = io.WriteString(w, `{"data":[],"dashboards":[],"dashboard_lists":[],"tests":[],"variables":[],"locations":[],"indexes":[]}`)
		}
	}))
	defer server.Close()
	t.Setenv("OPENEXIT_TEST_DD_API", "test-api-key")
	t.Setenv("OPENEXIT_TEST_DD_APP", "test-app-key")

	inventory, err := Scan(context.Background(), ScanOptions{
		WorkDir: filepath.Join(t.TempDir(), DefaultWorkDir), BaseURL: server.URL, HTTP: server.Client(),
		APIKeyEnv: "OPENEXIT_TEST_DD_API", AppKeyEnv: "OPENEXIT_TEST_DD_APP", Now: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("live scan: %v", err)
	}
	if !inventory.Catalog.Complete {
		t.Fatal("catalog should be complete when every endpoint responds")
	}
	metricCount := 0
	for _, resource := range inventory.Resources {
		if resource.Kind == "metric" {
			metricCount++
		}
	}
	if metricCount != 2 {
		t.Fatalf("metric count = %d, want 2 paginated resources", metricCount)
	}
	mu.Lock()
	defer mu.Unlock()
	if metricRequests != 2 {
		t.Fatalf("metric requests = %d, want 2", metricRequests)
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("Datadog scan used %s instead of GET", method)
		}
	}
}

func TestPermissionDeniedScanFailsClosedAndRemainsReviewable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v2/monitor/policy":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"errors":["secret response must not be persisted"]}`)
		case "/api/v1/logs/config/pipeline-order":
			_, _ = io.WriteString(w, `{"pipeline_ids":[]}`)
		default:
			_, _ = io.WriteString(w, `{"data":[],"dashboards":[],"dashboard_lists":[],"tests":[],"variables":[],"locations":[],"indexes":[],"meta":{"pagination":{}}}`)
		}
	}))
	defer server.Close()
	t.Setenv("OPENEXIT_DENIED_DD_API", "denied-api-key")
	t.Setenv("OPENEXIT_DENIED_DD_APP", "denied-app-key")
	workDir := filepath.Join(t.TempDir(), DefaultWorkDir)
	inventory, err := Scan(context.Background(), ScanOptions{
		WorkDir: workDir, BaseURL: server.URL, HTTP: server.Client(),
		APIKeyEnv: "OPENEXIT_DENIED_DD_API", AppKeyEnv: "OPENEXIT_DENIED_DD_APP", Now: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
	})
	if err == nil || inventory == nil {
		t.Fatalf("permission-denied scan should persist inventory and fail closed: inventory=%v err=%v", inventory, err)
	}
	if inventory.Catalog.Complete {
		t.Fatal("permission-denied catalog was marked complete")
	}
	var alerting *CatalogFamily
	for index := range inventory.Catalog.Coverage {
		if inventory.Catalog.Coverage[index].Family == "alerting" {
			alerting = &inventory.Catalog.Coverage[index]
		}
	}
	if alerting == nil || alerting.Status != CoveragePartial {
		t.Fatalf("alerting coverage = %#v, want partial", alerting)
	}
	policyDenied := false
	for _, endpoint := range alerting.Endpoints {
		if endpoint.Path == "/api/v2/monitor/policy" && endpoint.Status == CoveragePermissionDenied {
			policyDenied = true
		}
	}
	if !policyDenied {
		t.Fatalf("monitor-policy permission denial is not visible: %#v", alerting.Endpoints)
	}
	inventoryData, readErr := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(InventoryRel)))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, secret := range []string{"secret response", "denied-api-key", "denied-app-key"} {
		if strings.Contains(string(inventoryData), secret) {
			t.Fatalf("partial inventory exposed %q", secret)
		}
	}
	if _, _, err := Plan(PlanOptions{WorkDir: workDir}); err == nil {
		t.Fatal("plan unexpectedly accepted partial inventory without --allow-partial")
	}
	plan, validation, err := Plan(PlanOptions{WorkDir: workDir, AllowPartial: true})
	if err != nil {
		t.Fatalf("explicitly accepted partial plan: %v", err)
	}
	if validation.Status != "passed" || plan.Readiness.Collection.Numerator >= plan.Readiness.Collection.Denominator {
		t.Fatalf("partial limitation was not carried into plan: readiness=%#v validation=%#v", plan.Readiness, validation)
	}
}

func TestAPIErrorDoesNotExposeResponseOrCredentials(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Error(w, `{"errors":["test-api-key test-app-key secret-response"]}`, http.StatusForbidden)
	}))
	defer server.Close()
	client, err := newAPIClient("datadoghq.com", server.URL, "test-api-key", "test-app-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.get(context.Background(), "/api/v1/monitor")
	if err == nil {
		t.Fatal("expected API error")
	}
	for _, secret := range []string{"test-api-key", "test-app-key", "secret-response"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error exposed %q: %v", secret, err)
		}
	}
}

func TestAPIClientRefusesRedirectsBeforeCredentialsCanCrossHosts(t *testing.T) {
	t.Parallel()
	var redirectMu sync.Mutex
	redirectReached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		redirectMu.Lock()
		redirectReached = true
		redirectMu.Unlock()
		if request.Header.Get("DD-API-KEY") != "" || request.Header.Get("DD-APPLICATION-KEY") != "" {
			t.Error("Datadog credentials crossed a redirect boundary")
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, target.URL+"/capture", http.StatusFound)
	}))
	defer source.Close()
	client, err := newAPIClient("datadoghq.com", source.URL, "test-api-key", "test-app-key", source.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.get(context.Background(), "/api/v1/monitor")
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("redirect was not surfaced as a rejected API response: %v", err)
	}
	redirectMu.Lock()
	defer redirectMu.Unlock()
	if redirectReached {
		t.Fatal("redirect target was reached")
	}
}

func TestRedactionPreservesSecretResourceIdentity(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"id": "variable-123", "name": "deploy-token", "type": "secret", "is_secret": true,
		"value": "super-secret-value", "nested": map[string]any{"client_secret": "nested-secret"},
	}
	redacted := RedactValue(input).(map[string]any)
	if redacted["id"] != "variable-123" || redacted["name"] != "deploy-token" || redacted["type"] != "secret" || redacted["is_secret"] != true {
		t.Fatalf("redaction destroyed resource identity: %#v", redacted)
	}
	if redacted["value"] != "[REDACTED]" {
		t.Fatalf("secret value was not redacted: %#v", redacted["value"])
	}
	nested := redacted["nested"].(map[string]any)
	if nested["client_secret"] != "[REDACTED]" {
		t.Fatalf("nested secret was not redacted: %#v", nested)
	}
	textual := RedactValue(map[string]any{"tag": "api_key=embedded-value", "url": "https://example.invalid/?token=embedded-value"}).(map[string]any)
	if textual["tag"] != "[REDACTED]" || textual["url"] != "[REDACTED]" {
		t.Fatalf("textual secret assignments were not redacted: %#v", textual)
	}
}

func TestScoreUsesPublishedResourceWeights(t *testing.T) {
	t.Parallel()
	inventory := &Inventory{Catalog: Catalog{Coverage: []CatalogFamily{{Status: CoverageComplete}, {Status: CoverageEmpty}}}}
	conversions := []Conversion{
		{Status: StatusExact},
		{Status: StatusApproximate},
		{Status: StatusManual},
		{Status: StatusUnsupported},
	}
	validation := &ValidationReport{Checks: []ValidationCheck{{Status: "passed", Critical: true}}}
	readiness := Score(inventory, conversions, validation)
	if readiness.Translation.Numerator != 3 || readiness.Translation.Denominator != 8 || readiness.Translation.Value != 0.375 {
		t.Fatalf("unexpected translation factor: %#v", readiness.Translation)
	}
	if readiness.Score != 44 {
		t.Fatalf("score = %d, want round(100*(0.9*0.375+0.1)) = 44", readiness.Score)
	}
}

func TestExactOnlyScoreUsesSchemaCompatibleEmptyDeductions(t *testing.T) {
	t.Parallel()
	inventory := &Inventory{Catalog: Catalog{Coverage: []CatalogFamily{{Status: CoverageComplete}}}}
	readiness := Score(inventory, []Conversion{{Status: StatusExact}}, &ValidationReport{Checks: []ValidationCheck{{Status: "passed", Critical: true}}})
	if readiness.Deductions == nil || len(readiness.Deductions) != 0 {
		t.Fatalf("exact-only deductions = %#v, want a non-nil empty array", readiness.Deductions)
	}
}

func TestDashboardAndPrometheusCandidatesStayConservative(t *testing.T) {
	t.Parallel()
	stage := t.TempDir()
	resource := Resource{
		Ref: "datadog:dashboard:text-only", Kind: "dashboard", ID: "text-only", Name: "Text only",
		Evidence: Evidence{Path: "evidence/datadog/dashboard/text-only.json"},
		Spec: map[string]any{"widgets": []any{map[string]any{"definition": map[string]any{
			"type": "note", "title": "Runbook", "content": "Use the service runbook.",
		}}}},
	}
	conversion, err := generateDashboard(stage, resource)
	if err != nil {
		t.Fatal(err)
	}
	if conversion.Status != StatusApproximate || len(conversion.Components) != 1 || conversion.Components[0].Status != StatusExact {
		t.Fatalf("text dashboard overstated conversion fidelity: %#v", conversion)
	}
	if result := convertDashboardMetricQuery("avg:system.cpu.user{*}"); !result.OK || result.Expr != "avg(system_cpu_user)" {
		t.Fatalf("unfiltered Datadog selector was not converted conservatively: %#v", result)
	}
	for _, query := range []string{"avg:system.cpu.user{!env:prod}", "avg:system.cpu.user{env:(prod OR staging)}"} {
		if result := convertDashboardMetricQuery(query); result.OK {
			t.Fatalf("unsafe Datadog tag expression %q became executable PromQL: %#v", query, result)
		}
	}
	if alert := promAlertName("123 errors", "datadog:monitor:123"); !promIdentifierPattern.MatchString(alert) {
		t.Fatalf("generated invalid Prometheus alert name %q", alert)
	}
}

func TestSourceReferencesEscapeUnsafeResourceIDs(t *testing.T) {
	t.Parallel()
	ref := datadogSourceRef("dashboard", "folder/name\nwith space")
	if strings.ContainsAny(ref, "/\n ") || !strings.Contains(ref, "%2F") || !strings.Contains(ref, "%0A") {
		t.Fatalf("unsafe Datadog source reference %q", ref)
	}
}

func TestInstalledIntegrationFilteringFailsClosedWithoutProof(t *testing.T) {
	t.Parallel()
	spec := endpointSpec{InstalledOnly: true}
	items := []map[string]any{
		{"id": "installed", "attributes": map[string]any{"installed": true}},
		{"id": "available", "attributes": map[string]any{"installed": false}},
		{"id": "unknown", "attributes": map[string]any{}},
	}
	filtered, complete := filterEndpointItems(items, spec)
	if complete || len(filtered) != 1 || stringValue(filtered[0]["id"]) != "installed" {
		t.Fatalf("installed integration filter = %#v complete=%t", filtered, complete)
	}
}

func TestNewScanInvalidatesEveryStalePlanArtifact(t *testing.T) {
	t.Parallel()
	workDir := filepath.Join(t.TempDir(), DefaultWorkDir)
	fixture := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if _, err := Scan(context.Background(), ScanOptions{WorkDir: workDir, Fixture: fixture, Version: "test", Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Plan(PlanOptions{WorkDir: workDir}); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), ScanOptions{WorkDir: workDir, Fixture: fixture, Version: "test", Now: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"generated", "plan", "validation", ReportRel, BundleReadmeRel} {
		if _, err := os.Lstat(filepath.Join(workDir, filepath.FromSlash(stale))); !os.IsNotExist(err) {
			t.Fatalf("new scan left stale %s behind: %v", stale, err)
		}
	}
}

func TestExportTargetContainmentResolvesSymlinkedParents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workDir := filepath.Join(root, "state")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "state-alias")
	if err := os.Symlink(workDir, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := safeExportTarget(workDir, filepath.Join(alias, "migration")); err == nil {
		t.Fatal("export target inside a symlinked work directory was accepted")
	}
	if _, err := safeExportTarget(workDir, filepath.Join(root, "migration")); err != nil {
		t.Fatalf("safe sibling export target was rejected: %v", err)
	}
}

func assertTreeEqual(t *testing.T, left, right string) {
	t.Helper()
	leftFiles := readTree(t, left)
	rightFiles := readTree(t, right)
	if !reflect.DeepEqual(leftFiles, rightFiles) {
		t.Fatalf("deterministic workspaces differ\nleft: %#v\nright: %#v", leftFiles, rightFiles)
	}
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func readTestJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func verifyChecksumFile(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, BundleChecksumsRel))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid checksum line %q", line)
		}
		digest, _, err := DigestFile(filepath.Join(root, filepath.FromSlash(fields[1])))
		if err != nil {
			t.Fatal(err)
		}
		if digest != fields[0] {
			t.Fatalf("checksum mismatch for %s", fields[1])
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
