package datadogplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func validateWorkspace(stage, workDir string, inv *Inventory, plan *MigrationPlan, allowPartial bool) *ValidationReport {
	report := &ValidationReport{APIVersion: APIVersion, Kind: ValidationKind, Status: "passed", GeneratedAt: inv.Metadata.CollectedAt, Checks: []ValidationCheck{}}
	add := func(name, status, message string, critical bool) {
		report.Checks = append(report.Checks, ValidationCheck{Name: name, Status: status, Message: message, Critical: critical})
		if status == "failed" && critical {
			report.Status = "failed"
		}
	}

	if err := validateInventoryStructure(inv); err != nil {
		add("inventory-structure", "failed", err.Error(), true)
	} else {
		add("inventory-structure", "passed", "", true)
	}
	if err := validateInventoryDigest(inv); err != nil {
		add("inventory-digest", "failed", err.Error(), true)
	} else {
		add("inventory-digest", "passed", "", true)
	}
	if inv.Catalog.Complete {
		add("inventory-completeness", "passed", "", true)
	} else if allowPartial {
		add("inventory-completeness", "warning", "partial inventory explicitly accepted: "+strings.Join(incompleteFamilies(inv.Catalog), ", "), false)
	} else {
		add("inventory-completeness", "failed", strings.Join(incompleteFamilies(inv.Catalog), ", "), true)
	}
	if err := validatePlanIdentity(inv, plan); err != nil {
		add("plan-identity", "failed", err.Error(), true)
	} else {
		add("plan-identity", "passed", "", true)
	}
	if err := validateConversionCoverage(inv, plan); err != nil {
		add("conversion-coverage", "failed", err.Error(), true)
	} else {
		add("conversion-coverage", "passed", "", true)
	}
	if err := validateDeterministicRegeneration(stage, inv, plan); err != nil {
		add("deterministic-regeneration", "failed", err.Error(), true)
	} else {
		add("deterministic-regeneration", "passed", "", true)
	}
	if err := validateEvidence(workDir, inv); err != nil {
		add("evidence-integrity", "failed", err.Error(), true)
	} else {
		add("evidence-integrity", "passed", "", true)
	}
	if err := validateOutputProvenance(stage, plan); err != nil {
		add("output-provenance", "failed", err.Error(), true)
	} else {
		add("output-provenance", "passed", "", true)
	}
	if err := validateGrafanaOutputs(stage, plan); err != nil {
		add("grafana-candidates", "failed", err.Error(), true)
	} else {
		add("grafana-candidates", "passed", "", true)
	}
	if err := validatePrometheusOutputs(stage, plan); err != nil {
		add("prometheus-candidates", "failed", err.Error(), true)
	} else {
		add("prometheus-candidates", "passed", "", true)
	}
	if err := validateTelemetryOutputs(stage, inv); err != nil {
		add("telemetry-candidates", "failed", err.Error(), true)
	} else {
		add("telemetry-candidates", "passed", "", true)
	}
	if err := EnsureNoSymlinks(stage); err != nil {
		add("workspace-path-safety", "failed", err.Error(), true)
	} else {
		add("workspace-path-safety", "passed", "", true)
	}
	if err := validateNoSecrets(stage, filepath.Join(workDir, "evidence")); err != nil {
		add("secret-scan", "failed", err.Error(), true)
	} else {
		add("secret-scan", "passed", "", true)
	}
	if err := validateSchemaFile("openexit.datadog-inventory.schema.json", filepath.Join(workDir, filepath.FromSlash(InventoryRel))); err != nil {
		add("jsonschema-inventory", "failed", err.Error(), true)
	} else {
		add("jsonschema-inventory", "passed", "", true)
	}
	if err := validateSchemaFile("openexit.datadog-plan.schema.json", filepath.Join(stage, filepath.FromSlash(PlanRel))); err != nil {
		add("jsonschema-plan", "failed", err.Error(), true)
	} else {
		add("jsonschema-plan", "passed", "", true)
	}
	if _, err := os.Stat(filepath.Join(stage, ReportRel)); err == nil {
		if err := validateHTMLLinks(stage, workDir); err != nil {
			add("report-links", "failed", err.Error(), true)
		} else {
			add("report-links", "passed", "", true)
		}
	}

	sort.Slice(report.Checks, func(i, j int) bool { return report.Checks[i].Name < report.Checks[j].Name })
	return report
}

func validateInventoryStructure(inv *Inventory) error {
	if inv.APIVersion != APIVersion || inv.Kind != InventoryKind || inv.Metadata.Source != "datadog" || inv.Catalog.Version != CatalogVersion {
		return fmt.Errorf("unexpected inventory identity or catalog version")
	}
	expectedEndpoints := map[string][]string{}
	kindFamily := map[string]string{}
	for _, spec := range catalogEndpointSpecs {
		expectedEndpoints[spec.Family] = append(expectedEndpoints[spec.Family], spec.Path)
		kindFamily[spec.Kind] = spec.Family
	}
	families := map[string]CatalogFamily{}
	fixtureMode := false
	for _, family := range inv.Catalog.Coverage {
		if _, exists := families[family.Family]; exists {
			return fmt.Errorf("duplicate catalog family %s", family.Family)
		}
		families[family.Family] = family
		for _, endpoint := range family.Endpoints {
			if endpoint.Path == "fixture" {
				fixtureMode = true
			}
		}
	}
	if len(families) != len(catalogFamilies) {
		return fmt.Errorf("catalog has %d families; expected %d", len(families), len(catalogFamilies))
	}
	complete := true
	for _, name := range catalogFamilies {
		family, ok := families[name]
		if !ok {
			return fmt.Errorf("catalog family is missing: %s", name)
		}
		if len(family.Endpoints) == 0 {
			return fmt.Errorf("catalog family %s has no endpoint coverage", name)
		}
		if fixtureMode {
			if len(family.Endpoints) != 1 || family.Endpoints[0].Path != "fixture" {
				return fmt.Errorf("fixture catalog family %s has unexpected endpoint coverage", name)
			}
		} else {
			want := expectedEndpoints[name]
			if len(family.Endpoints) != len(want) {
				return fmt.Errorf("catalog family %s covers %d of %d endpoints", name, len(family.Endpoints), len(want))
			}
			for index, path := range want {
				if family.Endpoints[index].Path != path {
					return fmt.Errorf("catalog family %s endpoint %d is %s; expected %s", name, index, family.Endpoints[index].Path, path)
				}
			}
		}
		endpointCount := 0
		for _, endpoint := range family.Endpoints {
			if !validCoverageStatus(endpoint.Status) {
				return fmt.Errorf("catalog endpoint %s has invalid status %q", endpoint.Path, endpoint.Status)
			}
			if endpoint.Count < 0 {
				return fmt.Errorf("catalog endpoint %s has a negative count", endpoint.Path)
			}
			if endpointCoverageCountInconsistent(endpoint.Status, endpoint.Count) {
				return fmt.Errorf("catalog endpoint %s status %s is inconsistent with count %d", endpoint.Path, endpoint.Status, endpoint.Count)
			}
			endpointCount += endpoint.Count
		}
		if endpointCount != family.Count {
			return fmt.Errorf("catalog family %s count is %d; endpoint counts total %d", name, family.Count, endpointCount)
		}
		if want := catalogFamilyStatus(family.Endpoints, family.Count); family.Status != want {
			return fmt.Errorf("catalog family %s status is %s; endpoint coverage requires %s", name, family.Status, want)
		}
		if endpointCoverageCountInconsistent(family.Status, family.Count) {
			return fmt.Errorf("catalog family %s status %s is inconsistent with count %d", name, family.Status, family.Count)
		}
		if !coverageSatisfied(family.Status) {
			complete = false
		}
	}
	if inv.Catalog.Complete != complete {
		return fmt.Errorf("catalog complete is %t; coverage requires %t", inv.Catalog.Complete, complete)
	}

	resourceCounts := map[string]int{}
	refs := map[string]struct{}{}
	evidencePaths := map[string]struct{}{}
	previousRef := ""
	for _, resource := range inv.Resources {
		family, ok := kindFamily[resource.Kind]
		if !ok {
			return fmt.Errorf("resource %s has unknown kind %s", resource.Ref, resource.Kind)
		}
		if resource.Ref != datadogSourceRef(resource.Kind, resource.ID) {
			return fmt.Errorf("resource %s does not match kind and ID", resource.Ref)
		}
		if _, exists := refs[resource.Ref]; exists {
			return fmt.Errorf("duplicate inventory resource %s", resource.Ref)
		}
		refs[resource.Ref] = struct{}{}
		if previousRef != "" && resource.Ref < previousRef {
			return fmt.Errorf("inventory resources are not sorted by source reference")
		}
		previousRef = resource.Ref
		wantEvidence := "evidence/datadog/" + resource.Kind + "/" + safeFilename(resource.ID, resource.Ref) + ".json"
		if resource.Evidence.Path != wantEvidence {
			return fmt.Errorf("resource %s has unexpected evidence path %s", resource.Ref, resource.Evidence.Path)
		}
		if _, exists := evidencePaths[resource.Evidence.Path]; exists {
			return fmt.Errorf("duplicate evidence path %s", resource.Evidence.Path)
		}
		evidencePaths[resource.Evidence.Path] = struct{}{}
		resourceCounts[family]++
	}
	for _, family := range inv.Catalog.Coverage {
		if resourceCounts[family.Family] != family.Count {
			return fmt.Errorf("catalog family %s reports %d resources; inventory contains %d", family.Family, family.Count, resourceCounts[family.Family])
		}
	}
	return nil
}

func validCoverageStatus(status string) bool {
	switch status {
	case CoverageComplete, CoverageEmpty, CoverageNotAvailable, CoveragePartial, CoveragePermissionDenied, CoverageError:
		return true
	default:
		return false
	}
}

func endpointCoverageCountInconsistent(status string, count int) bool {
	switch status {
	case CoverageComplete:
		return count == 0
	case CoverageEmpty, CoverageNotAvailable, CoveragePermissionDenied, CoverageError:
		return count != 0
	default:
		return false
	}
}

func validateDeterministicRegeneration(stage string, inv *Inventory, plan *MigrationPlan) error {
	temp, err := os.MkdirTemp("", "openexit-reproduction-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()
	expected, err := generateConversions(temp, inv)
	if err != nil {
		return err
	}
	wantDigest, err := CanonicalDigest(expected)
	if err != nil {
		return err
	}
	gotDigest, err := CanonicalDigest(plan.Resources)
	if err != nil {
		return err
	}
	if wantDigest != gotDigest {
		return fmt.Errorf("conversion ledger does not match deterministic ruleset %s", RulesetVersion)
	}
	wantSummary, err := CanonicalDigest(summarizeConversions(expected))
	if err != nil {
		return err
	}
	gotSummary, err := CanonicalDigest(plan.Summary)
	if err != nil {
		return err
	}
	if wantSummary != gotSummary {
		return fmt.Errorf("plan summary does not match deterministic conversions")
	}
	wantFiles, err := generatedDigests(filepath.Join(temp, "generated"))
	if err != nil {
		return err
	}
	gotFiles, err := generatedDigests(filepath.Join(stage, "generated"))
	if err != nil {
		return err
	}
	if len(wantFiles) != len(gotFiles) {
		return fmt.Errorf("generated file set has %d files; deterministic ruleset produced %d", len(gotFiles), len(wantFiles))
	}
	for path, digest := range wantFiles {
		if gotFiles[path] != digest {
			return fmt.Errorf("generated file differs from deterministic ruleset: %s", path)
		}
	}
	return nil
}

func generatedDigests(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest, _, err := DigestFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = digest
		return nil
	})
	return files, err
}

func validateInventoryDigest(inv *Inventory) error {
	digest, err := CanonicalDigest(struct {
		Catalog   Catalog    `json:"catalog"`
		Resources []Resource `json:"resources"`
	}{Catalog: inv.Catalog, Resources: inv.Resources})
	if err != nil {
		return err
	}
	if digest != inv.Metadata.SnapshotDigest {
		return fmt.Errorf("inventory digest mismatch: expected %s, got %s", inv.Metadata.SnapshotDigest, digest)
	}
	return nil
}

func validatePlanIdentity(inv *Inventory, plan *MigrationPlan) error {
	if plan.APIVersion != APIVersion || plan.Kind != PlanKind {
		return fmt.Errorf("unexpected plan identity")
	}
	if plan.Target != DefaultTarget {
		return fmt.Errorf("unsupported target %q", plan.Target)
	}
	if plan.Metadata.InventoryDigest != inv.Metadata.SnapshotDigest {
		return fmt.Errorf("plan inventory digest does not match the current scan")
	}
	if plan.Metadata.RulesetVersion != RulesetVersion {
		return fmt.Errorf("plan ruleset is %q; expected %q", plan.Metadata.RulesetVersion, RulesetVersion)
	}
	if !plan.Metadata.GeneratedAt.Equal(inv.Metadata.CollectedAt) {
		return fmt.Errorf("plan timestamp does not match the inventory snapshot")
	}
	want, err := planDigest(inv.Metadata.SnapshotDigest, plan.Target)
	if err != nil {
		return err
	}
	if want != plan.Metadata.PlanID {
		return fmt.Errorf("plan ID does not match its deterministic inputs")
	}
	return nil
}

func validateConversionCoverage(inv *Inventory, plan *MigrationPlan) error {
	resources := map[string]Resource{}
	for _, resource := range inv.Resources {
		if _, exists := resources[resource.Ref]; exists {
			return fmt.Errorf("duplicate inventory resource %s", resource.Ref)
		}
		resources[resource.Ref] = resource
	}
	seen := map[string]struct{}{}
	for _, conversion := range plan.Resources {
		if _, ok := resources[conversion.SourceRef]; !ok {
			return fmt.Errorf("conversion references unknown source %s", conversion.SourceRef)
		}
		if _, exists := seen[conversion.SourceRef]; exists {
			return fmt.Errorf("duplicate conversion for %s", conversion.SourceRef)
		}
		seen[conversion.SourceRef] = struct{}{}
		if conversion.Status != StatusExact && conversion.Status != StatusApproximate && conversion.Status != StatusManual && conversion.Status != StatusUnsupported {
			return fmt.Errorf("%s has invalid status %q", conversion.SourceRef, conversion.Status)
		}
		if len(conversion.ReasonCodes) == 0 {
			return fmt.Errorf("%s has no conversion reason code", conversion.SourceRef)
		}
	}
	if len(seen) != len(resources) {
		return fmt.Errorf("plan covers %d of %d inventory resources", len(seen), len(resources))
	}
	return nil
}

func validateEvidence(workDir string, inv *Inventory) error {
	for _, resource := range inv.Resources {
		path, err := WorkspacePath(workDir, resource.Evidence.Path)
		if err != nil {
			return fmt.Errorf("%s evidence path: %w", resource.Ref, err)
		}
		digest, _, err := DigestFile(path)
		if err != nil {
			return fmt.Errorf("%s evidence: %w", resource.Ref, err)
		}
		if digest != resource.Evidence.SHA256 {
			return fmt.Errorf("%s evidence digest mismatch", resource.Ref)
		}
		var evidence map[string]any
		if err := ReadJSON(path, &evidence); err != nil {
			return fmt.Errorf("%s evidence content: %w", resource.Ref, err)
		}
		want, err := CanonicalDigest(resource.Spec)
		if err != nil {
			return err
		}
		got, err := CanonicalDigest(evidence)
		if err != nil {
			return err
		}
		if want != got {
			return fmt.Errorf("%s inventory spec does not match its evidence", resource.Ref)
		}
	}
	return nil
}

func validateOutputProvenance(stage string, plan *MigrationPlan) error {
	referenced := map[string][]string{}
	for _, conversion := range plan.Resources {
		for _, output := range conversion.Outputs {
			if err := SafeRelativePath(output.Path); err != nil {
				return fmt.Errorf("%s output %s: %w", conversion.SourceRef, output.Path, err)
			}
			path := filepath.Join(stage, filepath.FromSlash(output.Path))
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return fmt.Errorf("%s output is missing: %s", conversion.SourceRef, output.Path)
			}
			referenced[output.Path] = append(referenced[output.Path], conversion.SourceRef)
		}
	}
	allowedBaseline := map[string]bool{
		"generated/alloy/config.alloy":           true,
		"generated/opentelemetry/collector.yaml": true,
	}
	root := filepath.Join(stage, "generated")
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(referenced[rel]) == 0 && !allowedBaseline[rel] {
			return fmt.Errorf("generated output is not linked from the plan: %s", rel)
		}
		return nil
	})
}

func validateGrafanaOutputs(stage string, plan *MigrationPlan) error {
	for _, conversion := range plan.Resources {
		for _, output := range conversion.Outputs {
			if output.Kind != "grafana-dashboard" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(output.Path)))
			if err != nil {
				return err
			}
			var dashboard grafanaDashboard
			if err := json.Unmarshal(data, &dashboard); err != nil {
				return fmt.Errorf("%s: %w", output.Path, err)
			}
			if dashboard.Title == "" || dashboard.SchemaVersion <= 0 || dashboard.Panels == nil {
				return fmt.Errorf("%s is missing required Grafana dashboard fields", output.Path)
			}
			if stringValue(dashboard.OpenExit["sourceRef"]) != conversion.SourceRef || dashboard.OpenExit["productionReady"] != false {
				return fmt.Errorf("%s has invalid OpenExit source metadata", output.Path)
			}
		}
	}
	return nil
}

func validatePrometheusOutputs(stage string, plan *MigrationPlan) error {
	for _, conversion := range plan.Resources {
		for _, output := range conversion.Outputs {
			if output.Kind != "prometheus-alert-rule" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(output.Path)))
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "vector(0)") {
				return fmt.Errorf("%s contains a fake vector(0) placeholder", output.Path)
			}
			var rules prometheusRuleFile
			if err := yaml.Unmarshal(data, &rules); err != nil {
				return fmt.Errorf("%s: %w", output.Path, err)
			}
			if len(rules.Groups) != 1 || len(rules.Groups[0].Rules) != 1 {
				return fmt.Errorf("%s must contain exactly one source-linked rule", output.Path)
			}
			rule := rules.Groups[0].Rules[0]
			if !promIdentifierPattern.MatchString(rule.Alert) || strings.TrimSpace(rule.Expr) == "" {
				return fmt.Errorf("%s has an invalid alert name or empty expression", output.Path)
			}
			if rule.Labels["source_ref"] != conversion.SourceRef || rule.Labels["production_ready"] != "false" || rule.Labels["openexit_candidate"] != "true" {
				return fmt.Errorf("%s has invalid source or safety labels", output.Path)
			}
			if rule.Annotations["openexit_source_query"] == "" || rule.Annotations["openexit_review"] == "" {
				return fmt.Errorf("%s does not preserve source query and review guidance", output.Path)
			}
		}
	}
	return nil
}

func validateTelemetryOutputs(stage string, inv *Inventory) error {
	alloyData, err := os.ReadFile(filepath.Join(stage, "generated", "alloy", "config.alloy"))
	if err != nil {
		return err
	}
	otelData, err := os.ReadFile(filepath.Join(stage, "generated", "opentelemetry", "collector.yaml"))
	if err != nil {
		return err
	}
	alloy := string(alloyData)
	for _, marker := range []string{"otelcol.receiver.otlp", "otelcol.processor.batch", "otelcol.exporter.otlphttp", `sys.env("OPENEXIT_OTLP_ENDPOINT")`} {
		if !strings.Contains(alloy, marker) {
			return fmt.Errorf("alloy candidate is missing %s", marker)
		}
	}
	var otel map[string]any
	if err := yaml.Unmarshal(otelData, &otel); err != nil {
		return fmt.Errorf("OpenTelemetry candidate: %w", err)
	}
	for _, key := range []string{"receivers", "processors", "exporters", "service"} {
		if _, ok := otel[key]; !ok {
			return fmt.Errorf("OpenTelemetry candidate is missing %s", key)
		}
	}
	for _, ref := range configSourceRefs(inv.Resources) {
		if !strings.Contains(alloy, ref) || !strings.Contains(string(otelData), ref) {
			return fmt.Errorf("telemetry candidates do not preserve source reference %s", ref)
		}
	}
	return nil
}

func validateNoSecrets(roots ...string) error {
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if secretValuePattern.Match(data) {
				return fmt.Errorf("secret-like value found in %s", path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaFile(schemaFile, path string) error {
	schemaData, err := publicschemas.FS.ReadFile(schemaFile)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return err
	}
	if err := compiler.AddResource(schemaFile, document); err != nil {
		return err
	}
	schema, err := compiler.Compile(schemaFile)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

var hrefPattern = regexp.MustCompile(`href="([^"]+)"`)

func validateHTMLLinks(stage, workDir string) error {
	data, err := os.ReadFile(filepath.Join(stage, ReportRel))
	if err != nil {
		return err
	}
	for _, match := range hrefPattern.FindAllStringSubmatch(string(data), -1) {
		if len(match) != 2 {
			continue
		}
		target := html.UnescapeString(match[1])
		if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "#") {
			continue
		}
		pathPart, _, _ := strings.Cut(target, "#")
		if err := SafeRelativePath(pathPart); err != nil {
			return fmt.Errorf("unsafe report link %q", target)
		}
		root := stage
		if strings.HasPrefix(pathPart, "inventory/") || strings.HasPrefix(pathPart, "evidence/") {
			root = workDir
		}
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(pathPart))); err != nil || info.IsDir() {
			return fmt.Errorf("broken report link %q", target)
		}
	}
	return nil
}
