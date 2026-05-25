package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/assist"
	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/aiprovider"
	"github.com/RamazanKara/openexit/internal/collector/datadog"
	"github.com/RamazanKara/openexit/internal/collector/edge"
	"github.com/RamazanKara/openexit/internal/collector/githubenterprise"
	"github.com/RamazanKara/openexit/internal/collector/identity"
	openexport "github.com/RamazanKara/openexit/internal/export"
	"github.com/RamazanKara/openexit/internal/generate"
	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/validate"
	"github.com/RamazanKara/openexit/internal/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "openexit",
		Short:         "Local-first SaaS-to-open-source migration assessments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCommand())
	root.AddCommand(newInitCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newCollectCommand())
	root.AddCommand(newAssessCommand())
	root.AddCommand(newGenerateCommand())
	root.AddCommand(newValidateCommand())
	root.AddCommand(newExportCommand())
	root.AddCommand(newAssistCommand())
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print OpenExit version information",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name: %s\nversion: %s\ncommit: %s\ndate: %s\n", version.Name, version.Version, version.Commit, version.Date)
		},
	}
}

func newInitCommand() *cobra.Command {
	var source, target string
	cmd := &cobra.Command{
		Use:   "init <project-dir>",
		Short: "Initialize an OpenExit project directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := InitProjectWithEndpoints(args[0], source, target); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialized %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", defaultSource, "Source platform type")
	cmd.Flags().StringVar(&target, "target", "", "Target platform type; defaults to the source's standard target")
	return cmd
}

func newStatusCommand() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Validate an OpenExit project layout",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := CheckProject(project)
			if err != nil {
				return err
			}
			if len(status.Missing) > 0 {
				return fmt.Errorf("project is missing required directories: %s", strings.Join(status.Missing, ", "))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "project ok: %s\n", status.ProjectDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	return cmd
}

func newCollectCommand() *cobra.Command {
	collectCmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect source inventory",
	}
	collectCmd.AddCommand(newCollectFixtureCommand())
	collectCmd.AddCommand(newCollectDatadogCommand())
	collectCmd.AddCommand(newCollectGitHubFixtureCommand())
	collectCmd.AddCommand(newCollectIdentityFixtureCommand())
	collectCmd.AddCommand(newCollectEdgeFixtureCommand())
	collectCmd.AddCommand(newCollectAIFixtureCommand())
	return collectCmd
}

func newCollectFixtureCommand() *cobra.Command {
	var project, input string
	cmd := &cobra.Command{
		Use:   "fixture",
		Short: "Collect Datadog-like fixture inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "datadog", datadog.FixtureCollector{}, map[string]string{"input": input})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&input, "input", "", "Datadog fixture JSON file")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func newCollectDatadogCommand() *cobra.Command {
	var project, site, apiKeyEnv, appKeyEnv string
	cmd := &cobra.Command{
		Use:   "datadog",
		Short: "Collect read-only Datadog inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "datadog", datadog.LiveCollector{}, map[string]string{
				"site":        site,
				"api-key-env": apiKeyEnv,
				"app-key-env": appKeyEnv,
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&site, "site", "datadoghq.com", "Datadog site, such as datadoghq.com or datadoghq.eu")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "DATADOG_API_KEY", "Environment variable containing the Datadog API key")
	cmd.Flags().StringVar(&appKeyEnv, "app-key-env", "DATADOG_APP_KEY", "Environment variable containing the Datadog app key")
	return cmd
}

func newCollectGitHubFixtureCommand() *cobra.Command {
	var project, input string
	cmd := &cobra.Command{
		Use:   "github-fixture",
		Short: "Collect GitHub Enterprise-like fixture inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "github-enterprise", githubenterprise.FixtureCollector{}, map[string]string{"input": input})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&input, "input", "", "GitHub Enterprise fixture JSON file")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func newCollectIdentityFixtureCommand() *cobra.Command {
	var project, input string
	cmd := &cobra.Command{
		Use:   "identity-fixture",
		Short: "Collect Okta/Auth0-like fixture inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "identity", identity.FixtureCollector{}, map[string]string{"input": input})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&input, "input", "", "Okta/Auth0 identity fixture JSON file")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func newCollectEdgeFixtureCommand() *cobra.Command {
	var project, input string
	cmd := &cobra.Command{
		Use:   "edge-fixture",
		Short: "Collect Cloudflare/Akamai-like fixture inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "edge", edge.FixtureCollector{}, map[string]string{"input": input})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&input, "input", "", "Cloudflare/Akamai edge fixture JSON file")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func newCollectAIFixtureCommand() *cobra.Command {
	var project, input string
	cmd := &cobra.Command{
		Use:   "ai-fixture",
		Short: "Collect OpenAI/Anthropic-like fixture inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "ai-provider", aiprovider.FixtureCollector{}, map[string]string{"input": input})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&input, "input", "", "OpenAI/Anthropic AI provider fixture JSON file")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runCollector(ctx context.Context, cmd *cobra.Command, projectDir, expectedSource string, c collector.Collector, options map[string]string) error {
	cfg, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}
	if cfg.Source.Type != expectedSource {
		return fmt.Errorf("project source %q does not match %s collector source %q; initialize or update the project with source=%q target=%q", cfg.Source.Type, c.Name(), expectedSource, expectedSource, defaultTargetForSource(expectedSource))
	}
	inv, err := c.Collect(ctx, collector.CollectRequest{
		ProjectDir: projectDir,
		Project:    cfg.Metadata.Name,
		Source:     cfg.Source.Type,
		Options:    options,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "collected %s inventory: %s\n", c.Name(), inventorySummary(inv))
	return nil
}

func inventorySummary(inv *inventory.Inventory) string {
	switch inv.Source.Type {
	case "github-enterprise":
		return fmt.Sprintf("%d repositories, %d teams, %d workflows", inv.Summary.Repositories, inv.Summary.Teams, inv.Summary.ActionsWorkflows)
	case "identity":
		return fmt.Sprintf("%d applications, %d groups, %d policies", inv.Summary.IdentityApps, inv.Summary.IdentityGroups, inv.Summary.IdentityPolicies)
	case "edge":
		return fmt.Sprintf("%d DNS records, %d WAF rules, %d cache rules", inv.Summary.DNSRecords, inv.Summary.WAFRules, inv.Summary.CacheRules)
	case "ai-provider":
		return fmt.Sprintf("%d model usage classes, %d token profiles, %d tool usages", inv.Summary.AIModelUsageClasses, inv.Summary.AITokenVolumes, inv.Summary.AIToolUsages)
	default:
		return fmt.Sprintf("%d dashboards, %d monitors, %d SLOs", inv.Summary.Dashboards, inv.Summary.Monitors, inv.Summary.SLOs)
	}
}

func newAssessCommand() *cobra.Command {
	var project, target string
	cmd := &cobra.Command{
		Use:   "assess",
		Short: "Analyze inventory and write an assessment manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadProjectConfig(project)
			if err != nil {
				return err
			}
			if target == "" {
				target = cfg.Target.Type
			}
			inv, err := loadInventoryManifest(project)
			if err != nil {
				return err
			}
			if cfg.Source.Type != inv.Source.Type {
				return fmt.Errorf("project source %q does not match inventory source %q; collect inventory for the configured source or update openexit.yaml", cfg.Source.Type, inv.Source.Type)
			}
			if target != cfg.Target.Type {
				return fmt.Errorf("assessment target %q does not match project target %q; use --target %s or update openexit.yaml", target, cfg.Target.Type, cfg.Target.Type)
			}
			result, err := assessment.Run(cmd.Context(), inv, target, time.Now().UTC())
			if err != nil {
				return err
			}
			if err := writeAssessmentManifest(project, result); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "assessment written with %d findings\n", len(result.Findings))
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&target, "target", "", "Target migration platform")
	return cmd
}

func newGenerateCommand() *cobra.Command {
	var project string
	var artifacts []string
	var all bool
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate deterministic migration artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if err := generate.GenerateAll(project); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "generated all artifacts")
				return nil
			}
			if len(artifacts) == 0 {
				return fmt.Errorf("set --all or at least one --artifact")
			}
			if err := generate.Generate(project, artifacts); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "generated artifacts: %s\n", strings.Join(artifacts, ", "))
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringArrayVar(&artifacts, "artifact", nil, "Artifact to generate")
	cmd.Flags().BoolVar(&all, "all", false, "Generate all artifacts")
	return cmd
}

func newValidateCommand() *cobra.Command {
	var project string
	var strict bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate generated OpenExit output",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := validate.Run(project, strict)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "validation status: %s\n", report.Status)
			return err
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as failures")
	return cmd
}

func newExportCommand() *cobra.Command {
	var project, format, out string
	var force bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export an evidence bundle",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := openexport.Bundle(openexport.Options{ProjectDir: project, Format: format, Out: out, Force: force}); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "exported %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&format, "format", "zip", "Export format")
	cmd.Flags().StringVar(&out, "out", "", "Output bundle path")
	cmd.Flags().BoolVar(&force, "force", false, "Export even when validation has critical failures")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func newAssistCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "assist",
		Short: "Optional AI assist commands",
	}
	root.AddCommand(newAssistSummarizeCommand())
	return root
}

func newAssistSummarizeCommand() *cobra.Command {
	var project, providerName, model, input, out string
	var saveRedactedInput bool
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Create an optional AI-assisted summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadProjectConfig(project)
			if err != nil {
				return err
			}
			if providerName == "" {
				providerName = cfg.Assist.Provider
			}
			if providerName == "" {
				providerName = "noop"
			}
			provider, err := assistProvider(providerName)
			if err != nil {
				return err
			}
			if err := ensureAssistAllowed(cfg, providerName); err != nil {
				return err
			}
			if model == "" {
				model = cfg.Assist.Model
			}
			if input == "" {
				input = filepath.Join(project, "assessment", "openexit.assessment.yaml")
			}
			if out == "" {
				out = filepath.Join(project, "assessment", "executive-summary.ai.md")
			}
			if !strings.HasSuffix(out, ".ai.md") {
				return fmt.Errorf("assist outputs must use .ai.md suffix")
			}
			if _, err := os.Stat(out); err == nil {
				return fmt.Errorf("refusing to overwrite existing assist output %s", out)
			}
			data, err := os.ReadFile(input)
			if err != nil {
				return err
			}
			redactedInput := assist.Redact(string(data))
			req := assist.Request{
				Task:         "summarize",
				Model:        model,
				SystemPrompt: assist.SummarySystemPrompt,
				Input:        map[string]any{"content": redactedInput},
				Metadata:     map[string]string{"project": cfg.Metadata.Name, "input": input},
			}
			if saveRedactedInput {
				if err := writeAssistAuditInput(out, providerName, req); err != nil {
					return err
				}
			}
			resp, err := provider.Complete(cmd.Context(), req)
			if err != nil {
				return err
			}
			if dir := filepath.Dir(out); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
			}
			if err := os.WriteFile(out, []byte(assist.EnsureReviewHeader(resp.Text)+"\n"), 0o644); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "assist output written: %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&providerName, "provider", "", "Assist provider")
	cmd.Flags().StringVar(&model, "model", "", "Assist model")
	cmd.Flags().StringVar(&input, "input", "", "Input file")
	cmd.Flags().StringVar(&out, "out", "", "Output .ai.md file")
	cmd.Flags().BoolVar(&saveRedactedInput, "save-redacted-input", false, "Save redacted assist input next to the output for audit")
	return cmd
}

func assistProvider(name string) (assist.Provider, error) {
	switch name {
	case "noop", "":
		return assist.NoopProvider{}, nil
	case "litellm":
		return assist.NewLiteLLMProviderFromEnv(), nil
	default:
		return nil, fmt.Errorf("unsupported assist provider %q", name)
	}
}

func ensureAssistAllowed(cfg *ProjectConfig, providerName string) error {
	if providerName == "" || providerName == "noop" {
		return nil
	}
	if !cfg.Policy.AllowAI {
		return fmt.Errorf("policy.allowAI must be true to use provider %q", providerName)
	}
	if !cfg.Assist.Enabled {
		return fmt.Errorf("assist.enabled must be true to use provider %q", providerName)
	}
	if !cfg.Assist.AllowExternalProvider {
		return fmt.Errorf("assist.allowExternalProvider must be true to use provider %q", providerName)
	}
	if cfg.Assist.Provider != providerName {
		return fmt.Errorf("assist.provider must be %q to use this provider", providerName)
	}
	return nil
}

func writeAssistAuditInput(outPath, providerName string, req assist.Request) error {
	auditPath := strings.TrimSuffix(outPath, ".ai.md") + ".input.redacted.json"
	if _, err := os.Stat(auditPath); err == nil {
		return fmt.Errorf("refusing to overwrite existing assist audit input %s", auditPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if dir := filepath.Dir(auditPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	payload := map[string]any{
		"provider": providerName,
		"request":  req,
		"redacted": true,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(auditPath, append(data, '\n'), 0o644)
}

func loadInventoryManifest(project string) (*inventory.Inventory, error) {
	data, err := os.ReadFile(filepath.Join(project, "inventory", "openexit.inventory.yaml"))
	if err != nil {
		return nil, err
	}
	var inv inventory.Inventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	if err := inventory.Validate(&inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

func writeAssessmentManifest(project string, a *assessment.Assessment) error {
	if err := assessment.Validate(a); err != nil {
		return err
	}
	dir := filepath.Join(project, "assessment")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "openexit.assessment.yaml"), yamlData, 0o644); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "openexit.assessment.json"), append(jsonData, '\n'), 0o644)
}
