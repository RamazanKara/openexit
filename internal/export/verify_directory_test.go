package export

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RamazanKara/openexit/internal/datadogplan"
)

func TestVerifyMigrationDirectory(t *testing.T) {
	directory := testMigrationDirectory(t)
	report, err := Verify(VerifyOptions{BundlePath: directory})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" || report.ManifestFiles == 0 || report.DirectoryFiles != report.ManifestFiles+2 || report.ChecksumEntries != report.DirectoryFiles-1 {
		t.Fatalf("unexpected directory verification report: %+v", report)
	}
	if report.ArchiveFiles != 0 || report.Build.Version != "test" {
		t.Fatalf("unexpected directory metadata: %+v", report)
	}
}

func TestVerifyRejectsAlteredMigrationDirectories(t *testing.T) {
	for _, tt := range []struct {
		name   string
		path   string
		data   string
		remove bool
		want   string
	}{
		{"tampered report", "index.html", "tampered", false, "manifest digest mismatch for index.html"},
		{"missing payload", "index.html", "", true, "manifest references missing file index.html"},
		{"extra payload", "review notes.txt", "unlisted", false, "missing manifest entry for review notes.txt"},
		{"missing manifest", "manifest.json", "", true, "read manifest.json"},
		{"invalid manifest", "manifest.json", "{", false, "manifest schema"},
		{"wrong manifest schema", "manifest.json", "{}", false, "manifest schema"},
		{"missing checksums", "SHA256SUMS", "", true, "read SHA256SUMS"},
		{"empty checksums", "SHA256SUMS", "", false, "missing checksum entry"},
		{"malformed checksums", "SHA256SUMS", "\ninvalid", false, "SHA256SUMS line 2"},
		{"checksum mismatch", "SHA256SUMS", strings.Repeat("0", 64) + "  index.html\n", false, "checksum digest mismatch for index.html"},
		{"missing checksum file", "SHA256SUMS", strings.Repeat("0", 64) + "  missing.txt\n", false, "checksum references missing file missing.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			directory := testMigrationDirectory(t)
			path := filepath.Join(directory, tt.path)
			var err error
			if tt.remove {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, []byte(tt.data), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			report, err := Verify(VerifyOptions{BundlePath: directory})
			if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got report=%+v err=%v", tt.want, report, err)
			}
		})
	}
}

func TestVerifyDirectoryManifestEntries(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*datadogplan.BundleManifest)
		want   string
	}{
		{"traversal", func(m *datadogplan.BundleManifest) { m.Files[0].Path = "../outside" }, "path traversal"},
		{"windows path", func(m *datadogplan.BundleManifest) { m.Files[0].Path = "C:/outside" }, "unsafe path"},
		{"backslash", func(m *datadogplan.BundleManifest) { m.Files[0].Path = `evidence\outside` }, "unclean path"},
		{"duplicate", func(m *datadogplan.BundleManifest) { m.Files = append(m.Files, m.Files[0]) }, "duplicate manifest file entry"},
		{"omitted file", func(m *datadogplan.BundleManifest) { m.Files = m.Files[1:] }, "missing manifest entry"},
		{"wrong size", func(m *datadogplan.BundleManifest) { m.Files[0].Size++ }, "manifest size mismatch"},
		{"self reference", func(m *datadogplan.BundleManifest) { m.Files[0].Path = "manifest.json" }, "must not list bundle metadata"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			directory := testMigrationDirectory(t)
			path := filepath.Join(directory, datadogplan.BundleManifestRel)
			var manifest datadogplan.BundleManifest
			if err := datadogplan.ReadJSON(path, &manifest); err != nil {
				t.Fatal(err)
			}
			tt.change(&manifest)
			if err := datadogplan.WriteJSON(path, &manifest); err != nil {
				t.Fatal(err)
			}
			report, err := Verify(VerifyOptions{BundlePath: directory})
			if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got report=%+v err=%v", tt.want, report, err)
			}
		})
	}
}

func TestVerifyDirectoryRejectsSymlinks(t *testing.T) {
	for _, rootLink := range []bool{false, true} {
		directory := testMigrationDirectory(t)
		link, target := filepath.Join(directory, "link"), filepath.Join(directory, "index.html")
		if rootLink {
			link, target = filepath.Join(t.TempDir(), "link"), directory
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if rootLink {
			directory = link
		}
		report, err := Verify(VerifyOptions{BundlePath: directory})
		if err == nil || report.Status != "failed" {
			t.Fatalf("accepted symlink: %+v", report)
		}
	}
}

func TestParseDirectoryChecksums(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, tt := range []struct {
		name string
		data string
		want string
		fail bool
	}{
		{"LF", digest + "  index.html\n", "index.html", false},
		{"CRLF", strings.ToUpper(digest) + "  review  notes.txt\r\n", "review  notes.txt", false},
		{"unicode", digest + "  review-ü.md", "review-ü.md", false},
		{"short hash", "aa  index.html", "", true},
		{"bad hash", strings.Repeat("z", 64) + "  index.html", "", true},
		{"single separator", digest + " index.html", "", true},
		{"traversal", digest + "  ../outside", "", true},
		{"absolute", digest + "  /outside", "", true},
		{"drive", digest + "  C:/outside", "", true},
		{"backslash", digest + `  a\b`, "", true},
		{"trailing dot", digest + "  file.", "", true},
		{"self", digest + "  SHA256SUMS", "", true},
		{"duplicate", digest + "  index.html\n" + digest + "  index.html", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDirectoryChecksums([]byte(tt.data))
			if (err != nil) != tt.fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && (len(got) != 1 || got[tt.want] != digest) {
				t.Fatalf("unexpected checksums: %v", got)
			}
		})
	}
}

func FuzzDirectoryChecksums(f *testing.F) {
	for _, seed := range []string{"", "invalid", strings.Repeat("a", 64) + "  index.html\n", strings.Repeat("0", 64) + "  review  notes.md\r\n", strings.Repeat("a", 64) + "  ../outside"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data string) {
		checksums, err := parseDirectoryChecksums([]byte(data))
		if err != nil {
			return
		}
		var canonical strings.Builder
		for name, digest := range checksums {
			if safeManifestPath(name) != nil || !isSHA256Hex(digest) || name == datadogplan.BundleChecksumsRel {
				t.Fatalf("accepted unsafe entry: %q %q", digest, name)
			}
			canonical.WriteString(digest + "  " + name + "\n")
		}
		again, err := parseDirectoryChecksums([]byte(canonical.String()))
		if err != nil || !reflect.DeepEqual(checksums, again) {
			t.Fatalf("checksum round trip failed: %v", err)
		}
	})
}

func FuzzDirectoryManifest(f *testing.F) {
	file := datadogplan.BundleFile{Path: "README.md", Size: 0, SHA256: datadogplan.DigestBytes(nil)}
	seed, err := json.Marshal(datadogplan.BundleManifest{
		APIVersion: datadogplan.APIVersion, Kind: datadogplan.BundleKind,
		PlanID: strings.Repeat("0", 64), InventoryDigest: strings.Repeat("0", 64),
		Build: datadogplan.BuildInfo{Version: "test", Commit: "test", Date: "unknown"},
		Files: []datadogplan.BundleFile{file},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("{}"))
	f.Add([]byte("{"))
	f.Fuzz(func(t *testing.T, data []byte) {
		report := &VerificationReport{Status: "passed"}
		verifyDirectoryManifest(data, map[string]datadogplan.BundleFile{file.Path: file}, report)
		if report.Status == "passed" && (!json.Valid(data) || report.ManifestFiles != 1 || len(report.Errors) != 0) {
			t.Fatalf("accepted invalid manifest: %+v", report)
		}
	})
}

func testMigrationDirectory(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	workDir := filepath.Join(root, "state")
	directory := filepath.Join(root, "migration notes ü")
	if _, err := datadogplan.Scan(context.Background(), datadogplan.ScanOptions{WorkDir: workDir, Fixture: "../../testdata/datadog/small.json"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := datadogplan.Plan(datadogplan.PlanOptions{WorkDir: workDir}); err != nil {
		t.Fatal(err)
	}
	if _, err := datadogplan.Export(datadogplan.ExportOptions{WorkDir: workDir, Out: directory, Version: "test"}); err != nil {
		t.Fatal(err)
	}
	return directory
}
