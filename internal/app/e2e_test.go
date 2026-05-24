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
}

func TestGitHubEnterpriseForgejoFixturePipeline(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ghe-demo")
	fixturePath := filepath.Join("..", "..", "testdata", "github-enterprise", "small.json")

	commands := [][]string{
		{"init", projectDir},
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
		"assessment/openexit.assessment.yaml",
		"assessment/forgejo-migration-assessment.md",
		"assessment/ci-compatibility-report.md",
		"assessment/branch-protection-mapping.md",
		"assessment/runner-migration-plan.md",
		"assessment/repository-ownership-report.md",
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
		"assessment/openexit.assessment.yaml",
		"assessment/openexit.assessment.json",
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
	defer reader.Close()

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
