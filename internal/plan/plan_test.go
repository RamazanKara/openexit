package plan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
)

func TestBuildDatadogMigrationPlanTracksMissingArtifacts(t *testing.T) {
	projectDir := t.TempDir()
	writePlanTestFiles(t, projectDir,
		"assessment/assessment.md",
		"assessment/risk-register.md",
		"assessment/manual-review.md",
		"assessment/cost-drivers.md",
		"assessment/target-architecture.md",
		"generated-config/grafana/README.md",
	)
	inv := inventory.New("demo", "datadog", "datadoghq.eu", "test", time.Unix(0, 0).UTC())
	a := assessment.New("demo", "datadog", "grafana-lgtm", time.Unix(0, 0).UTC(), nil, assessment.Score{Level: "medium"})
	p, err := Build(projectDir, inv, a, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("build migration plan: %v", err)
	}
	if p.Kind != Kind || len(p.Phases) != 4 {
		t.Fatalf("unexpected plan shape: %+v", p)
	}
	if p.Summary.Status != "incomplete" || p.Summary.MissingArtifacts == 0 {
		t.Fatalf("expected missing artifacts to mark plan incomplete: %+v", p.Summary)
	}
	shadow := phaseByName(p, "shadow")
	if shadow.Status != "incomplete" || len(shadow.MissingArtifacts) == 0 {
		t.Fatalf("expected shadow phase to report missing artifacts: %+v", shadow)
	}
}

func TestWriteMigrationPlanArtifacts(t *testing.T) {
	projectDir := t.TempDir()
	inv := inventory.New("demo", "edge", "cloudflare/example.com", "test", time.Unix(0, 0).UTC())
	a := assessment.New("demo", "edge", "varnish-haproxy-coraza", time.Unix(0, 0).UTC(), nil, assessment.Score{Level: "medium"})
	p, err := Build(projectDir, inv, a, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("build migration plan: %v", err)
	}
	if err := Write(projectDir, p); err != nil {
		t.Fatalf("write migration plan: %v", err)
	}
	for _, rel := range []string{
		"assessment/openexit.migration-plan.yaml",
		"assessment/openexit.migration-plan.json",
		"assessment/migration-plan.md",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
}

func writePlanTestFiles(t *testing.T, projectDir string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		path := filepath.Join(projectDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func phaseByName(p *Plan, name string) Phase {
	for _, phase := range p.Phases {
		if phase.Name == name {
			return phase
		}
	}
	return Phase{}
}
