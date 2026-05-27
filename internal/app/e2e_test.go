package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	for _, marker := range []string{"jsonschema-project: passed", "grafana-dashboard-candidates: passed", "prometheus-rule-candidates: passed", "opentelemetry-candidate: passed"} {
		if !strings.Contains(string(report), marker) {
			t.Fatalf("expected validation report marker %q, got:\n%s", marker, string(report))
		}
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
}

func TestIdentityFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "identity-demo")
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
}

func TestEdgeFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "edge-demo")
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
}

func TestAIProviderFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ai-demo")
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

func executeForTest(args ...string) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
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
	}
}

func expectedBundleFiles() []string {
	files := []string{
		"openexit-evidence/README.md",
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
	return verifyChecksums(files)
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
