package app

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/RamazanKara/openexit/internal/datadogplan"
	"github.com/RamazanKara/openexit/internal/version"
	"github.com/spf13/cobra"
)

func newDatadogWorkflowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datadog",
		Short: "Plan a read-only Datadog to Grafana LGTM migration",
		Long:  "Scan Datadog with read-only API requests, generate deterministic Grafana, Prometheus, Alloy, and OpenTelemetry candidates, and export a reviewable migration bundle.",
	}
	cmd.AddCommand(newDatadogScanCommand())
	cmd.AddCommand(newDatadogPlanCommand())
	cmd.AddCommand(newDatadogExportCommand())
	cmd.AddCommand(newDatadogExplainCommand())
	return cmd
}

func newDatadogScanCommand() *cobra.Command {
	var workDir, site, apiKeyEnv, appKeyEnv, fixture, baseURL string
	var allowPartial, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Inventory Datadog resources using GET-only API requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			inventory, err := datadogplan.Scan(cmd.Context(), datadogplan.ScanOptions{
				WorkDir:      workDir,
				Site:         site,
				APIKeyEnv:    apiKeyEnv,
				AppKeyEnv:    appKeyEnv,
				Fixture:      fixture,
				BaseURL:      baseURL,
				Version:      version.Version,
				AllowPartial: allowPartial,
			})
			if inventory != nil {
				if jsonOutput {
					if outputErr := writeDatadogJSON(cmd.OutOrStdout(), inventory); outputErr != nil {
						return outputErr
					}
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "inventory: %s\nresources: %d\ncatalog-complete: %t\ndigest: %s\n",
					filepath.Join(workDir, filepath.FromSlash(datadogplan.InventoryRel)), len(inventory.Resources), inventory.Catalog.Complete, inventory.Metadata.SnapshotDigest)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", datadogplan.DefaultWorkDir, "OpenExit Datadog state directory")
	cmd.Flags().StringVar(&site, "site", "datadoghq.com", "Datadog site, such as datadoghq.com or datadoghq.eu")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "DATADOG_API_KEY", "Environment variable containing the read-only Datadog API key")
	cmd.Flags().StringVar(&appKeyEnv, "app-key-env", "DATADOG_APP_KEY", "Environment variable containing the read-only Datadog application key")
	cmd.Flags().StringVar(&fixture, "fixture", "", "Read a local Datadog fixture instead of calling the API")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Persist and accept an incomplete catalog scan")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write the inventory as JSON")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Override the Datadog API base URL")
	_ = cmd.Flags().MarkHidden("base-url")
	return cmd
}

func newDatadogPlanCommand() *cobra.Command {
	var workDir, target string
	var allowPartial, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate deterministic migration candidates and a static report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, validation, err := datadogplan.Plan(datadogplan.PlanOptions{
				WorkDir:      workDir,
				Target:       target,
				AllowPartial: allowPartial,
			})
			if plan != nil {
				if jsonOutput {
					if outputErr := writeDatadogJSON(cmd.OutOrStdout(), plan); outputErr != nil {
						return outputErr
					}
					return err
				}
				validationStatus := "unknown"
				if validation != nil {
					validationStatus = validation.Status
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "plan: %s\nreport: %s\nexit-readiness: %d/100 (%s)\nvalidation: %s\n",
					filepath.Join(workDir, filepath.FromSlash(datadogplan.PlanRel)), filepath.Join(workDir, datadogplan.ReportRel),
					plan.Readiness.Score, plan.Readiness.Level, validationStatus)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", datadogplan.DefaultWorkDir, "OpenExit Datadog state directory")
	cmd.Flags().StringVar(&target, "target", datadogplan.DefaultTarget, "Migration target (v0.1 supports grafana-lgtm)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Plan from a scan explicitly accepted as incomplete")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write the migration plan as JSON")
	return cmd
}

func newDatadogExportCommand() *cobra.Command {
	var workDir, out string
	var force, allowPartial, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a validated, reviewable migration directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := datadogplan.Export(datadogplan.ExportOptions{
				WorkDir:      workDir,
				Out:          out,
				Force:        force,
				AllowPartial: allowPartial,
				Version:      version.Version,
				Commit:       version.Commit,
				Date:         version.Date,
			})
			if manifest != nil {
				if jsonOutput {
					if outputErr := writeDatadogJSON(cmd.OutOrStdout(), manifest); outputErr != nil {
						return outputErr
					}
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "exported: %s\nplan-id: %s\nfiles: %d\n", out, manifest.PlanID, len(manifest.Files))
			}
			return err
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", datadogplan.DefaultWorkDir, "OpenExit Datadog state directory")
	cmd.Flags().StringVar(&out, "out", "", "Migration output directory")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing output directory transactionally")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Export a plan created from an explicitly accepted partial scan")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write the exported bundle manifest as JSON")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func newDatadogExplainCommand() *cobra.Command {
	var workDir string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "explain <source-ref>",
		Short: "Explain a resource's recorded conversion decision",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conversion, err := datadogplan.Explain(workDir, args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeDatadogJSON(cmd.OutOrStdout(), conversion)
			}
			var out strings.Builder
			fmt.Fprintf(&out, "source: %s\nname: %s\nstatus: %s\nsummary: %s\nreasons: %s\nevidence: %s\n",
				conversion.SourceRef, conversion.SourceName, conversion.Status, conversion.Summary,
				strings.Join(conversion.ReasonCodes, ", "), conversion.EvidencePath)
			if conversion.SourceURL != "" {
				fmt.Fprintf(&out, "source-url: %s\n", conversion.SourceURL)
			}
			for _, change := range conversion.SemanticChanges {
				fmt.Fprintf(&out, "change: %s (%s): %s\n", change.Code, change.Impact, change.Description)
			}
			for _, component := range conversion.Components {
				fmt.Fprintf(&out, "component: %s (%s): %s\n", component.ID, component.Kind, component.Status)
				if len(component.ReasonCodes) > 0 {
					fmt.Fprintf(&out, "  reasons: %s\n", strings.Join(component.ReasonCodes, ", "))
				}
				if component.SourceQuery != "" {
					fmt.Fprintf(&out, "  source-query: %s\n", component.SourceQuery)
				}
				if component.TargetQuery != "" {
					fmt.Fprintf(&out, "  target-query: %s\n", component.TargetQuery)
				}
				if component.Review != "" {
					fmt.Fprintf(&out, "  review: %s\n", component.Review)
				}
			}
			for _, output := range conversion.Outputs {
				fmt.Fprintf(&out, "output: %s (%s)\n", output.Path, output.Kind)
			}
			_, err = io.WriteString(cmd.OutOrStdout(), out.String())
			return err
		},
	}
	cmd.Flags().StringVar(&workDir, "workdir", datadogplan.DefaultWorkDir, "OpenExit Datadog state or exported migration directory")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write the conversion record as JSON")
	return cmd
}

func writeDatadogJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
