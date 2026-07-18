package datadogplan

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type PlanOptions struct {
	WorkDir      string
	Target       string
	AllowPartial bool
}

func Plan(opts PlanOptions) (*MigrationPlan, *ValidationReport, error) {
	if opts.WorkDir == "" {
		opts.WorkDir = DefaultWorkDir
	}
	if opts.Target == "" {
		opts.Target = DefaultTarget
	}
	if opts.Target != DefaultTarget {
		return nil, nil, fmt.Errorf("unsupported target %q; v0.1 supports only %s", opts.Target, DefaultTarget)
	}
	var inv Inventory
	if err := ReadJSON(filepath.Join(opts.WorkDir, filepath.FromSlash(InventoryRel)), &inv); err != nil {
		return nil, nil, fmt.Errorf("read Datadog inventory: %w", err)
	}
	if err := validateInventoryStructure(&inv); err != nil {
		return nil, nil, fmt.Errorf("datadog inventory structure: %w", err)
	}
	if err := validateInventoryDigest(&inv); err != nil {
		return nil, nil, err
	}
	if !inv.Catalog.Complete && !opts.AllowPartial {
		return nil, nil, &IncompleteScanError{Families: incompleteFamilies(inv.Catalog)}
	}
	stage, err := os.MkdirTemp(opts.WorkDir, ".plan-*")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	conversions, err := generateConversions(stage, &inv)
	if err != nil {
		return nil, nil, err
	}
	planID, err := planDigest(inv.Metadata.SnapshotDigest, opts.Target)
	if err != nil {
		return nil, nil, err
	}
	plan := &MigrationPlan{
		APIVersion: APIVersion,
		Kind:       PlanKind,
		Metadata: PlanMetadata{
			PlanID:          planID,
			InventoryDigest: inv.Metadata.SnapshotDigest,
			GeneratedAt:     inv.Metadata.CollectedAt,
			RulesetVersion:  RulesetVersion,
		},
		Target:    opts.Target,
		Summary:   summarizeConversions(conversions),
		Resources: conversions,
	}
	plan.Readiness = Score(&inv, conversions, nil)
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(PlanRel)), plan); err != nil {
		return nil, nil, err
	}
	preliminary := validateWorkspace(stage, opts.WorkDir, &inv, plan, opts.AllowPartial)
	plan.Readiness = Score(&inv, conversions, preliminary)
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(PlanRel)), plan); err != nil {
		return nil, nil, err
	}
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(ValidationRel)), preliminary); err != nil {
		return nil, nil, err
	}
	if err := renderReport(stage, &inv, plan, preliminary); err != nil {
		return nil, nil, err
	}
	finalReport := validateWorkspace(stage, opts.WorkDir, &inv, plan, opts.AllowPartial)
	plan.Readiness = Score(&inv, conversions, finalReport)
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(PlanRel)), plan); err != nil {
		return nil, nil, err
	}
	if err := WriteJSON(filepath.Join(stage, filepath.FromSlash(ValidationRel)), finalReport); err != nil {
		return nil, nil, err
	}
	if err := renderReport(stage, &inv, plan, finalReport); err != nil {
		return nil, nil, err
	}
	if err := validateSchemaFile("openexit.datadog-validation.schema.json", filepath.Join(stage, filepath.FromSlash(ValidationRel))); err != nil {
		return nil, nil, fmt.Errorf("validation report schema: %w", err)
	}
	if err := validateHTMLLinks(stage, opts.WorkDir); err != nil {
		return nil, nil, err
	}
	if err := replacePlanState(opts.WorkDir, stage); err != nil {
		return nil, nil, err
	}
	if finalReport.Status == "failed" {
		return plan, finalReport, fmt.Errorf("datadog migration plan validation failed")
	}
	return plan, finalReport, nil
}

func planDigest(inventoryDigest, target string) (string, error) {
	return CanonicalDigest(struct {
		InventoryDigest string `json:"inventoryDigest"`
		Target          string `json:"target"`
		Ruleset         string `json:"ruleset"`
	}{InventoryDigest: inventoryDigest, Target: target, Ruleset: RulesetVersion})
}

func summarizeConversions(conversions []Conversion) PlanSummary {
	summary := PlanSummary{Total: len(conversions)}
	outputs := map[string]struct{}{}
	for _, conversion := range conversions {
		switch conversion.Status {
		case StatusExact:
			summary.Exact++
		case StatusApproximate:
			summary.Approximate++
		case StatusManual:
			summary.Manual++
		case StatusUnsupported:
			summary.Unsupported++
		}
		for _, output := range conversion.Outputs {
			outputs[output.Path] = struct{}{}
		}
	}
	// The two target-baseline telemetry files are always generated.
	outputs["generated/alloy/config.alloy"] = struct{}{}
	outputs["generated/opentelemetry/collector.yaml"] = struct{}{}
	summary.OutputFiles = len(outputs)
	return summary
}

type stateMove struct {
	source string
	target string
	backup string
}

func replacePlanState(workDir, stage string) error {
	targets := []string{"generated", "plan", "validation", ReportRel, BundleReadmeRel}
	moves := make([]stateMove, 0, len(targets))
	stamp := time.Now().UTC().Format("20060102150405.000000000")
	for _, name := range targets {
		source := filepath.Join(stage, filepath.FromSlash(name))
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("planned output missing %s: %w", name, err)
		}
		target := filepath.Join(workDir, filepath.FromSlash(name))
		moves = append(moves, stateMove{source: source, target: target, backup: target + ".previous-" + stamp})
	}
	if err := backupStateMoves(moves); err != nil {
		return err
	}
	for index := range moves {
		if err := os.Rename(moves[index].source, moves[index].target); err != nil {
			for rollback := 0; rollback < index; rollback++ {
				_ = os.RemoveAll(moves[rollback].target)
			}
			restoreMoves(moves)
			return err
		}
	}
	for _, item := range moves {
		_ = os.RemoveAll(item.backup)
	}
	return nil
}

func backupStateMoves(moves []stateMove) error {
	for _, item := range moves {
		if _, err := os.Lstat(item.backup); err == nil {
			return fmt.Errorf("state backup path already exists: %s", item.backup)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for index, item := range moves {
		if _, err := os.Lstat(item.target); err == nil {
			if err := os.Rename(item.target, item.backup); err != nil {
				restoreMoves(moves[:index])
				return err
			}
		} else if !os.IsNotExist(err) {
			restoreMoves(moves[:index])
			return err
		}
	}
	return nil
}

func restoreMoves(moves []stateMove) {
	for _, item := range moves {
		if _, err := os.Lstat(item.backup); err == nil {
			_ = os.RemoveAll(item.target)
			_ = os.Rename(item.backup, item.target)
		}
	}
}
