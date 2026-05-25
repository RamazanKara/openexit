package aiprovider_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/aiprovider"
	"github.com/RamazanKara/openexit/internal/evidence"
)

func TestFixtureCollector(t *testing.T) {
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := aiprovider.FixtureCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "ai-demo",
		Options: map[string]string{
			"input": filepath.Join("..", "..", "..", "testdata", "ai-provider", "small.json"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Source.Type != "ai-provider" {
		t.Fatalf("unexpected source: %s", inv.Source.Type)
	}
	if inv.Summary.AIModelUsageClasses != 3 || inv.Summary.AITokenVolumes != 3 || inv.Summary.AIToolUsages != 3 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	path, err := evidence.PathForRef(projectDir, inv.Assets.AIModelUsageClasses[0].EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("AI model usage evidence ref did not resolve: %v", err)
	}
}
