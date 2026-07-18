package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatadogV01CLIWorkflow(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, ".openexit")
	exportDir := filepath.Join(root, "migration")
	fixture := filepath.Join("..", "..", "testdata", "datadog", "small.json")

	out, err := executeForTestWithOutput("datadog", "scan", "--fixture", fixture, "--workdir", workDir)
	if err != nil {
		t.Fatalf("datadog scan: %v\n%s", err, out)
	}
	for _, marker := range []string{"resources: 7", "catalog-complete: true", "digest:"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("scan output missing %q:\n%s", marker, out)
		}
	}

	out, err = executeForTestWithOutput("datadog", "plan", "--target", "grafana-lgtm", "--workdir", workDir)
	if err != nil {
		t.Fatalf("datadog plan: %v\n%s", err, out)
	}
	for _, marker := range []string{"exit-readiness:", "validation: passed", "report:"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("plan output missing %q:\n%s", marker, out)
		}
	}

	out, err = executeForTestWithOutput("datadog", "export", "--out", exportDir, "--workdir", workDir)
	if err != nil {
		t.Fatalf("datadog export: %v\n%s", err, out)
	}
	for _, marker := range []string{"exported:", "plan-id:", "files:"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("export output missing %q:\n%s", marker, out)
		}
	}
	for _, rel := range []string{
		"inventory/datadog.inventory.json",
		"plan/openexit.plan.json",
		"generated/grafana/dashboards",
		"generated/prometheus/rules",
		"generated/alloy/config.alloy",
		"generated/opentelemetry/collector.yaml",
		"validation/validation.json",
		"index.html",
		"manifest.json",
		"SHA256SUMS",
	} {
		if _, err := os.Stat(filepath.Join(exportDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing exported %s: %v", rel, err)
		}
	}
}

func TestPrimaryCommandSurfaceKeepsLegacyProvidersExperimental(t *testing.T) {
	root := NewRootCommand()
	visible := map[string]bool{}
	for _, command := range root.Commands() {
		if !command.Hidden {
			visible[command.Name()] = true
		}
	}
	if !visible["datadog"] || !visible["experimental"] {
		t.Fatalf("primary command surface is missing datadog or experimental: %#v", visible)
	}
	for _, legacy := range []string{"init", "demo", "run", "collect", "assess", "map", "generate", "validate", "export", "assist"} {
		if visible[legacy] {
			t.Fatalf("legacy command %s should be hidden from the primary surface", legacy)
		}
	}
	experimental, _, err := root.Find([]string{"experimental"})
	if err != nil {
		t.Fatal(err)
	}
	if command, _, err := experimental.Find([]string{"collect"}); err != nil || command.Name() != "collect" {
		t.Fatalf("experimental provider collector is unavailable: command=%v err=%v", command, err)
	}
}
