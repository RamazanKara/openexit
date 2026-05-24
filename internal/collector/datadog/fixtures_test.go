package datadog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/app"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/datadog"
)

func TestFixtureCollectorProfiles(t *testing.T) {
	fixtures := []string{
		"small.json",
		"monitor-heavy.json",
		"dashboard-heavy.json",
		"unsupported-widgets.json",
		"high-cardinality.json",
		"prompt-injection.json",
		"risk-rules.json",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			projectDir := t.TempDir()
			if err := app.InitProject(projectDir); err != nil {
				t.Fatal(err)
			}
			c := datadog.FixtureCollector{}
			inv, err := c.Collect(context.Background(), collector.CollectRequest{
				ProjectDir: projectDir,
				Project:    "test",
				Source:     "datadog",
				Options: map[string]string{
					"input": filepath.Join("..", "..", "..", "testdata", "datadog", fixture),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if inv.Summary.Dashboards+inv.Summary.Monitors+inv.Summary.SLOs == 0 {
				t.Fatalf("expected assets for %s", fixture)
			}
			data, err := os.ReadFile(filepath.Join(projectDir, "inventory", "openexit.inventory.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(data) == "" {
				t.Fatal("empty inventory manifest")
			}
		})
	}
}
