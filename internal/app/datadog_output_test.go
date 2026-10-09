package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/datadogplan"
)

func TestDatadogJSONWorkflow(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "state")
	outDir := filepath.Join(root, "migration")
	for _, tt := range []struct {
		name string
		args []string
		path string
		kind string
	}{
		{"scan", []string{"scan", "--fixture", "../../testdata/datadog/small.json"}, filepath.Join(workDir, datadogplan.InventoryRel), datadogplan.InventoryKind},
		{"plan", []string{"plan"}, filepath.Join(workDir, datadogplan.PlanRel), datadogplan.PlanKind},
		{"export", []string{"export", "--out", outDir}, filepath.Join(outDir, datadogplan.BundleManifestRel), datadogplan.BundleKind},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"datadog"}, tt.args...)
			args = append(args, "--workdir", workDir, "--json")
			out, err := executeForTestWithOutput(args...)
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal([]byte(out), &document); err != nil {
				t.Fatalf("stdout must contain exactly one JSON document: %v\n%s", err, out)
			}
			if document["kind"] != tt.kind {
				t.Fatalf("kind = %v, want %s", document["kind"], tt.kind)
			}
			saved, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(saved, []byte(out)) {
				t.Fatal("JSON stdout differs from the persisted document")
			}
		})
	}
	for _, jsonOutput := range []bool{false, true} {
		args := []string{"verify-bundle", outDir}
		if jsonOutput {
			args = append(args, "--json")
		}
		out, err := executeForTestWithOutput(args...)
		if err != nil || (jsonOutput && !json.Valid([]byte(out))) || (!jsonOutput && !strings.Contains(out, "files: directory=")) {
			t.Fatalf("verify directory json=%t: %v\n%s", jsonOutput, err, out)
		}
	}
}

func TestDatadogJSONErrors(t *testing.T) {
	for _, args := range [][]string{
		{"scan", "--fixture", "missing.json"},
		{"plan", "--target", "unsupported"},
		{"export", "--out", filepath.Join(t.TempDir(), "migration")},
	} {
		t.Run(args[0], func(t *testing.T) {
			command := append([]string{"datadog"}, args...)
			command = append(command, "--workdir", t.TempDir(), "--json")
			out, err := executeForTestWithOutput(command...)
			if err == nil || out != "" {
				t.Fatalf("expected an error with empty stdout, got err=%v stdout=%q", err, out)
			}
		})
	}
}

func TestDatadogJSONPartialScan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("DATADOG_API_KEY", "test-api-key")
	t.Setenv("DATADOG_APP_KEY", "test-app-key")
	for _, allowPartial := range []bool{false, true} {
		name := "fail closed"
		if allowPartial {
			name = "explicitly accepted"
		}
		t.Run(name, func(t *testing.T) {
			args := []string{"datadog", "scan", "--workdir", t.TempDir(), "--base-url", server.URL, "--json"}
			if allowPartial {
				args = append(args, "--allow-partial")
			}
			out, err := executeForTestWithOutput(args...)
			if (err == nil) != allowPartial {
				t.Fatalf("allow-partial=%t, err=%v", allowPartial, err)
			}
			var inv datadogplan.Inventory
			if err := json.Unmarshal([]byte(out), &inv); err != nil {
				t.Fatal(err)
			}
			if inv.Kind != datadogplan.InventoryKind || inv.Catalog.Complete || len(inv.Catalog.Coverage) == 0 {
				t.Fatalf("partial inventory not reported: %+v", inv)
			}
		})
	}
}

func TestDatadogJSONFailedPlan(t *testing.T) {
	workDir, plan := datadogPlanForTest(t)
	if err := os.WriteFile(filepath.Join(workDir, filepath.FromSlash(plan.Resources[0].EvidencePath)), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := executeForTestWithOutput("datadog", "plan", "--workdir", workDir, "--json")
	if err == nil || !json.Valid([]byte(out)) {
		t.Fatalf("failed plan must keep its nonzero exit and JSON result: %v\n%s", err, out)
	}
}

func TestDatadogExplain(t *testing.T) {
	workDir, plan := datadogPlanForTest(t)
	before, err := os.ReadFile(filepath.Join(workDir, datadogplan.PlanRel))
	if err != nil {
		t.Fatal(err)
	}
	for _, conversion := range plan.Resources {
		t.Run(conversion.SourceRef, func(t *testing.T) {
			args := []string{"datadog", "explain", conversion.SourceRef, "--workdir", workDir}
			out, err := executeForTestWithOutput(args...)
			if err != nil {
				t.Fatal(err)
			}
			for _, marker := range []string{conversion.SourceRef, conversion.SourceName, "status: " + conversion.Status, conversion.Summary, conversion.EvidencePath} {
				if !strings.Contains(out, marker) {
					t.Fatalf("explanation missing %q:\n%s", marker, out)
				}
			}
			for _, component := range conversion.Components {
				for _, marker := range []string{component.ID, component.SourceQuery, component.TargetQuery, component.Review} {
					if !strings.Contains(out, marker) {
						t.Fatalf("explanation missing component detail %q", marker)
					}
				}
			}
			out, err = executeForTestWithOutput(append(args, "--json")...)
			if err != nil {
				t.Fatal(err)
			}
			var got datadogplan.Conversion
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, conversion) {
				t.Fatalf("explanation does not match the recorded decision: %+v", got)
			}
		})
	}
	after, err := os.ReadFile(filepath.Join(workDir, datadogplan.PlanRel))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("explain changed the plan: %v", err)
	}
}

func TestDatadogExplainErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		ref    string
		change string
		want   string
	}{
		{"unknown", "datadog:monitor:missing", "", "not found"},
		{"short reference", "123456", "", "full sourceRef"},
		{"missing plan", "datadog:monitor:123456", "remove", "run datadog plan first"},
		{"malformed plan", "datadog:monitor:123456", "malformed", "parse"},
		{"stale plan", "datadog:monitor:123456", "stale", "run datadog plan again"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workDir, plan := datadogPlanForTest(t)
			path := filepath.Join(workDir, datadogplan.PlanRel)
			var err error
			switch tt.change {
			case "remove":
				err = os.Remove(path)
			case "malformed":
				err = os.WriteFile(path, []byte("{"), 0o644)
			case "stale":
				plan.Metadata.InventoryDigest = strings.Repeat("0", 64)
				err = datadogplan.WriteJSON(path, plan)
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := executeForTestWithOutput("datadog", "explain", tt.ref, "--workdir", workDir, "--json")
			if err == nil || !strings.Contains(err.Error(), tt.want) || out != "" {
				t.Fatalf("want %q and empty stdout, got err=%v stdout=%q", tt.want, err, out)
			}
		})
	}
}

func datadogPlanForTest(t *testing.T) (string, datadogplan.MigrationPlan) {
	t.Helper()
	workDir := t.TempDir()
	for _, args := range [][]string{
		{"datadog", "scan", "--fixture", "../../testdata/datadog/small.json", "--workdir", workDir},
		{"datadog", "plan", "--workdir", workDir},
	} {
		if err := executeForTest(args...); err != nil {
			t.Fatal(err)
		}
	}
	var plan datadogplan.MigrationPlan
	if err := datadogplan.ReadJSON(filepath.Join(workDir, datadogplan.PlanRel), &plan); err != nil {
		t.Fatal(err)
	}
	return workDir, plan
}
