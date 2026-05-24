package identity_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/identity"
	"github.com/RamazanKara/openexit/internal/evidence"
)

func TestFixtureCollector(t *testing.T) {
	projectDir := t.TempDir()
	if err := app.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}
	c := identity.FixtureCollector{}
	inv, err := c.Collect(context.Background(), collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    "identity-demo",
		Options: map[string]string{
			"input": filepath.Join("..", "..", "..", "testdata", "identity", "small.json"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Source.Type != "identity" {
		t.Fatalf("unexpected source: %s", inv.Source.Type)
	}
	if inv.Summary.IdentityApps != 3 || inv.Summary.IdentityGroups != 2 || inv.Summary.MFASettings != 1 {
		t.Fatalf("unexpected summary: %+v", inv.Summary)
	}
	path, err := evidence.PathForRef(projectDir, inv.Assets.IdentityApps[0].EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("application evidence ref did not resolve: %v", err)
	}
}
