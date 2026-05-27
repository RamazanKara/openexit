package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaValidatorAcceptsYAMLAndRejectsMissingRequiredFields(t *testing.T) {
	validator, err := newSchemaValidator()
	if err != nil {
		t.Fatalf("schema validator: %v", err)
	}
	dir := t.TempDir()
	validPath := filepath.Join(dir, "openexit.yaml")
	if err := os.WriteFile(validPath, []byte(`apiVersion: openexit.dev/v1alpha1
kind: Project
metadata:
  name: demo
source:
  type: datadog
target:
  type: grafana-lgtm
policy:
  allowNetworkWrites: false
  allowProductionWrites: false
  allowAI: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateFile("openexit.project.schema.json", validPath); err != nil {
		t.Fatalf("valid project schema failed: %v", err)
	}

	invalidPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(`apiVersion: openexit.dev/v1alpha1
kind: Project
metadata:
  name: demo
source:
  type: datadog
target:
  type: grafana-lgtm
policy:
  allowNetworkWrites: false
  allowProductionWrites: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	err = validator.ValidateFile("openexit.project.schema.json", invalidPath)
	if err == nil {
		t.Fatal("expected missing policy.allowAI to fail schema validation")
	}
	message := formatSchemaError(err)
	if !strings.Contains(message, "allowAI") {
		t.Fatalf("expected useful missing field message, got %q", message)
	}
}
