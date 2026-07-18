package datadogplan

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	BundleManifestRel  = "manifest.json"
	BundleChecksumsRel = "SHA256SUMS"
)

type ExportOptions struct {
	WorkDir      string
	Out          string
	Force        bool
	AllowPartial bool
	Version      string
	Commit       string
	Date         string
}

func Export(opts ExportOptions) (*BundleManifest, error) {
	if opts.WorkDir == "" {
		opts.WorkDir = DefaultWorkDir
	}
	if strings.TrimSpace(opts.Out) == "" {
		return nil, fmt.Errorf("export output directory is required")
	}
	out, err := safeExportTarget(opts.WorkDir, opts.Out)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(out); err == nil && !opts.Force {
		return nil, fmt.Errorf("export target already exists: %s (use --force to replace it)", out)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var inventory Inventory
	if err := ReadJSON(filepath.Join(opts.WorkDir, filepath.FromSlash(InventoryRel)), &inventory); err != nil {
		return nil, fmt.Errorf("read Datadog inventory: %w", err)
	}
	var plan MigrationPlan
	if err := ReadJSON(filepath.Join(opts.WorkDir, filepath.FromSlash(PlanRel)), &plan); err != nil {
		return nil, fmt.Errorf("read Datadog migration plan: %w", err)
	}
	var savedValidation ValidationReport
	if err := ReadJSON(filepath.Join(opts.WorkDir, filepath.FromSlash(ValidationRel)), &savedValidation); err != nil {
		return nil, fmt.Errorf("read Datadog validation report: %w", err)
	}
	if !inventory.Catalog.Complete && !opts.AllowPartial {
		return nil, &IncompleteScanError{Families: incompleteFamilies(inventory.Catalog)}
	}
	if savedValidation.Status != "passed" {
		return nil, fmt.Errorf("migration plan validation status is %q; regenerate a passing plan before export", savedValidation.Status)
	}
	if err := validateSchemaFile("openexit.datadog-validation.schema.json", filepath.Join(opts.WorkDir, filepath.FromSlash(ValidationRel))); err != nil {
		return nil, fmt.Errorf("saved validation report schema: %w", err)
	}
	validation := validateWorkspace(opts.WorkDir, opts.WorkDir, &inventory, &plan, opts.AllowPartial)
	if validation.Status != "passed" {
		return nil, fmt.Errorf("current migration workspace failed export validation: %s", failedValidationChecks(validation))
	}
	if err := validateSavedValidation(&savedValidation, validation); err != nil {
		return nil, err
	}
	if err := validateReadiness(&inventory, &plan, validation); err != nil {
		return nil, err
	}

	parent := filepath.Dir(out)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".openexit-export-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	for _, rel := range []string{"inventory", "evidence", "plan", "generated", "validation", ReportRel, BundleReadmeRel} {
		source := filepath.Join(opts.WorkDir, filepath.FromSlash(rel))
		if _, err := os.Lstat(source); err != nil {
			return nil, fmt.Errorf("required export content %s: %w", rel, err)
		}
		if err := copyExportPath(source, filepath.Join(stage, filepath.FromSlash(rel))); err != nil {
			return nil, fmt.Errorf("copy %s: %w", rel, err)
		}
	}
	if err := EnsureNoSymlinks(stage); err != nil {
		return nil, err
	}

	// Re-read and revalidate the staged bytes. This closes the gap between
	// validating the workspace and copying it if another process changes local
	// state concurrently.
	var copiedInventory Inventory
	if err := ReadJSON(filepath.Join(stage, filepath.FromSlash(InventoryRel)), &copiedInventory); err != nil {
		return nil, fmt.Errorf("read staged Datadog inventory: %w", err)
	}
	var copiedPlan MigrationPlan
	if err := ReadJSON(filepath.Join(stage, filepath.FromSlash(PlanRel)), &copiedPlan); err != nil {
		return nil, fmt.Errorf("read staged Datadog migration plan: %w", err)
	}
	var copiedSavedValidation ValidationReport
	if err := ReadJSON(filepath.Join(stage, filepath.FromSlash(ValidationRel)), &copiedSavedValidation); err != nil {
		return nil, fmt.Errorf("read staged Datadog validation report: %w", err)
	}
	if err := validateSchemaFile("openexit.datadog-validation.schema.json", filepath.Join(stage, filepath.FromSlash(ValidationRel))); err != nil {
		return nil, fmt.Errorf("staged validation report schema: %w", err)
	}
	copiedValidation := validateWorkspace(stage, stage, &copiedInventory, &copiedPlan, opts.AllowPartial)
	if copiedValidation.Status != "passed" {
		return nil, fmt.Errorf("staged migration bundle failed export validation: %s", failedValidationChecks(copiedValidation))
	}
	if err := validateSavedValidation(&copiedSavedValidation, copiedValidation); err != nil {
		return nil, fmt.Errorf("staged migration bundle: %w", err)
	}
	if err := validateReadiness(&copiedInventory, &copiedPlan, copiedValidation); err != nil {
		return nil, fmt.Errorf("staged migration bundle: %w", err)
	}
	inventory = copiedInventory
	plan = copiedPlan

	manifest := &BundleManifest{
		APIVersion:      APIVersion,
		Kind:            BundleKind,
		PlanID:          plan.Metadata.PlanID,
		InventoryDigest: inventory.Metadata.SnapshotDigest,
		Build: BuildInfo{
			Version: nonEmpty(opts.Version, "dev"),
			Commit:  nonEmpty(opts.Commit, "unknown"),
			Date:    nonEmpty(opts.Date, "unknown"),
		},
	}
	manifest.Files, err = bundleFiles(stage, &inventory, &plan)
	if err != nil {
		return nil, err
	}
	if err := WriteJSON(filepath.Join(stage, BundleManifestRel), manifest); err != nil {
		return nil, err
	}
	if err := validateSchemaFile("openexit.migration-bundle.schema.json", filepath.Join(stage, BundleManifestRel)); err != nil {
		return nil, fmt.Errorf("bundle manifest schema: %w", err)
	}
	if err := writeBundleChecksums(stage); err != nil {
		return nil, err
	}
	if err := EnsureNoSymlinks(stage); err != nil {
		return nil, err
	}
	if err := installExportDirectory(stage, out, opts.Force); err != nil {
		return nil, err
	}
	return manifest, nil
}

func safeExportTarget(workDir, out string) (string, error) {
	work, err := filepath.Abs(workDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	work = filepath.Clean(work)
	target = filepath.Clean(target)
	volumeRoot := filepath.Clean(filepath.VolumeName(target) + string(filepath.Separator))
	if target == volumeRoot {
		return "", fmt.Errorf("refusing to export over filesystem root %s", target)
	}
	resolvedWork, err := resolvePathForContainment(work)
	if err != nil {
		return "", fmt.Errorf("resolve OpenExit work directory: %w", err)
	}
	resolvedTarget, err := resolvePathForContainment(target)
	if err != nil {
		return "", fmt.Errorf("resolve export target: %w", err)
	}
	if pathContains(resolvedWork, resolvedTarget) || pathContains(resolvedTarget, resolvedWork) {
		return "", fmt.Errorf("export target and OpenExit work directory must not contain one another")
	}
	return target, nil
}

func resolvePathForContainment(path string) (string, error) {
	path = filepath.Clean(path)
	current := path
	var suffix []string
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing parent for %s", path)
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func copyExportPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink is not allowed: %s", source)
	}
	if !info.IsDir() {
		return copyExportFile(source, target)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyExportFile(path, destination)
	})
}

func copyExportFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func bundleFiles(stage string, inventory *Inventory, plan *MigrationPlan) ([]BundleFile, error) {
	refs := bundleSourceRefs(inventory, plan)
	var files []BundleFile
	err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err := SafeRelativePath(rel); err != nil {
			return fmt.Errorf("bundle file %s: %w", rel, err)
		}
		digest, size, err := DigestFile(path)
		if err != nil {
			return err
		}
		files = append(files, BundleFile{Path: rel, Size: size, SHA256: digest, SourceRefs: refs[rel]})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func bundleSourceRefs(inventory *Inventory, plan *MigrationPlan) map[string][]string {
	refs := map[string][]string{}
	for _, resource := range inventory.Resources {
		refs[resource.Evidence.Path] = append(refs[resource.Evidence.Path], resource.Ref)
	}
	for _, conversion := range plan.Resources {
		for _, output := range conversion.Outputs {
			refs[output.Path] = append(refs[output.Path], conversion.SourceRef)
		}
	}
	configurationRefs := configSourceRefs(inventory.Resources)
	for _, rel := range []string{"generated/alloy/config.alloy", "generated/opentelemetry/collector.yaml"} {
		refs[rel] = append(refs[rel], configurationRefs...)
	}
	for rel, values := range refs {
		refs[rel] = SortedUnique(values)
	}
	return refs
}

func writeBundleChecksums(stage string) error {
	var lines []string
	err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == BundleChecksumsRel {
			return nil
		}
		digest, _, err := DigestFile(path)
		if err != nil {
			return err
		}
		lines = append(lines, digest+"  "+rel)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(lines)
	return WriteText(filepath.Join(stage, BundleChecksumsRel), strings.Join(lines, "\n")+"\n")
}

func installExportDirectory(stage, target string, force bool) error {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return os.Rename(stage, target)
	} else if err != nil {
		return err
	}
	if !force {
		return fmt.Errorf("export target already exists: %s", target)
	}
	backup := target + ".previous-" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := os.Lstat(backup); err == nil {
		return fmt.Errorf("export backup path already exists: %s", backup)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("export installed, but previous target cleanup failed: %w", err)
	}
	return nil
}

func failedValidationChecks(report *ValidationReport) string {
	var failed []string
	for _, check := range report.Checks {
		if check.Status == "failed" {
			failed = append(failed, check.Name)
		}
	}
	if len(failed) == 0 {
		return "unknown validation failure"
	}
	return strings.Join(failed, ", ")
}

func validateSavedValidation(saved, current *ValidationReport) error {
	want, err := CanonicalDigest(current)
	if err != nil {
		return err
	}
	got, err := CanonicalDigest(saved)
	if err != nil {
		return err
	}
	if want != got {
		return fmt.Errorf("saved validation report does not match current deterministic validation")
	}
	return nil
}

func validateReadiness(inventory *Inventory, plan *MigrationPlan, validation *ValidationReport) error {
	expected := Score(inventory, plan.Resources, validation)
	want, err := CanonicalDigest(expected)
	if err != nil {
		return err
	}
	got, err := CanonicalDigest(plan.Readiness)
	if err != nil {
		return err
	}
	if want != got {
		return fmt.Errorf("plan readiness does not match the current inventory, conversions, and validation")
	}
	return nil
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
