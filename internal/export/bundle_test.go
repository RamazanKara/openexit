package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/openexit/internal/validate"
)

func TestBundleRoundTrip(t *testing.T) {
	for _, name := range []string{"notes.md", "review notes.md", "review  notes.md", "review-ü.md"} {
		t.Run(name, func(t *testing.T) {
			bundle := testBundle(t, name)
			report, err := Verify(VerifyOptions{BundlePath: bundle})
			if err != nil {
				t.Fatal(err)
			}
			if report.Status != "passed" || report.ManifestFiles != 2 || report.ArchiveFiles != 5 || report.ChecksumEntries != 4 {
				t.Fatalf("unexpected report: %+v", report)
			}
			if report.Project != (BundleProject{Name: "test-project", Source: "datadog", Target: "grafana-lgtm"}) {
				t.Fatalf("unexpected project: %+v", report.Project)
			}
			if report.Validation.Checks != 2 || report.Validation.Passed != 1 || report.Validation.Warnings != 1 {
				t.Fatalf("unexpected validation summary: %+v", report.Validation)
			}
		})
	}
}

func testBundle(t *testing.T, name string) string {
	t.Helper()
	project := t.TempDir()
	for rel, data := range map[string]string{
		"openexit.yaml":         "metadata:\n  name: test-project\nsource:\n  type: datadog\ntarget:\n  type: grafana-lgtm\n",
		"assessment/" + name:    "Candidate requires review.\n",
		"unrelated/ignored.txt": "not part of the bundle",
	} {
		path := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := bundleFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected only two payload files, got %v", files)
	}
	bundle := filepath.Join(t.TempDir(), "bundle.zip")
	report := &validate.Report{Status: "passed", Checks: []validate.Check{{Status: "passed"}, {Status: "warning"}}}
	if err := writeZip(project, bundle, files, report); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestVerifyRejectsInvalidBundles(t *testing.T) {
	tests := []struct {
		name   string
		entry  string
		data   []byte
		remove bool
		mode   os.FileMode
		want   string
	}{
		{name: "missing readme", entry: "README.md", remove: true, want: "missing openexit-evidence/README.md"},
		{name: "missing manifest", entry: "manifest.json", remove: true, want: "missing openexit-evidence/manifest.json"},
		{name: "missing checksums", entry: "checksums.txt", remove: true, want: "missing openexit-evidence/checksums.txt"},
		{name: "invalid manifest", entry: "manifest.json", data: []byte("{"), want: "manifest schema"},
		{name: "invalid schema", entry: "manifest.json", data: []byte("{}"), want: "manifest schema"},
		{name: "changed payload", entry: "openexit.yaml", data: []byte("tampered"), want: "manifest digest mismatch for openexit.yaml"},
		{name: "missing payload", entry: "openexit.yaml", remove: true, want: "manifest references missing file openexit.yaml"},
		{name: "unlisted payload", entry: "extra.txt", data: []byte("unlisted"), want: "missing manifest entry for extra.txt"},
		{name: "path traversal", entry: "../outside", data: []byte("unsafe"), want: "path traversal"},
		{name: "windows path", entry: "C:/outside", data: []byte("unsafe"), want: "unsafe path"},
		{name: "windows trailing dots", entry: "evidence/.../outside", data: []byte("unsafe"), want: "unsafe path"},
		{name: "windows trailing spaces", entry: "evidence/.. /outside", data: []byte("unsafe"), want: "unsafe path"},
		{name: "symlink", entry: "openexit.yaml", data: []byte("outside"), mode: os.ModeSymlink | 0o777, want: "non-regular archive entry"},
		{name: "symlink with directory suffix", entry: "evidence/link/", mode: os.ModeSymlink | 0o777, want: "non-regular archive entry"},
		{name: "malformed checksum", entry: "checksums.txt", data: []byte("invalid\n"), want: "malformed checksum line"},
		{name: "invalid digest", entry: "checksums.txt", data: []byte("invalid  openexit.yaml\n"), want: "invalid checksum digest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := testBundle(t, "notes.md")
			data, err := os.ReadFile(bundle)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			writer := zip.NewWriter(&output)
			name := bundlePrefix + "/" + tt.entry
			for _, file := range reader.File {
				if file.Name == name {
					continue
				}
				if err := writer.Copy(file); err != nil {
					t.Fatal(err)
				}
			}
			if !tt.remove {
				header := &zip.FileHeader{Name: name, Method: zip.Deflate}
				header.SetMode(tt.mode)
				entry, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write(tt.data); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(bundle, output.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			report, err := Verify(VerifyOptions{BundlePath: bundle})
			if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Verify = %+v, %v; want %q", report, err, tt.want)
			}
		})
	}
}

func TestVerifyManifestRequiresEveryPayloadFile(t *testing.T) {
	digest := sha256.Sum256(nil)
	manifest := buildManifest(t.TempDir(), time.Now().UTC().Format(time.RFC3339), nil,
		[]BundleManifestFile{{Path: "listed.txt", Size: 0, SHA256: hex.EncodeToString(digest[:])}})
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	report := &VerificationReport{Status: "passed"}
	verifyManifest(map[string][]byte{
		bundlePrefix + "/manifest.json": data,
		bundlePrefix + "/listed.txt":    nil,
		bundlePrefix + "/extra.txt":     []byte("not in the ledger"),
	}, report)
	if !strings.Contains(strings.Join(report.Errors, "\n"), "missing manifest entry for extra.txt") {
		t.Fatalf("unlisted payload accepted: %+v", report)
	}
}

func TestBundleRefusesInvalidProjectUnlessForced(t *testing.T) {
	project := t.TempDir()
	out := filepath.Join(t.TempDir(), "bundle.zip")
	for _, opts := range []Options{
		{ProjectDir: project, Out: out, Format: "tar"},
		{ProjectDir: project},
		{ProjectDir: project, Out: out},
	} {
		if err := Bundle(opts); err == nil {
			t.Fatalf("expected refusal for %+v", opts)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("refused export created output: %v", err)
		}
	}
	if err := Bundle(Options{ProjectDir: project, Out: out, Force: true}); err != nil {
		t.Fatal(err)
	}
	report, err := Verify(VerifyOptions{BundlePath: out})
	if err != nil || report.Validation.Status != "failed" {
		t.Fatalf("forced bundle should preserve failed validation: %+v, %v", report, err)
	}
}
