package release

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateWriteAndVerifyManifest(t *testing.T) {
	distDir := t.TempDir()
	version := "1.2.3"
	platforms := []string{"linux/amd64", "windows/amd64"}
	writeArtifact(t, distDir, ArtifactName(version, "linux", "amd64"), "linux binary")
	writeArtifact(t, distDir, ArtifactName(version, "windows", "amd64"), "windows binary")

	manifest, err := Generate(ManifestOptions{
		DistDir:     distDir,
		Version:     version,
		Commit:      "abc123",
		Date:        "2026-05-27T00:00:00Z",
		Platforms:   platforms,
		GeneratedAt: time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("generate manifest: %v", err)
	}
	if len(manifest.Artifacts) != 2 || manifest.Build.Version != version {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	manifestPath := filepath.Join(distDir, DefaultManifest)
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeSHA256SUMS(distDir, manifest.Artifacts); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	report, err := Verify(VerifyOptions{ManifestPath: manifestPath, DistDir: distDir, RequireChecksums: true})
	if err != nil {
		t.Fatalf("verify release: %v", err)
	}
	if report.Status != "passed" || len(report.Artifacts) != 2 || report.ChecksumEntries != 2 {
		t.Fatalf("unexpected verification report: %+v", report)
	}
}

func TestVerifyManifestRejectsTamperedArtifact(t *testing.T) {
	distDir := t.TempDir()
	version := "1.2.3"
	name := ArtifactName(version, "linux", "amd64")
	writeArtifact(t, distDir, name, "original binary")
	manifest, err := Generate(ManifestOptions{
		DistDir:   distDir,
		Version:   version,
		Commit:    "abc123",
		Date:      "2026-05-27T00:00:00Z",
		Platforms: []string{"linux/amd64"},
	})
	if err != nil {
		t.Fatalf("generate manifest: %v", err)
	}
	manifestPath := filepath.Join(distDir, DefaultManifest)
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeSHA256SUMS(distDir, manifest.Artifacts); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	writeArtifact(t, distDir, name, "tampered binary")

	report, err := Verify(VerifyOptions{ManifestPath: manifestPath, DistDir: distDir, RequireChecksums: true})
	if err == nil {
		t.Fatalf("expected tampering to fail, got report %+v", report)
	}
	joined := strings.Join(report.Errors, "\n")
	for _, marker := range []string{"sha256 mismatch", "SHA256SUMS digest mismatch"} {
		if !strings.Contains(joined, marker) {
			t.Fatalf("expected marker %q in errors: %s", marker, joined)
		}
	}
}

func TestVerifyManifestCanCheckSingleArtifactSubset(t *testing.T) {
	distDir := t.TempDir()
	version := "1.2.3"
	linux := ArtifactName(version, "linux", "amd64")
	windows := ArtifactName(version, "windows", "amd64")
	writeArtifact(t, distDir, linux, "linux binary")
	writeArtifact(t, distDir, windows, "windows binary")
	manifest, err := Generate(ManifestOptions{
		DistDir:   distDir,
		Version:   version,
		Commit:    "abc123",
		Date:      "2026-05-27T00:00:00Z",
		Platforms: []string{"linux/amd64", "windows/amd64"},
	})
	if err != nil {
		t.Fatalf("generate manifest: %v", err)
	}
	manifestPath := filepath.Join(distDir, DefaultManifest)
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeSHA256SUMS(distDir, manifest.Artifacts); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	if err := os.Remove(filepath.Join(distDir, windows)); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(VerifyOptions{
		ManifestPath:     manifestPath,
		DistDir:          distDir,
		RequireChecksums: true,
		Artifacts:        []string{linux},
	})
	if err != nil {
		t.Fatalf("verify filtered release: %v", err)
	}
	if report.Status != "passed" || len(report.Artifacts) != 1 || report.Artifacts[0].Path != linux {
		t.Fatalf("unexpected filtered verification report: %+v", report)
	}

	report, err = Verify(VerifyOptions{ManifestPath: manifestPath, DistDir: distDir, RequireChecksums: true})
	if err == nil {
		t.Fatalf("expected full verification to fail after removing artifact, got %+v", report)
	}
	if !strings.Contains(strings.Join(report.Errors, "\n"), windows) {
		t.Fatalf("expected missing windows artifact error, got %+v", report.Errors)
	}
}

func writeArtifact(t *testing.T, distDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(distDir, name), []byte(content), 0o755); err != nil {
		t.Fatalf("write artifact %s: %v", name, err)
	}
}

func writeSHA256SUMS(distDir string, artifacts []Artifact) error {
	var builder strings.Builder
	for _, artifact := range artifacts {
		data, err := os.ReadFile(filepath.Join(distDir, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		builder.WriteString(hex.EncodeToString(sum[:]))
		builder.WriteString("  ")
		builder.WriteString(artifact.Path)
		builder.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(distDir, ChecksumFileName), []byte(builder.String()), 0o644)
}
