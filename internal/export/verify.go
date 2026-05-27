package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const bundlePrefix = "openexit-evidence"

type VerifyOptions struct {
	BundlePath string
}

type VerificationReport struct {
	BundlePath      string           `json:"bundlePath"`
	Status          string           `json:"status"`
	Project         BundleProject    `json:"project"`
	Build           BundleBuild      `json:"build"`
	Validation      BundleValidation `json:"validation"`
	ManifestFiles   int              `json:"manifestFiles"`
	ChecksumEntries int              `json:"checksumEntries"`
	ArchiveFiles    int              `json:"archiveFiles"`
	Errors          []string         `json:"errors,omitempty"`
}

func Verify(opts VerifyOptions) (*VerificationReport, error) {
	report := &VerificationReport{BundlePath: opts.BundlePath, Status: "passed"}
	if strings.TrimSpace(opts.BundlePath) == "" {
		reportError(report, "--bundle is required")
		return report, verificationError(report)
	}
	reader, err := zip.OpenReader(opts.BundlePath)
	if err != nil {
		reportError(report, err.Error())
		return report, verificationError(report)
	}
	defer func() { _ = reader.Close() }()

	files := map[string][]byte{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		report.ArchiveFiles++
		if err := safeArchiveName(file.Name); err != nil {
			reportError(report, err.Error())
			continue
		}
		if _, exists := files[file.Name]; exists {
			reportError(report, "duplicate archive entry "+file.Name)
			continue
		}
		data, err := readZipFile(file)
		if err != nil {
			reportError(report, fmt.Sprintf("read %s: %v", file.Name, err))
			continue
		}
		files[file.Name] = data
	}

	if _, ok := files[bundlePrefix+"/README.md"]; !ok {
		reportError(report, "missing "+bundlePrefix+"/README.md")
	}
	manifest := verifyManifest(files, report)
	verifyChecksums(files, report)
	if manifest != nil {
		report.Project = manifest.Project
		report.Build = manifest.Build
		report.Validation = manifest.Validation
		report.ManifestFiles = len(manifest.Files)
	}
	if len(report.Errors) > 0 {
		report.Status = "failed"
		return report, verificationError(report)
	}
	return report, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func verifyManifest(files map[string][]byte, report *VerificationReport) *BundleManifest {
	data, ok := files[bundlePrefix+"/manifest.json"]
	if !ok {
		reportError(report, "missing "+bundlePrefix+"/manifest.json")
		return nil
	}
	if err := validateManifestSchema(data); err != nil {
		reportError(report, "manifest schema: "+err.Error())
		return nil
	}
	var manifest BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		reportError(report, "manifest json: "+err.Error())
		return nil
	}
	if _, err := time.Parse(time.RFC3339, manifest.GeneratedAt); err != nil {
		reportError(report, "manifest generatedAt is not RFC3339")
	}
	seen := map[string]struct{}{}
	for _, entry := range manifest.Files {
		if err := safeManifestPath(entry.Path); err != nil {
			reportError(report, "manifest path "+entry.Path+": "+err.Error())
			continue
		}
		if _, exists := seen[entry.Path]; exists {
			reportError(report, "duplicate manifest file entry "+entry.Path)
			continue
		}
		seen[entry.Path] = struct{}{}
		name := bundlePrefix + "/" + entry.Path
		data, ok := files[name]
		if !ok {
			reportError(report, "manifest references missing file "+entry.Path)
			continue
		}
		sum := sha256.Sum256(data)
		if digest := hex.EncodeToString(sum[:]); digest != entry.SHA256 {
			reportError(report, "manifest digest mismatch for "+entry.Path)
		}
		if int64(len(data)) != entry.Size {
			reportError(report, "manifest size mismatch for "+entry.Path)
		}
	}
	return &manifest
}

func verifyChecksums(files map[string][]byte, report *VerificationReport) {
	data, ok := files[bundlePrefix+"/checksums.txt"]
	if !ok {
		reportError(report, "missing "+bundlePrefix+"/checksums.txt")
		return
	}
	seen := map[string]struct{}{}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			reportError(report, "malformed checksum line "+line)
			continue
		}
		digest, rel := parts[0], parts[1]
		report.ChecksumEntries++
		if !isSHA256Hex(digest) {
			reportError(report, "invalid checksum digest for "+rel)
			continue
		}
		if err := safeManifestPath(rel); err != nil {
			reportError(report, "checksum path "+rel+": "+err.Error())
			continue
		}
		if _, exists := seen[rel]; exists {
			reportError(report, "duplicate checksum entry "+rel)
			continue
		}
		seen[rel] = struct{}{}
		fileData, ok := files[bundlePrefix+"/"+rel]
		if !ok {
			reportError(report, "checksum references missing file "+rel)
			continue
		}
		sum := sha256.Sum256(fileData)
		if hex.EncodeToString(sum[:]) != digest {
			reportError(report, "checksum digest mismatch for "+rel)
		}
	}
	for name := range files {
		rel, ok := strings.CutPrefix(name, bundlePrefix+"/")
		if !ok || rel == "checksums.txt" {
			continue
		}
		if _, exists := seen[rel]; !exists {
			reportError(report, "missing checksum entry for "+rel)
		}
	}
}

func validateManifestSchema(data []byte) error {
	schemaData, err := publicschemas.FS.ReadFile("openexit.evidence-bundle.schema.json")
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return err
	}
	if err := compiler.AddResource("openexit.evidence-bundle.schema.json", document); err != nil {
		return err
	}
	schema, err := compiler.Compile("openexit.evidence-bundle.schema.json")
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func safeArchiveName(name string) error {
	if !strings.HasPrefix(name, bundlePrefix+"/") {
		return fmt.Errorf("archive entry outside %s: %s", bundlePrefix, name)
	}
	rel := strings.TrimPrefix(name, bundlePrefix+"/")
	return safeManifestPath(rel)
}

func safeManifestPath(rel string) error {
	if rel == "" {
		return errors.New("empty path")
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return errors.New("absolute path not allowed")
	}
	cleaned := path.Clean(strings.ReplaceAll(rel, `\`, "/"))
	if cleaned == "." || cleaned != rel {
		return errors.New("unclean path not allowed")
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." {
			return errors.New("path traversal not allowed")
		}
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func reportError(report *VerificationReport, message string) {
	report.Status = "failed"
	report.Errors = append(report.Errors, message)
}

func verificationError(report *VerificationReport) error {
	if len(report.Errors) == 0 {
		return nil
	}
	errors := append([]string(nil), report.Errors...)
	sort.Strings(errors)
	return fmt.Errorf("bundle verification failed: %s", strings.Join(errors, "; "))
}
