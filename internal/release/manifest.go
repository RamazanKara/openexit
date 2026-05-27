package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	APIVersion       = "openexit.dev/v1alpha1"
	Kind             = "ReleaseManifest"
	DefaultManifest  = "RELEASE_MANIFEST.json"
	ChecksumFileName = "SHA256SUMS"
	ArtifactBinary   = "binary"
	ArtifactAsset    = "asset"
)

var DefaultPlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"windows/amd64",
}

type Manifest struct {
	APIVersion  string     `json:"apiVersion"`
	Kind        string     `json:"kind"`
	GeneratedAt string     `json:"generatedAt"`
	Build       Build      `json:"build"`
	Artifacts   []Artifact `json:"artifacts"`
}

type Build struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type Artifact struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	OS     string `json:"os,omitempty"`
	Arch   string `json:"arch,omitempty"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type ManifestOptions struct {
	DistDir     string
	Version     string
	Commit      string
	Date        string
	Platforms   []string
	Assets      []string
	GeneratedAt time.Time
}

type VerifyOptions struct {
	ManifestPath     string
	DistDir          string
	RequireChecksums bool
	Artifacts        []string
}

type VerificationReport struct {
	ManifestPath    string                 `json:"manifestPath"`
	DistDir         string                 `json:"distDir"`
	Status          string                 `json:"status"`
	Build           Build                  `json:"build"`
	GeneratedAt     string                 `json:"generatedAt"`
	Artifacts       []ArtifactVerification `json:"artifacts"`
	ChecksumEntries int                    `json:"checksumEntries"`
	Errors          []string               `json:"errors,omitempty"`
}

type ArtifactVerification struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	OS     string   `json:"os"`
	Arch   string   `json:"arch"`
	Path   string   `json:"path"`
	Size   int64    `json:"size"`
	SHA256 string   `json:"sha256"`
	Status string   `json:"status"`
	Errors []string `json:"errors,omitempty"`
}

func ArtifactName(version, osName, arch string) string {
	ext := ""
	if osName == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("openexit_%s_%s_%s%s", version, osName, arch, ext)
}

func Generate(opts ManifestOptions) (*Manifest, error) {
	distDir := opts.DistDir
	if strings.TrimSpace(distDir) == "" {
		distDir = "dist"
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		return nil, errors.New("version is required")
	}
	platforms := opts.Platforms
	if len(platforms) == 0 {
		platforms = append([]string(nil), DefaultPlatforms...)
	}
	generatedAt := opts.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	manifest := &Manifest{
		APIVersion:  APIVersion,
		Kind:        Kind,
		GeneratedAt: generatedAt.Format(time.RFC3339),
		Build: Build{
			Version: version,
			Commit:  nonEmpty(opts.Commit, "unknown"),
			Date:    nonEmpty(opts.Date, "unknown"),
		},
	}
	for _, platform := range platforms {
		osName, arch, err := splitPlatform(platform)
		if err != nil {
			return nil, err
		}
		name := ArtifactName(version, osName, arch)
		info, digest, err := fileDigest(filepath.Join(distDir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		manifest.Artifacts = append(manifest.Artifacts, Artifact{
			Name:   name,
			Type:   ArtifactBinary,
			OS:     osName,
			Arch:   arch,
			Path:   name,
			Size:   info.Size(),
			SHA256: digest,
		})
	}
	for _, asset := range opts.Assets {
		asset = strings.TrimSpace(asset)
		if asset == "" {
			return nil, errors.New("asset path is required")
		}
		if err := safeArtifactPath(asset); err != nil {
			return nil, fmt.Errorf("asset %s: %w", asset, err)
		}
		info, digest, err := fileDigest(filepath.Join(distDir, filepath.FromSlash(asset)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", asset, err)
		}
		manifest.Artifacts = append(manifest.Artifacts, Artifact{
			Name:   path.Base(asset),
			Type:   ArtifactAsset,
			Path:   asset,
			Size:   info.Size(),
			SHA256: digest,
		})
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path
	})
	return manifest, nil
}

func Write(path string, manifest *Manifest) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("manifest output path is required")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Verify(opts VerifyOptions) (*VerificationReport, error) {
	report := &VerificationReport{ManifestPath: opts.ManifestPath, DistDir: opts.DistDir, Status: "passed"}
	if strings.TrimSpace(opts.ManifestPath) == "" {
		reportError(report, "manifest path is required")
		return report, verificationError(report)
	}
	data, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
		reportError(report, err.Error())
		return report, verificationError(report)
	}
	if err := ValidateManifestSchema(data); err != nil {
		reportError(report, "manifest schema: "+err.Error())
		return report, verificationError(report)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		reportError(report, "manifest json: "+err.Error())
		return report, verificationError(report)
	}
	if _, err := time.Parse(time.RFC3339, manifest.GeneratedAt); err != nil {
		reportError(report, "manifest generatedAt is not RFC3339")
	}
	distDir := opts.DistDir
	if strings.TrimSpace(distDir) == "" {
		distDir = filepath.Dir(opts.ManifestPath)
	}
	report.DistDir = distDir
	report.Build = manifest.Build
	report.GeneratedAt = manifest.GeneratedAt

	seen := map[string]struct{}{}
	filters := artifactFilters(opts.Artifacts, report)
	matchedFilters := map[string]struct{}{}
	checksums := readChecksums(filepath.Join(distDir, ChecksumFileName), report, opts.RequireChecksums)
	for _, artifact := range manifest.Artifacts {
		if len(filters) > 0 && !artifactSelected(artifact, filters, matchedFilters) {
			continue
		}
		result := ArtifactVerification{
			Name:   artifact.Name,
			Type:   artifact.Type,
			OS:     artifact.OS,
			Arch:   artifact.Arch,
			Path:   artifact.Path,
			Size:   artifact.Size,
			SHA256: artifact.SHA256,
			Status: "passed",
		}
		if err := safeArtifactPath(artifact.Path); err != nil {
			resultError(&result, "path: "+err.Error())
		}
		if artifact.Name == "" {
			resultError(&result, "name is required")
		}
		if artifact.Type == "" {
			resultError(&result, "type is required")
		}
		if artifact.Type == ArtifactBinary && (artifact.OS == "" || artifact.Arch == "") {
			resultError(&result, "binary artifact requires os and arch")
		}
		if _, exists := seen[artifact.Path]; exists {
			resultError(&result, "duplicate artifact path")
		}
		seen[artifact.Path] = struct{}{}
		actualDigest := ""
		if result.Status == "passed" {
			info, digest, err := fileDigest(filepath.Join(distDir, filepath.FromSlash(artifact.Path)))
			if err != nil {
				resultError(&result, err.Error())
			} else {
				actualDigest = digest
				if info.Size() != artifact.Size {
					resultError(&result, "size mismatch")
				}
				if digest != artifact.SHA256 {
					resultError(&result, "sha256 mismatch")
				}
			}
		}
		if checksums != nil {
			digest, ok := checksums[artifact.Path]
			if !ok {
				resultError(&result, "missing SHA256SUMS entry")
			} else if digest != artifact.SHA256 {
				resultError(&result, "SHA256SUMS digest mismatch")
			} else if actualDigest != "" && digest != actualDigest {
				resultError(&result, "SHA256SUMS digest mismatch")
			}
		}
		if result.Status != "passed" {
			for _, message := range result.Errors {
				reportError(report, artifact.Path+": "+message)
			}
		}
		report.Artifacts = append(report.Artifacts, result)
	}
	if len(filters) > 0 {
		for filter := range filters {
			if _, ok := matchedFilters[filter]; !ok {
				reportError(report, "manifest has no artifact "+filter)
			}
		}
	}
	if len(report.Artifacts) == 0 {
		reportError(report, "manifest has no artifacts to verify")
	}
	if checksums != nil && len(filters) == 0 {
		for rel := range checksums {
			if _, ok := seen[rel]; !ok {
				reportError(report, "SHA256SUMS entry not present in manifest "+rel)
			}
		}
	}
	return finishReport(report)
}

func ValidateManifestSchema(data []byte) error {
	schemaData, err := publicschemas.FS.ReadFile("openexit.release-manifest.schema.json")
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return err
	}
	if err := compiler.AddResource("openexit.release-manifest.schema.json", document); err != nil {
		return err
	}
	schema, err := compiler.Compile("openexit.release-manifest.schema.json")
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func splitPlatform(platform string) (string, string, error) {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("platform must be os/arch: %s", platform)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func fileDigest(path string) (os.FileInfo, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, "", err
	}
	return info, hex.EncodeToString(hash.Sum(nil)), nil
}

func readChecksums(path string, report *VerificationReport, require bool) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		if require {
			reportError(report, ChecksumFileName+": "+err.Error())
		}
		return nil
	}
	checksums := map[string]string{}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			reportError(report, "malformed SHA256SUMS line "+line)
			continue
		}
		digest, rel := parts[0], parts[1]
		report.ChecksumEntries++
		if !isSHA256Hex(digest) {
			reportError(report, "invalid SHA256SUMS digest for "+rel)
			continue
		}
		if err := safeArtifactPath(rel); err != nil {
			reportError(report, "SHA256SUMS path "+rel+": "+err.Error())
			continue
		}
		if _, exists := checksums[rel]; exists {
			reportError(report, "duplicate SHA256SUMS entry "+rel)
			continue
		}
		checksums[rel] = digest
	}
	return checksums
}

func artifactFilters(artifacts []string, report *VerificationReport) map[string]struct{} {
	if len(artifacts) == 0 {
		return nil
	}
	filters := map[string]struct{}{}
	for _, artifact := range artifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact == "" {
			reportError(report, "empty artifact filter")
			continue
		}
		if err := safeArtifactPath(artifact); err != nil {
			reportError(report, "artifact filter "+artifact+": "+err.Error())
			continue
		}
		filters[artifact] = struct{}{}
	}
	return filters
}

func artifactSelected(artifact Artifact, filters, matched map[string]struct{}) bool {
	selected := false
	for _, candidate := range []string{artifact.Path, artifact.Name} {
		if _, ok := filters[candidate]; ok {
			matched[candidate] = struct{}{}
			selected = true
		}
	}
	return selected
}

func safeArtifactPath(rel string) error {
	if rel == "" {
		return errors.New("empty path")
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return errors.New("absolute path not allowed")
	}
	if strings.Contains(rel, `\`) {
		return errors.New("backslash path not allowed")
	}
	cleaned := path.Clean(rel)
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

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func resultError(result *ArtifactVerification, message string) {
	result.Status = "failed"
	result.Errors = append(result.Errors, message)
}

func reportError(report *VerificationReport, message string) {
	report.Status = "failed"
	report.Errors = append(report.Errors, message)
}

func finishReport(report *VerificationReport) (*VerificationReport, error) {
	if len(report.Errors) == 0 {
		return report, nil
	}
	sort.Strings(report.Errors)
	return report, verificationError(report)
}

func verificationError(report *VerificationReport) error {
	if len(report.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("release verification failed: %s", strings.Join(report.Errors, "; "))
}
