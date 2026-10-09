package export

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/datadogplan"
)

func verifyDirectory(directory string) (*VerificationReport, error) {
	report := &VerificationReport{BundlePath: directory, Status: "passed"}
	info, err := os.Lstat(filepath.Clean(directory))
	if err != nil {
		reportError(report, err.Error())
		return report, verificationError(report)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		reportError(report, "symlink bundle directory is not allowed")
		return report, verificationError(report)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		reportError(report, err.Error())
		return report, verificationError(report)
	}
	defer func() { _ = root.Close() }()
	files := map[string]datadogplan.BundleFile{}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name != "." {
			if err := safeManifestPath(name); err != nil {
				return fmt.Errorf("directory path %q: %w", name, err)
			}
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular directory entry %s", name)
		}
		file, err := root.Open(name)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		files[name] = datadogplan.BundleFile{Path: name, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}
		return nil
	})
	if err != nil {
		reportError(report, err.Error())
		return report, verificationError(report)
	}
	report.DirectoryFiles = len(files)
	for _, name := range []string{datadogplan.BundleReadmeRel, datadogplan.ReportRel, datadogplan.InventoryRel, datadogplan.PlanRel, datadogplan.ValidationRel} {
		if _, ok := files[name]; !ok {
			reportError(report, "missing required bundle file "+name)
		}
	}
	data, err := root.ReadFile(datadogplan.BundleManifestRel)
	if err != nil {
		reportError(report, "read manifest.json: "+err.Error())
	} else {
		verifyDirectoryManifest(data, files, report)
	}
	data, err = root.ReadFile(datadogplan.BundleChecksumsRel)
	if err != nil {
		reportError(report, "read SHA256SUMS: "+err.Error())
	} else {
		checksums, err := parseDirectoryChecksums(data)
		if err != nil {
			reportError(report, err.Error())
		} else {
			report.ChecksumEntries = len(checksums)
			for name, digest := range checksums {
				file, ok := files[name]
				if !ok {
					reportError(report, "checksum references missing file "+name)
				} else if file.SHA256 != digest {
					reportError(report, "checksum digest mismatch for "+name)
				}
			}
			for name := range files {
				if name == datadogplan.BundleChecksumsRel {
					continue
				}
				if _, ok := checksums[name]; !ok {
					reportError(report, "missing checksum entry for "+name)
				}
			}
		}
	}
	sort.Strings(report.Errors)
	return report, verificationError(report)
}

func verifyDirectoryManifest(data []byte, files map[string]datadogplan.BundleFile, report *VerificationReport) {
	if err := validateManifestSchema(data, "openexit.migration-bundle.schema.json"); err != nil {
		reportError(report, "manifest schema: "+err.Error())
		return
	}
	var manifest datadogplan.BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		reportError(report, "manifest json: "+err.Error())
		return
	}
	report.Build = BundleBuild{Version: manifest.Build.Version, Commit: manifest.Build.Commit, Date: manifest.Build.Date}
	report.ManifestFiles = len(manifest.Files)
	seen := map[string]bool{}
	for _, entry := range manifest.Files {
		if err := safeManifestPath(entry.Path); err != nil {
			reportError(report, "manifest path "+entry.Path+": "+err.Error())
			continue
		}
		if entry.Path == datadogplan.BundleManifestRel || entry.Path == datadogplan.BundleChecksumsRel {
			reportError(report, "manifest must not list bundle metadata "+entry.Path)
			continue
		}
		if seen[entry.Path] {
			reportError(report, "duplicate manifest file entry "+entry.Path)
			continue
		}
		seen[entry.Path] = true
		file, ok := files[entry.Path]
		if !ok {
			reportError(report, "manifest references missing file "+entry.Path)
			continue
		}
		if entry.SHA256 != file.SHA256 {
			reportError(report, "manifest digest mismatch for "+entry.Path)
		}
		if entry.Size != file.Size {
			reportError(report, "manifest size mismatch for "+entry.Path)
		}
	}
	for name := range files {
		if name != datadogplan.BundleManifestRel && name != datadogplan.BundleChecksumsRel && !seen[name] {
			reportError(report, "missing manifest entry for "+name)
		}
	}
}

func parseDirectoryChecksums(data []byte) (map[string]string, error) {
	checksums := map[string]string{}
	for index, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		digest, name, ok := strings.Cut(line, "  ")
		if !ok || !isSHA256Hex(digest) {
			return nil, fmt.Errorf("SHA256SUMS line %d: expected SHA-256, two spaces, and a relative path", index+1)
		}
		if err := safeManifestPath(name); err != nil {
			return nil, fmt.Errorf("SHA256SUMS line %d: %w", index+1, err)
		}
		if name == datadogplan.BundleChecksumsRel {
			return nil, fmt.Errorf("SHA256SUMS line %d: checksum file cannot list itself", index+1)
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("SHA256SUMS line %d: duplicate checksum entry %s", index+1, name)
		}
		checksums[name] = strings.ToLower(digest)
	}
	return checksums, nil
}
