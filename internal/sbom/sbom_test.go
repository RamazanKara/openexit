package sbom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"
)

func TestFromBuildInfoProducesCycloneDXSBOM(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.25.10",
		Main: debug.Module{
			Path:    "github.com/RamazanKara/openexit",
			Version: "(devel)",
		},
		Deps: []*debug.Module{
			{Path: "github.com/spf13/cobra", Version: "v1.10.2", Sum: "h1:cobra"},
			{Path: "github.com/santhosh-tekuri/jsonschema/v6", Version: "v6.0.2", Sum: "h1:jsonschema"},
		},
	}
	bom := FromBuildInfo(info, Options{
		Name:      "openexit",
		Version:   "1.2.3",
		Commit:    "abc123",
		Date:      "2026-05-27T00:00:00Z",
		Timestamp: time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
	})

	if bom.BOMFormat != BOMFormat || bom.SpecVersion != SpecVersion || bom.Metadata.Component.Name != "openexit" {
		t.Fatalf("unexpected BOM metadata: %+v", bom)
	}
	if bom.Metadata.Component.Version != "1.2.3" || len(bom.Components) != 2 {
		t.Fatalf("unexpected BOM component summary: %+v", bom)
	}
	if bom.Components[0].Name != "github.com/santhosh-tekuri/jsonschema/v6" || bom.Components[1].Name != "github.com/spf13/cobra" {
		t.Fatalf("components should be sorted by module path: %+v", bom.Components)
	}
	data, err := json.Marshal(bom)
	if err != nil {
		t.Fatalf("marshal BOM: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("BOM is not valid JSON: %s", string(data))
	}
}

func TestWriteSBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "SBOM.cdx.json")
	bom := &BOM{BOMFormat: BOMFormat, SpecVersion: SpecVersion, Version: 1}
	if err := Write(path, bom); err != nil {
		t.Fatalf("write SBOM: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("written SBOM is not valid JSON: %s", string(data))
	}
}
