package datadogplan

import (
	"fmt"
	"path/filepath"
)

func Explain(workDir, sourceRef string) (*Conversion, error) {
	if workDir == "" {
		workDir = DefaultWorkDir
	}
	var inv Inventory
	if err := ReadJSON(filepath.Join(workDir, filepath.FromSlash(InventoryRel)), &inv); err != nil {
		return nil, fmt.Errorf("read Datadog inventory (run datadog scan first): %w", err)
	}
	var plan MigrationPlan
	planPath := filepath.Join(workDir, filepath.FromSlash(PlanRel))
	if err := ReadJSON(planPath, &plan); err != nil {
		return nil, fmt.Errorf("read Datadog migration plan (run datadog plan first): %w", err)
	}
	if err := validateSchemaFile("openexit.datadog-plan.schema.json", planPath); err != nil {
		return nil, fmt.Errorf("plan schema: %w", err)
	}
	if err := validateInventoryDigest(&inv); err != nil {
		return nil, err
	}
	if err := validatePlanIdentity(&inv, &plan); err != nil {
		return nil, fmt.Errorf("run datadog plan again: %w", err)
	}
	if err := validateConversionCoverage(&inv, &plan); err != nil {
		return nil, err
	}
	for _, conversion := range plan.Resources {
		if conversion.SourceRef == sourceRef {
			return &conversion, nil
		}
	}
	return nil, fmt.Errorf("resource %q not found in the plan; use a full sourceRef from %s", sourceRef, PlanRel)
}
