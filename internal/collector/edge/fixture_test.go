package edge_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/edge"
	"github.com/RamazanKara/openexit/internal/evidence"
)

func TestFixtureCollector(t *testing.T) {
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := edge.FixtureCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "edge-demo",
		Options: map[string]string{
			"input": filepath.Join("..", "..", "..", "testdata", "edge", "small.json"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Source.Type != "edge" {
		t.Fatalf("unexpected source: %s", inv.Source.Type)
	}
	if inv.Summary.DNSRecords != 2 || inv.Summary.WAFRules != 2 || inv.Summary.CacheRules != 2 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	path, err := evidence.PathForRef(projectDir, inv.Assets.WAFRules[0].EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("WAF evidence ref did not resolve: %v", err)
	}
}
