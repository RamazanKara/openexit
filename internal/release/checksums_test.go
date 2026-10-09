package release

import (
	"strings"
	"testing"
)

func TestParseChecksumsModes(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, tt := range []struct {
		name string
		data string
		path string
		fail bool
	}{
		{"text", digest + "  openexit.exe\n", "openexit.exe", false},
		{"binary", digest + " *openexit.exe\n", "openexit.exe", false},
		{"binary CRLF", digest + " *openexit.exe\r\n", "openexit.exe", false},
		{"literal star in text mode", digest + "  *openexit\n", "*openexit", false},
		{"legacy tab separator", digest + "\topenexit.exe\n", "openexit.exe", false},
		{"mixed mode duplicate", digest + "  openexit.exe\n" + digest + " *openexit.exe\n", "", true},
		{"binary traversal", digest + " *../outside\n", "", true},
		{"empty binary path", digest + " *\n", "", true},
		{"bad digest", "invalid *openexit.exe\n", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := &VerificationReport{Status: "passed"}
			checksums := parseChecksums([]byte(tt.data), report)
			if (len(report.Errors) > 0) != tt.fail {
				t.Fatalf("unexpected errors: %v", report.Errors)
			}
			if !tt.fail && (len(checksums) != 1 || checksums[tt.path] != digest) {
				t.Fatalf("unexpected checksums: %v", checksums)
			}
		})
	}
}

func FuzzReleaseChecksums(f *testing.F) {
	for _, data := range []string{"", "invalid", strings.Repeat("a", 64) + "  openexit.exe\n", strings.Repeat("a", 64) + " *openexit.exe\r\n"} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data string) {
		report := &VerificationReport{Status: "passed"}
		for name, digest := range parseChecksums([]byte(data), report) {
			if safeArtifactPath(name) != nil || !isSHA256Hex(digest) {
				t.Fatalf("accepted invalid checksum: %q %q", name, digest)
			}
		}
		if (report.Status == "failed") != (len(report.Errors) > 0) {
			t.Fatalf("inconsistent parser status: %+v", report)
		}
	})
}
