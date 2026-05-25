package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitProjectIdempotentAndStatus(t *testing.T) {
	dir := t.TempDir()
	if err := InitProject(dir); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "inventory", "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InitProject(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("init removed existing file: %v", err)
	}
	status, err := CheckProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.ConfigOK || len(status.Missing) != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInitProjectWithEndpoints(t *testing.T) {
	dir := t.TempDir()
	if err := InitProjectWithEndpoints(dir, "github-enterprise", "forgejo"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.Type != "github-enterprise" || cfg.Target.Type != "forgejo" {
		t.Fatalf("unexpected endpoints: source=%s target=%s", cfg.Source.Type, cfg.Target.Type)
	}
}

func TestInitProjectDefaultsTargetForSource(t *testing.T) {
	dir := t.TempDir()
	if err := InitProjectWithEndpoints(dir, "edge", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source.Type != "edge" || cfg.Target.Type != "varnish-haproxy-coraza" {
		t.Fatalf("unexpected endpoints: source=%s target=%s", cfg.Source.Type, cfg.Target.Type)
	}
}

func TestInitProjectRejectsUnsupportedPath(t *testing.T) {
	dir := t.TempDir()
	if err := InitProjectWithEndpoints(dir, "github-enterprise", "grafana-lgtm"); err == nil {
		t.Fatal("expected unsupported source/target pair to fail")
	}
}

func TestInvalidProjectConfig(t *testing.T) {
	dir := t.TempDir()
	if err := InitProject(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, projectFileName), []byte("kind: Wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckProject(dir); err == nil {
		t.Fatal("expected invalid project config error")
	}
}
