package githubenterprise_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/githubenterprise"
	"github.com/RamazanKara/openexit/internal/evidence"
)

func TestFixtureCollector(t *testing.T) {
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := githubenterprise.FixtureCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "ghe-demo",
		Options: map[string]string{
			"input": filepath.Join("..", "..", "..", "testdata", "github-enterprise", "small.json"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Source.Type != "github-enterprise" {
		t.Fatalf("unexpected source: %s", inv.Source.Type)
	}
	if inv.Summary.Repositories != 2 || inv.Summary.ActionsWorkflows != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	path, err := evidence.PathForRef(projectDir, inv.Assets.Repositories[0].EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("repository evidence ref did not resolve: %v", err)
	}
}
