package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestDefinitionOfDonePipelineAndBundle(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	bundlePath := filepath.Join(t.TempDir(), "openexit-demo.zip")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")

	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
		{"export", "--project", projectDir, "--format", "zip", "--out", bundlePath},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}

	for _, rel := range expectedProjectFiles() {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", rel, err)
		}
	}
	if err := assertBundle(bundlePath, expectedBundleFiles()); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"jsonschema-project: passed", "project-path-safety: passed", "grafana-dashboard-candidates: passed", "prometheus-rule-candidates: passed", "opentelemetry-candidate: passed", "argocd-candidate: passed"} {
		if !strings.Contains(string(report), marker) {
			t.Fatalf("expected validation report marker %q, got:\n%s", marker, string(report))
		}
	}
}

func TestVerifyBundleCommand(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	bundlePath := filepath.Join(t.TempDir(), "openexit-demo.zip")
	if _, err := executeForTestWithOutput("demo", projectDir, "--out", bundlePath); err != nil {
		t.Fatalf("openexit demo failed: %v", err)
	}

	out, err := executeForTestWithOutput("verify-bundle", bundlePath)
	if err != nil {
		t.Fatalf("openexit verify-bundle failed: %v\n%s", err, out)
	}
	for _, marker := range []string{
		"status: passed",
		"project: demo (datadog -> grafana-lgtm)",
		"validation: passed",
		"files: archive=",
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected verify-bundle marker %q, got:\n%s", marker, out)
		}
	}

	jsonOut, err := executeForTestWithOutput("verify-bundle", bundlePath, "--json")
	if err != nil {
		t.Fatalf("openexit verify-bundle --json failed: %v\n%s", err, jsonOut)
	}
	var report struct {
		Status        string `json:"status"`
		ManifestFiles int    `json:"manifestFiles"`
		Project       struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("decode verify-bundle JSON: %v\n%s", err, jsonOut)
	}
	if report.Status != "passed" || report.ManifestFiles == 0 || report.Project.Source != "datadog" || report.Project.Target != "grafana-lgtm" {
		t.Fatalf("unexpected verify-bundle JSON report: %+v", report)
	}
}

func TestVerifyBundleCommandRejectsTampering(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	bundlePath := filepath.Join(t.TempDir(), "openexit-demo.zip")
	if _, err := executeForTestWithOutput("demo", projectDir, "--out", bundlePath); err != nil {
		t.Fatalf("openexit demo failed: %v", err)
	}
	corruptBundleEntry(t, bundlePath, "openexit-evidence/openexit.yaml", []byte("tampered: true\n"))

	out, err := executeForTestWithOutput("verify-bundle", bundlePath)
	if err == nil {
		t.Fatalf("expected verify-bundle to reject tampered bundle, got:\n%s", out)
	}
	for _, marker := range []string{
		"status: failed",
		"manifest digest mismatch for openexit.yaml",
		"checksum digest mismatch for openexit.yaml",
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected tamper marker %q, got:\n%s", marker, out)
		}
	}
}

func TestDoctorCommandReportsRuntimeReadiness(t *testing.T) {
	out, err := executeForTestWithOutput("doctor")
	if err != nil {
		t.Fatalf("openexit doctor failed: %v\n%s", err, out)
	}
	for _, marker := range []string{
		"status:",
		"version-metadata:",
		"schema-bundle: passed",
		"promtool:",
		"kubeconform:",
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected doctor marker %q, got:\n%s", marker, out)
		}
	}

	jsonOut, err := executeForTestWithOutput("doctor", "--json")
	if err != nil {
		t.Fatalf("openexit doctor --json failed: %v\n%s", err, jsonOut)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("decode doctor JSON: %v\n%s", err, jsonOut)
	}
	if report.Status == "" || len(report.Checks) < 4 {
		t.Fatalf("unexpected doctor report: %+v", report)
	}
	seen := map[string]bool{}
	for _, check := range report.Checks {
		seen[check.Name] = true
	}
	for _, name := range []string{"version-metadata", "schema-bundle", "promtool", "kubeconform"} {
		if !seen[name] {
			t.Fatalf("doctor report missing check %s: %+v", name, report)
		}
	}
}

func TestStatusReportsPipelineReadiness(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")

	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}

	out, err := executeForTestWithOutput("status", "--project", projectDir)
	if err != nil {
		t.Fatalf("openexit status failed: %v", err)
	}
	for _, marker := range []string{
		"source: datadog",
		"target: grafana-lgtm",
		"layout: ok",
		"inventory: present",
		"assessment: present",
		"mapping: present",
		"generated: present",
		"validation: passed",
		"export-ready: yes",
		"next: openexit export",
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected status marker %q, got:\n%s", marker, out)
		}
	}

	jsonOut, err := executeForTestWithOutput("status", "--project", projectDir, "--json")
	if err != nil {
		t.Fatalf("openexit status --json failed: %v", err)
	}
	var status ProjectStatus
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("decode status JSON: %v\n%s", err, jsonOut)
	}
	if !status.ReadyForExport || status.Validation.Status != "passed" {
		t.Fatalf("expected export-ready passed status, got %+v", status)
	}
	if status.Inventory.Assets["dashboards"] != 1 || status.Generated.Candidates == 0 || len(status.NextActions) != 1 {
		t.Fatalf("unexpected status summary: %+v", status)
	}
}

func TestRunCommandCompletesCollectedProject(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	bundlePath := filepath.Join(t.TempDir(), "openexit-run.zip")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")

	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	out, err := executeForTestWithOutput("run", "--project", projectDir, "--export", "--out", bundlePath)
	if err != nil {
		t.Fatalf("openexit run failed: %v\n%s", err, out)
	}
	for _, marker := range []string{
		"run: assess datadog -> grafana-lgtm",
		"run: map",
		"run: generate --all",
		"run: validate",
		"validation status: passed",
		"export-ready: yes",
		"exported " + bundlePath,
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected run output marker %q, got:\n%s", marker, out)
		}
	}
	for _, rel := range expectedProjectFiles() {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", rel, err)
		}
	}
	if err := assertBundle(bundlePath, expectedBundleFiles()); err != nil {
		t.Fatal(err)
	}
}

func TestDemoCommandCreatesCompleteProject(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	bundlePath := filepath.Join(t.TempDir(), "openexit-demo.zip")
	out, err := executeForTestWithOutput("demo", projectDir, "--out", bundlePath)
	if err != nil {
		t.Fatalf("openexit demo failed: %v\n%s", err, out)
	}
	for _, marker := range []string{
		"demo: datadog -> grafana-lgtm",
		"collected datadog-fixture inventory",
		"validation status: passed",
		"export-ready: yes",
		"exported " + bundlePath,
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("expected demo output marker %q, got:\n%s", marker, out)
		}
	}
	for _, rel := range expectedProjectFiles() {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", rel, err)
		}
	}
	if err := assertBundle(bundlePath, expectedBundleFiles()); err != nil {
		t.Fatal(err)
	}
}

func TestDemoCommandSupportsBuiltInSources(t *testing.T) {
	for _, source := range []string{"datadog", "github-enterprise", "identity", "edge", "ai-provider"} {
		t.Run(source, func(t *testing.T) {
			projectDir := filepath.Join(t.TempDir(), source)
			bundlePath := filepath.Join(t.TempDir(), source+".zip")
			out, err := executeForTestWithOutput("demo", projectDir, "--source", source, "--out", bundlePath)
			if err != nil {
				t.Fatalf("openexit demo --source %s failed: %v\n%s", source, err, out)
			}
			if !strings.Contains(out, "source: "+source) || !strings.Contains(out, "export-ready: yes") {
				t.Fatalf("expected ready status for source %s, got:\n%s", source, out)
			}
			if info, err := os.Stat(bundlePath); err != nil || info.Size() == 0 {
				t.Fatalf("expected non-empty demo bundle for %s: info=%+v err=%v", source, info, err)
			}
		})
	}
}

func TestDemoCommandRejectsNonEmptyProjectWithoutForce(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeForTestWithOutput("demo", projectDir); err == nil {
		t.Fatal("expected demo to reject a non-empty directory without --force")
	}
}

func TestCLICommandFailuresAreActionable(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	if err := executeForTest("init", projectDir); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"generate", "--project", projectDir}, want: "set --all or at least one --artifact"},
		{args: []string{"collect", "fixture", "--project", projectDir, "--input", filepath.Join(t.TempDir(), "missing.json")}, want: "read fixture"},
		{args: []string{"assist", "summarize", "--project", projectDir, "--provider", "not-a-provider"}, want: "unsupported assist provider"},
	}
	for _, tt := range tests {
		if err := executeForTest(tt.args...); err == nil {
			t.Fatalf("expected openexit %s to fail", strings.Join(tt.args, " "))
		} else if !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("expected openexit %s error to contain %q, got %q", strings.Join(tt.args, " "), tt.want, err.Error())
		}
	}
}

func TestInvalidFixtureInputFails(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	if err := executeForTest("init", projectDir); err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(fixturePath, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("collect", "fixture", "--project", projectDir, "--input", fixturePath); err == nil {
		t.Fatal("expected invalid fixture input to fail")
	}
}

func TestValidationScansGeneratedTextArtifacts(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	secretPath := filepath.Join(projectDir, "generated-config", "manual.candidate.conf")
	if err := os.WriteFile(secretPath, []byte("api_key: dd123456789012345678901234567890\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when generated text artifact contains a secret-like value")
	}
}

func TestValidationAndExportRejectSymlinkedBundleInputs(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "symlink-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside-secret.txt")
	if err := os.WriteFile(outside, []byte("external material must not enter bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(projectDir, "evidence", "outside-secret.txt")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not available on this platform: %v", err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when export scope contains a symlink")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "project-path-safety") || !strings.Contains(string(report), "symlink not allowed") {
		t.Fatalf("expected path safety failure in validation report, got:\n%s", string(report))
	}
	bundlePath := filepath.Join(t.TempDir(), "symlink.zip")
	if err := executeForTest("export", "--project", projectDir, "--format", "zip", "--out", bundlePath, "--force"); err == nil {
		t.Fatal("expected forced export to reject symlinked bundle input")
	}
	if _, err := os.Stat(bundlePath); err == nil {
		t.Fatal("export wrote a bundle despite symlinked bundle input")
	}
}

func TestValidationRejectsMismatchedProjectManifest(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "mismatch-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "github-enterprise", "small.json")
	commands := [][]string{
		{"init", projectDir, "--source", "github-enterprise", "--target", "forgejo"},
		{"collect", "github-fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "forgejo"},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	cfg, err := LoadProjectConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Source.Type = "datadog"
	if err := WriteProjectConfig(projectDir, *cfg); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when project source/target do not match inventory and assessment")
	}
}

func TestValidationRejectsProjectSchemaViolation(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "schema-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	configPath := filepath.Join(projectDir, "openexit.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	withoutAllowAI := strings.Replace(string(data), "    allowAI: false\n", "", 1)
	if withoutAllowAI == string(data) {
		t.Fatal("test fixture did not contain policy.allowAI")
	}
	if err := os.WriteFile(configPath, []byte(withoutAllowAI), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when project manifest violates JSON Schema")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "jsonschema-project") || !strings.Contains(string(report), "allowAI") {
		t.Fatalf("expected schema failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsProductionReadyGrafanaCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "grafana-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	dashboardPath := filepath.Join(projectDir, "generated-config", "grafana", "dashboards", "production-api-overview.candidate.json")
	data, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), `"productionReady": "false"`, `"productionReady": "true"`, 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain productionReady metadata")
	}
	if err := os.WriteFile(dashboardPath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when a Grafana candidate is marked production ready")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "grafana-dashboard-candidates") || !strings.Contains(string(report), "productionReady") {
		t.Fatalf("expected Grafana candidate failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsProductionReadyPrometheusCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "prometheus-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	rulesPath := filepath.Join(projectDir, "generated-config", "prometheus", "rules", "datadog-monitor-candidates.yaml")
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), `production_ready: "false"`, `production_ready: "true"`, 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain production_ready label")
	}
	if err := os.WriteFile(rulesPath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when a Prometheus candidate is marked production ready")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "prometheus-rule-candidates") || !strings.Contains(string(report), "production_ready") {
		t.Fatalf("expected Prometheus candidate failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenOpenTelemetryCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "otel-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	configPath := filepath.Join(projectDir, "generated-config", "opentelemetry", "collector.candidate.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "    traces:\n      receivers: [otlp]\n      processors: [memory_limiter, batch]\n      exporters: [otlp/tempo]\n", "", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain traces pipeline")
	}
	if err := os.WriteFile(configPath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when OpenTelemetry traces pipeline is missing")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "opentelemetry-candidate") || !strings.Contains(string(report), "service.pipelines.traces") {
		t.Fatalf("expected OpenTelemetry candidate failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsAutomatedArgoCDCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "argocd-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "grafana-lgtm"},
		{"map", "--project", projectDir},
		{"generate", "--project", projectDir, "--all"},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	appPath := filepath.Join(projectDir, "generated-config", "argocd", "grafana-stack-application.candidate.yaml")
	data, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "  syncPolicy: {}\n", "  syncPolicy:\n    automated:\n      prune: true\n", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain empty syncPolicy")
	}
	if err := os.WriteFile(appPath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when ArgoCD candidate enables automated sync")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "argocd-candidate") || !strings.Contains(string(report), "syncPolicy.automated") {
		t.Fatalf("expected ArgoCD candidate failure in validation report, got:\n%s", string(report))
	}
}

func TestCollectRejectsMismatchedProjectSource(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "mismatch-collect")
	fixturePath := filepath.Join("..", "..", "testdata", "github-enterprise", "small.json")
	if err := executeForTest("init", projectDir); err != nil {
		t.Fatal(err)
	}
	err := executeForTest("collect", "github-fixture", "--project", projectDir, "--input", fixturePath)
	if err == nil {
		t.Fatal("expected collect to fail for a mismatched project source")
	}
	if !strings.Contains(err.Error(), "project source") {
		t.Fatalf("expected project source error, got %q", err.Error())
	}
}

func TestAssessRejectsMismatchedProjectTarget(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "mismatch-assess")
	fixturePath := filepath.Join("..", "..", "testdata", "datadog", "small.json")
	commands := [][]string{
		{"init", projectDir},
		{"collect", "fixture", "--project", projectDir, "--input", fixturePath},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
	err := executeForTest("assess", "--project", projectDir, "--target", "forgejo")
	if err == nil {
		t.Fatal("expected assess to fail for a mismatched target")
	}
	if !strings.Contains(err.Error(), "assessment target") {
		t.Fatalf("expected assessment target error, got %q", err.Error())
	}
}

func TestGitHubEnterpriseForgejoFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ghe-demo")
	runGitHubEnterpriseFixturePipeline(t, projectDir)
	for _, rel := range []string{
		"inventory/openexit.inventory.yaml",
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.migration-plan.yaml",
		"assessment/migration-plan.md",
		"assessment/forgejo-migration-assessment.md",
		"assessment/ci-compatibility-report.md",
		"assessment/branch-protection-mapping.md",
		"assessment/runner-migration-plan.md",
		"assessment/repository-ownership-report.md",
		"generated-config/forgejo/migration-candidate.yaml",
		"evidence/github-enterprise/repositories/platform-api.json",
		"evidence/github-enterprise/actions-workflows/platform-api-github-workflows-ci-yml.json",
		"validation/openexit.validation.yaml",
	} {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected GitHub generated file %s: %v", rel, err)
		}
	}
	assessmentData, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"ghe.actions.github-hosted-runner.001",
		"ghe.branch-protection.force-push.001",
		"ghe.deploy-key.write-access.001",
		"ghe.github-app.webhook-review.001",
	} {
		if !strings.Contains(string(assessmentData), id) {
			t.Fatalf("expected assessment finding %s", id)
		}
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "forgejo-migration-candidate: passed") {
		t.Fatalf("expected Forgejo candidate validation marker, got:\n%s", string(report))
	}
}

func TestValidationRejectsProductionReadyForgejoCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ghe-demo")
	runGitHubEnterpriseFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "forgejo", "migration-candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "productionReady: false", "productionReady: true", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain productionReady marker")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when Forgejo candidate is marked production ready")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "forgejo-migration-candidate") || !strings.Contains(string(report), "productionReady") {
		t.Fatalf("expected Forgejo candidate productionReady failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenForgejoRepositoryCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ghe-demo")
	runGitHubEnterpriseFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "forgejo", "migration-candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "targetRepository: platform/api", "targetRepository: platform/api-drifted", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain targetRepository metadata")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when Forgejo repository candidate drifts from inventory")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "forgejo-migration-candidate") || !strings.Contains(string(report), "targetRepository") {
		t.Fatalf("expected Forgejo candidate targetRepository failure in validation report, got:\n%s", string(report))
	}
}

func TestIdentityFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "identity-demo")
	runIdentityFixturePipeline(t, projectDir)
	for _, rel := range []string{
		"inventory/openexit.inventory.yaml",
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.migration-plan.yaml",
		"assessment/migration-plan.md",
		"assessment/identity-migration-risk-register.md",
		"assessment/break-glass-checklist.md",
		"assessment/identity-cutover-plan.md",
		"assessment/identity-rollback-plan.md",
		"generated-config/identity/realm-client-candidate.yaml",
		"evidence/identity/applications/app-api.json",
		"evidence/identity/mfa-settings/default.json",
		"validation/openexit.validation.yaml",
	} {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected identity generated file %s: %v", rel, err)
		}
	}
	assessmentData, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"identity.oidc.implicit-grant.001",
		"identity.redirect-uri.insecure.001",
		"identity.mfa.not-required.001",
		"identity.break-glass.mfa-missing.001",
	} {
		if !strings.Contains(string(assessmentData), id) {
			t.Fatalf("expected assessment finding %s", id)
		}
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "identity-realm-client-candidate: passed") {
		t.Fatalf("expected identity candidate validation marker, got:\n%s", string(report))
	}
}

func TestValidationRejectsProductionReadyIdentityCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "identity-demo")
	runIdentityFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "identity", "realm-client-candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "productionReady: false", "productionReady: true", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain productionReady marker")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when identity candidate is marked production ready")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "identity-realm-client-candidate") || !strings.Contains(string(report), "productionReady") {
		t.Fatalf("expected identity candidate productionReady failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenIdentityClientCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "identity-demo")
	runIdentityFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "identity", "realm-client-candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "clientId: api-web", "clientId: api-web-drifted", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain clientId metadata")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when identity client candidate drifts from inventory")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "identity-realm-client-candidate") || !strings.Contains(string(report), "clientId") {
		t.Fatalf("expected identity candidate clientId failure in validation report, got:\n%s", string(report))
	}
}

func TestEdgeFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "edge-demo")
	runEdgeFixturePipeline(t, projectDir)
	for _, rel := range []string{
		"inventory/openexit.inventory.yaml",
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.migration-plan.yaml",
		"assessment/migration-plan.md",
		"assessment/cache-parity-report.md",
		"assessment/waf-enforcement-risk-report.md",
		"generated-config/edge/varnish/default.candidate.vcl",
		"generated-config/edge/haproxy/haproxy.candidate.cfg",
		"generated-config/edge/coraza/coraza-rules.candidate.conf",
		"evidence/edge/waf-rules/waf-sqli.json",
		"evidence/edge/cache-rules/cache-static.json",
		"validation/openexit.validation.yaml",
	} {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected edge generated file %s: %v", rel, err)
		}
	}
	assessmentData, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"edge.waf.managed-rule.001",
		"edge.waf.observe-only.001",
		"edge.origin.tls-verify-disabled.001",
		"edge.tls.minimum-version-low.001",
	} {
		if !strings.Contains(string(assessmentData), id) {
			t.Fatalf("expected assessment finding %s", id)
		}
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "edge-candidates: passed") {
		t.Fatalf("expected edge candidate validation marker, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenEdgeVCLCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "edge-demo")
	runEdgeFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "edge", "varnish", "default.candidate.vcl")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "set beresp.ttl = 86400s", "set beresp.ttl = 60s", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain VCL edge TTL")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when VCL TTL drifts from inventory")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "edge-candidates") || !strings.Contains(string(report), "edge TTL") {
		t.Fatalf("expected edge candidate TTL failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenEdgeHAProxyCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "edge-demo")
	runEdgeFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "edge", "haproxy", "haproxy.candidate.cfg")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "ssl verify none sni str(api-origin.example.com)", "ssl verify required sni str(api-origin.example.com)", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain HAProxy TLS verify marker")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when HAProxy TLS verification drifts from inventory")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "edge-candidates") || !strings.Contains(string(report), "TLS verify none") {
		t.Fatalf("expected edge candidate TLS verification failure in validation report, got:\n%s", string(report))
	}
}

func TestAIProviderFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ai-demo")
	runAIProviderFixturePipeline(t, projectDir)
	for _, rel := range []string{
		"inventory/openexit.inventory.yaml",
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.migration-plan.yaml",
		"assessment/migration-plan.md",
		"assessment/self-hosted-llm-readiness-report.md",
		"assessment/vllm-sizing-assumptions.md",
		"assessment/evaluation-plan.md",
		"assessment/data-sensitivity-report.md",
		"generated-config/ai/litellm/config.candidate.yaml",
		"evidence/ai-provider/model-usage-classes/support-chat.json",
		"evidence/ai-provider/tool-usages/ticket-update.json",
		"validation/openexit.validation.yaml",
	} {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected AI generated file %s: %v", rel, err)
		}
	}
	assessmentData, err := os.ReadFile(filepath.Join(projectDir, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{
		"ai.capacity.peak-throughput.001",
		"ai.data.sensitive-prompts.001",
		"ai.latency.tight-slo.001",
		"ai.tool.external-write.001",
	} {
		if !strings.Contains(string(assessmentData), id) {
			t.Fatalf("expected assessment finding %s", id)
		}
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "litellm-config-candidate: passed") {
		t.Fatalf("expected LiteLLM candidate validation marker, got:\n%s", string(report))
	}
}

func TestValidationRejectsProductionReadyLiteLLMCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ai-demo")
	runAIProviderFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "ai", "litellm", "config.candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "productionReady: false", "productionReady: true", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain productionReady marker")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when LiteLLM candidate is marked production ready")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "litellm-config-candidate") || !strings.Contains(string(report), "productionReady") {
		t.Fatalf("expected LiteLLM candidate productionReady failure in validation report, got:\n%s", string(report))
	}
}

func TestValidationRejectsBrokenLiteLLMRouteCandidate(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ai-demo")
	runAIProviderFixturePipeline(t, projectDir)
	candidatePath := filepath.Join(projectDir, "generated-config", "ai", "litellm", "config.candidate.yaml")
	data, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.Replace(string(data), "model: vllm/support-chat-replacement-for-gpt-4o", "model: vllm/support-chat-unreviewed", 1)
	if corrupt == string(data) {
		t.Fatal("test fixture did not contain LiteLLM model route")
	}
	if err := os.WriteFile(candidatePath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executeForTest("validate", "--project", projectDir); err == nil {
		t.Fatal("expected validation to fail when LiteLLM route drifts from inventory")
	}
	report, err := os.ReadFile(filepath.Join(projectDir, "validation", "validation-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "litellm-config-candidate") || !strings.Contains(string(report), "litellm_params") {
		t.Fatalf("expected LiteLLM route failure in validation report, got:\n%s", string(report))
	}
}

func TestExportRefusesInvalidProjectWithoutForce(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "broken")
	if err := executeForTest("init", projectDir); err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(t.TempDir(), "broken.zip")
	err := executeForTest("export", "--project", projectDir, "--format", "zip", "--out", bundlePath)
	if err == nil {
		t.Fatal("expected export to fail when validation has critical failures")
	}
	if _, statErr := os.Stat(bundlePath); statErr == nil {
		t.Fatal("export wrote a bundle despite validation failure")
	}
}

func runGitHubEnterpriseFixturePipeline(t *testing.T, projectDir string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "testdata", "github-enterprise", "small.json")
	commands := [][]string{
		{"init", projectDir, "--source", "github-enterprise", "--target", "forgejo"},
		{"collect", "github-fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "forgejo"},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
}

func runIdentityFixturePipeline(t *testing.T, projectDir string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "testdata", "identity", "small.json")
	commands := [][]string{
		{"init", projectDir, "--source", "identity", "--target", "keycloak-zitadel"},
		{"collect", "identity-fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "keycloak-zitadel"},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
}

func runEdgeFixturePipeline(t *testing.T, projectDir string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "testdata", "edge", "small.json")
	commands := [][]string{
		{"init", projectDir, "--source", "edge", "--target", "varnish-haproxy-coraza"},
		{"collect", "edge-fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "varnish-haproxy-coraza"},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
}

func runAIProviderFixturePipeline(t *testing.T, projectDir string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "testdata", "ai-provider", "small.json")
	commands := [][]string{
		{"init", projectDir, "--source", "ai-provider", "--target", "vllm-litellm"},
		{"collect", "ai-fixture", "--project", projectDir, "--input", fixturePath},
		{"assess", "--project", projectDir, "--target", "vllm-litellm"},
		{"generate", "--project", projectDir, "--all"},
		{"validate", "--project", projectDir},
	}
	for _, args := range commands {
		if err := executeForTest(args...); err != nil {
			t.Fatalf("openexit %s failed: %v", strings.Join(args, " "), err)
		}
	}
}

func executeForTest(args ...string) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func executeForTestWithOutput(args ...string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func expectedProjectFiles() []string {
	return []string{
		"openexit.yaml",
		"inventory/openexit.inventory.yaml",
		"inventory/openexit.inventory.json",
		"mapping/openexit.mapping.yaml",
		"mapping/openexit.mapping.json",
		"mapping/mapping-summary.md",
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.assessment.json",
		"assessment/openexit.migration-plan.yaml",
		"assessment/openexit.migration-plan.json",
		"assessment/migration-plan.md",
		"assessment/assessment.md",
		"assessment/risk-register.md",
		"assessment/manual-review.md",
		"assessment/cost-drivers.md",
		"assessment/target-architecture.md",
		"assessment/acceptance-criteria.md",
		"assessment/rollback-plan.md",
		"assessment/runbook.md",
		"assessment/restore-drill-checklist.md",
		"assessment/alert-shadowing-plan.md",
		"generated-config/grafana/README.md",
		"generated-config/grafana/dashboards/production-api-overview.candidate.json",
		"generated-config/prometheus/README.md",
		"generated-config/prometheus/rules/datadog-monitor-candidates.yaml",
		"generated-config/opentelemetry/README.md",
		"generated-config/opentelemetry/collector.candidate.yaml",
		"generated-config/argocd/README.md",
		"generated-config/argocd/grafana-stack-application.candidate.yaml",
		"validation/openexit.validation.yaml",
		"validation/validation-report.md",
		"evidence/datadog/raw-fixture.json",
		"evidence/datadog/dashboards/abc-123.json",
		"evidence/datadog/monitors/123456.json",
		"evidence/datadog/monitors/789012.json",
		"evidence/datadog/slos/slo-123.json",
		"evidence/datadog/integrations/kubernetes.json",
	}
}

func expectedBundleFiles() []string {
	files := []string{
		"openexit-evidence/README.md",
		"openexit-evidence/manifest.json",
		"openexit-evidence/checksums.txt",
	}
	for _, rel := range expectedProjectFiles() {
		files = append(files, "openexit-evidence/"+rel)
	}
	return files
}

func assertBundle(bundlePath string, expected []string) error {
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	files := map[string][]byte{}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		files[file.Name] = data
	}
	for _, name := range expected {
		if _, ok := files[name]; !ok {
			return &missingBundleFileError{name: name}
		}
	}
	readme := files["openexit-evidence/README.md"]
	for _, marker := range [][]byte{[]byte("Version:"), []byte("Commit:"), []byte("Bundle generated at:"), []byte("not production-ready")} {
		if !bytes.Contains(readme, marker) {
			return &checksumError{message: "bundle README missing marker " + string(marker)}
		}
	}
	if err := verifyBundleManifest(files); err != nil {
		return err
	}
	return verifyChecksums(files)
}

func verifyBundleManifest(files map[string][]byte) error {
	data, ok := files["openexit-evidence/manifest.json"]
	if !ok {
		return &missingBundleFileError{name: "openexit-evidence/manifest.json"}
	}
	var manifest struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Build      struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			Date    string `json:"date"`
		} `json:"build"`
		Project struct {
			Name   string `json:"name"`
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"project"`
		Validation struct {
			Status   string `json:"status"`
			Checks   int    `json:"checks"`
			Passed   int    `json:"passed"`
			Failed   int    `json:"failed"`
			Warnings int    `json:"warnings"`
		} `json:"validation"`
		Files []struct {
			Path   string `json:"path"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if err := validateBundleManifestSchema(data); err != nil {
		return err
	}
	if manifest.APIVersion != "openexit.dev/v1alpha1" || manifest.Kind != "EvidenceBundle" {
		return &checksumError{message: "unexpected bundle manifest identity"}
	}
	if manifest.Project.Source != "datadog" || manifest.Project.Target != "grafana-lgtm" {
		return &checksumError{message: "bundle manifest has wrong source/target"}
	}
	if manifest.Validation.Status != "passed" || manifest.Validation.Checks == 0 || manifest.Validation.Failed != 0 {
		return &checksumError{message: "bundle manifest has wrong validation summary"}
	}
	if manifest.Build.Version == "" || manifest.Build.Commit == "" || manifest.Build.Date == "" {
		return &checksumError{message: "bundle manifest missing build metadata"}
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Files {
		data, ok := files["openexit-evidence/"+entry.Path]
		if !ok {
			return &missingBundleFileError{name: "openexit-evidence/" + entry.Path}
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != entry.SHA256 || int64(len(data)) != entry.Size {
			return &checksumError{message: "bundle manifest digest mismatch for " + entry.Path}
		}
		seen[entry.Path] = true
	}
	for _, rel := range []string{"openexit.yaml", "validation/openexit.validation.yaml", "generated-config/prometheus/rules/datadog-monitor-candidates.yaml"} {
		if !seen[rel] {
			return &missingBundleFileError{name: "bundle manifest entry " + rel}
		}
	}
	return nil
}

func validateBundleManifestSchema(data []byte) error {
	schemaData, err := publicschemas.FS.ReadFile("openexit.evidence-bundle.schema.json")
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return err
	}
	if err := compiler.AddResource("openexit.evidence-bundle.schema.json", document); err != nil {
		return err
	}
	schema, err := compiler.Compile("openexit.evidence-bundle.schema.json")
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func corruptBundleEntry(t *testing.T, bundlePath, entryName string, replacement []byte) {
	t.Helper()
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	tmpPath := bundlePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	replaced := false
	for _, file := range reader.File {
		header := file.FileHeader
		w, err := zw.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == entryName {
			if _, err := w.Write(replacement); err != nil {
				t.Fatal(err)
			}
			replaced = true
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(w, rc)
		closeErr := rc.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Fatalf("bundle entry %s not found", entryName)
	}
	if err := os.Rename(tmpPath, bundlePath); err != nil {
		t.Fatal(err)
	}
}

func verifyChecksums(files map[string][]byte) error {
	checksumData, ok := files["openexit-evidence/checksums.txt"]
	if !ok {
		return &missingBundleFileError{name: "openexit-evidence/checksums.txt"}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(checksumData)), "\n") {
		sum, rel, ok := strings.Cut(line, "  ")
		if !ok {
			return &checksumError{message: "malformed checksum line: " + line}
		}
		data, ok := files["openexit-evidence/"+rel]
		if !ok {
			return &missingBundleFileError{name: "openexit-evidence/" + rel}
		}
		actualBytes := sha256.Sum256(data)
		actual := hex.EncodeToString(actualBytes[:])
		if !bytes.Equal([]byte(sum), []byte(actual)) {
			return &checksumError{message: "checksum mismatch for " + rel}
		}
	}
	return nil
}

type missingBundleFileError struct {
	name string
}

func (err *missingBundleFileError) Error() string {
	return "missing bundle file " + err.name
}

type checksumError struct {
	message string
}

func (err *checksumError) Error() string {
	return err.message
}
