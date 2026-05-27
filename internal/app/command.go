package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/RamazanKara/openexit/internal/mapping"
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
	root.AddCommand(newDemoCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newRunCommand())
	root.AddCommand(newCollectCommand())
	root.AddCommand(newAssessCommand())
	root.AddCommand(newMapCommand())
	root.AddCommand(newGenerateCommand())
	root.AddCommand(newValidateCommand())
	root.AddCommand(newExportCommand())
	root.AddCommand(newVerifyBundleCommand())
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
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show OpenExit project readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := CheckProject(project)
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(status); encErr != nil {
					return encErr
				}
				return err
			}
			if err != nil {
				return err
			}
			writeProjectStatus(cmd.OutOrStdout(), status)
			if len(status.Missing) > 0 {
				return fmt.Errorf("project is missing required directories: %s", strings.Join(status.Missing, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable project status")
	return cmd
}

func newRunCommand() *cobra.Command {
	var project, out string
	var exportBundle, strict bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the deterministic pipeline for a collected project",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := runProjectWorkflow(cmd.Context(), cmd.OutOrStdout(), workflowOptions{
				ProjectDir: project,
				Strict:     strict,
				Export:     exportBundle,
				Out:        out,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat validation warnings as failures")
	cmd.Flags().BoolVar(&exportBundle, "export", false, "Export an evidence bundle after validation passes")
	cmd.Flags().StringVar(&out, "out", "", "Output bundle path when --export is set; defaults to openexit-evidence.zip")
	return cmd
}

type workflowOptions struct {
	ProjectDir string
	Strict     bool
	Export     bool
	Out        string
}

func runProjectWorkflow(ctx context.Context, w io.Writer, opts workflowOptions) (*ProjectStatus, error) {
	if opts.ProjectDir == "" {
		opts.ProjectDir = "."
	}
	cfg, err := LoadProjectConfig(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(w, "run: assess %s -> %s\n", cfg.Source.Type, cfg.Target.Type)
	a, err := runAssessment(ctx, opts.ProjectDir, cfg.Target.Type)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(w, "assessment written with %d findings\n", len(a.Findings))

	_, _ = fmt.Fprintln(w, "run: map")
	if err := writeMapping(opts.ProjectDir); err != nil {
		return nil, err
	}
	result, err := loadMappingManifest(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(w, "mapping written: %d dashboard drafts, %d alert rule drafts, %d manual review items\n", len(result.DashboardDrafts), len(result.AlertRuleDrafts), len(result.ManualReview))

	_, _ = fmt.Fprintln(w, "run: generate --all")
	if err := generate.GenerateAll(opts.ProjectDir); err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintln(w, "generated all artifacts")

	_, _ = fmt.Fprintln(w, "run: validate")
	report, validationErr := validate.Run(opts.ProjectDir, opts.Strict)
	if report != nil {
		_, _ = fmt.Fprintf(w, "validation status: %s\n", report.Status)
	}
	status, statusErr := CheckProject(opts.ProjectDir)
	if statusErr != nil {
		return status, statusErr
	}
	writeProjectStatus(w, status)
	if validationErr != nil {
		return status, validationErr
	}
	if opts.Export {
		if opts.Out == "" {
			opts.Out = "openexit-evidence.zip"
		}
		_, _ = fmt.Fprintf(w, "run: export %s\n", opts.Out)
		if err := openexport.Bundle(openexport.Options{ProjectDir: opts.ProjectDir, Format: "zip", Out: opts.Out}); err != nil {
			return status, err
		}
		_, _ = fmt.Fprintf(w, "exported %s\n", opts.Out)
	}
	return status, nil
}

func writeProjectStatus(w io.Writer, status *ProjectStatus) {
	_, _ = fmt.Fprintf(w, "project: %s\n", status.ProjectDir)
	_, _ = fmt.Fprintf(w, "source: %s\n", status.Source)
	_, _ = fmt.Fprintf(w, "target: %s\n", status.Target)
	if len(status.Missing) > 0 {
		_, _ = fmt.Fprintf(w, "layout: missing %s\n", strings.Join(status.Missing, ", "))
	} else {
		_, _ = fmt.Fprintln(w, "layout: ok")
	}
	_, _ = fmt.Fprintf(w, "inventory: %s\n", formatInventoryStatus(status.Inventory))
	_, _ = fmt.Fprintf(w, "assessment: %s\n", formatAssessmentStatus(status.Assessment))
	_, _ = fmt.Fprintf(w, "mapping: %s\n", formatMappingStatus(status.Mapping))
	_, _ = fmt.Fprintf(w, "generated: %s\n", formatGeneratedStatus(status.Generated))
	_, _ = fmt.Fprintf(w, "validation: %s\n", formatValidationStatus(status.Validation))
	if status.ReadyForExport {
		_, _ = fmt.Fprintln(w, "export-ready: yes")
	} else {
		_, _ = fmt.Fprintln(w, "export-ready: no")
	}
	for _, action := range status.NextActions {
		_, _ = fmt.Fprintf(w, "next: %s\n", action)
	}
}

func formatInventoryStatus(status InventoryStatus) string {
	if !status.Present {
		return "missing"
	}
	text := "present"
	if counts := formatCounts(status.Assets); counts != "" {
		text += " (" + counts + ")"
	}
	if status.Warnings > 0 {
		text += fmt.Sprintf(" warnings=%d", status.Warnings)
	}
	if status.Error != "" {
		text += " error=" + status.Error
	}
	return text
}

func formatAssessmentStatus(status AssessmentStatus) string {
	if !status.Present {
		return "missing"
	}
	parts := []string{fmt.Sprintf("findings=%d", status.Findings)}
	if severity := formatCounts(status.Severity); severity != "" {
		parts = append(parts, severity)
	}
	if status.Score > 0 {
		parts = append(parts, fmt.Sprintf("score=%d", status.Score))
	}
	if status.Level != "" {
		parts = append(parts, "level="+status.Level)
	}
	if status.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("warnings=%d", status.Warnings))
	}
	text := "present (" + strings.Join(parts, " ") + ")"
	if status.Error != "" {
		text += " error=" + status.Error
	}
	return text
}

func formatMappingStatus(status MappingStatus) string {
	if !status.Present {
		return "missing"
	}
	text := fmt.Sprintf("present (dashboardDrafts=%d alertRuleDrafts=%d unsupportedItems=%d manualReview=%d)", status.DashboardDrafts, status.AlertRuleDrafts, status.UnsupportedItems, status.ManualReview)
	if status.Error != "" {
		text += " error=" + status.Error
	}
	return text
}

func formatGeneratedStatus(status GeneratedStatus) string {
	if !status.Present {
		if status.Error != "" {
			return "missing error=" + status.Error
		}
		return "missing"
	}
	text := fmt.Sprintf("present (files=%d candidates=%d)", status.Files, status.Candidates)
	if status.Error != "" {
		text += " error=" + status.Error
	}
	return text
}

func formatValidationStatus(status ValidationStatus) string {
	if !status.Present {
		return "missing"
	}
	text := fmt.Sprintf("%s (checks=%d passed=%d failed=%d warnings=%d)", status.Status, status.Checks, status.Passed, status.Failed, status.Warnings)
	if status.Error != "" {
		text += " error=" + status.Error
	}
	return text
}

func formatCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, " ")
}

func newCollectCommand() *cobra.Command {
	collectCmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect source inventory",
	}
	collectCmd.AddCommand(newCollectFixtureCommand())
	collectCmd.AddCommand(newCollectDatadogCommand())
	collectCmd.AddCommand(newCollectGitHubCommand())
	collectCmd.AddCommand(newCollectGitHubFixtureCommand())
	collectCmd.AddCommand(newCollectOktaCommand())
	collectCmd.AddCommand(newCollectAuth0Command())
	collectCmd.AddCommand(newCollectIdentityFixtureCommand())
	collectCmd.AddCommand(newCollectCloudflareCommand())
	collectCmd.AddCommand(newCollectAkamaiCommand())
	collectCmd.AddCommand(newCollectEdgeFixtureCommand())
	collectCmd.AddCommand(newCollectOpenAICommand())
	collectCmd.AddCommand(newCollectAnthropicCommand())
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

func newCollectGitHubCommand() *cobra.Command {
	var project, owner, ownerType, baseURL, tokenEnv string
	var repos []string
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Collect read-only GitHub or GitHub Enterprise inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "github-enterprise", githubenterprise.LiveCollector{}, map[string]string{
				"owner":      owner,
				"owner-type": ownerType,
				"base-url":   baseURL,
				"token-env":  tokenEnv,
				"repos":      strings.Join(repos, "\n"),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub organization or user login to inventory")
	cmd.Flags().StringVar(&ownerType, "owner-type", "org", "GitHub owner type: org or user")
	cmd.Flags().StringVar(&baseURL, "base-url", "https://api.github.com", "GitHub API base URL; use the /api/v3 URL for GitHub Enterprise Server")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "GITHUB_TOKEN", "Environment variable containing a read-only GitHub token")
	cmd.Flags().StringArrayVar(&repos, "repo", nil, "Limit collection to a repository; repeatable; accepts name or owner/name")
	_ = cmd.MarkFlagRequired("owner")
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

func newCollectOktaCommand() *cobra.Command {
	var project, orgURL, tokenEnv, authScheme string
	var breakGlassUsers []string
	cmd := &cobra.Command{
		Use:   "okta",
		Short: "Collect read-only Okta identity inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "identity", identity.LiveCollector{}, map[string]string{
				"org-url":           orgURL,
				"token-env":         tokenEnv,
				"auth-scheme":       authScheme,
				"break-glass-users": strings.Join(breakGlassUsers, "\n"),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&orgURL, "org-url", "", "Okta org URL, such as https://dev-123456.okta.com")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "OKTA_API_TOKEN", "Environment variable containing a read-only Okta token")
	cmd.Flags().StringVar(&authScheme, "auth-scheme", "SSWS", "Authorization scheme for the token: SSWS or Bearer")
	cmd.Flags().StringArrayVar(&breakGlassUsers, "break-glass-user", nil, "Break-glass user login to inventory; repeatable")
	_ = cmd.MarkFlagRequired("org-url")
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

func newCollectAuth0Command() *cobra.Command {
	var project, domain, tokenEnv string
	var breakGlassUsers []string
	cmd := &cobra.Command{
		Use:   "auth0",
		Short: "Collect read-only Auth0 identity inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "identity", identity.Auth0Collector{}, map[string]string{
				"domain":            domain,
				"token-env":         tokenEnv,
				"break-glass-users": strings.Join(breakGlassUsers, "\n"),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&domain, "domain", "", "Auth0 tenant domain, such as https://example.us.auth0.com")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "AUTH0_MANAGEMENT_TOKEN", "Environment variable containing an Auth0 Management API token")
	cmd.Flags().StringArrayVar(&breakGlassUsers, "break-glass-user", nil, "Break-glass user email, username, or Auth0 user ID to inventory; repeatable")
	_ = cmd.MarkFlagRequired("domain")
	return cmd
}

func newCollectCloudflareCommand() *cobra.Command {
	var project, zoneID, tokenEnv, baseURL string
	cmd := &cobra.Command{
		Use:   "cloudflare",
		Short: "Collect read-only Cloudflare edge inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "edge", edge.LiveCollector{}, map[string]string{
				"zone-id":   zoneID,
				"token-env": tokenEnv,
				"base-url":  baseURL,
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID to inventory")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "CLOUDFLARE_API_TOKEN", "Environment variable containing a read-only Cloudflare API token")
	cmd.Flags().StringVar(&baseURL, "base-url", "https://api.cloudflare.com/client/v4", "Cloudflare API base URL")
	_ = cmd.MarkFlagRequired("zone-id")
	return cmd
}

func newCollectAkamaiCommand() *cobra.Command {
	var project, edgerc, section, baseURL, contractID, groupID, accountSwitchKey string
	var hostEnv, clientTokenEnv, accessTokenEnv, clientSecretEnv, accountSwitchKeyEnv string
	var zones, propertyIDs, securityConfigIDs []string
	var discoverProperties, discoverSecurityConfigs bool
	var securityVersion int
	cmd := &cobra.Command{
		Use:   "akamai",
		Short: "Collect read-only Akamai edge inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "edge", edge.AkamaiCollector{}, map[string]string{
				"edgerc":                    edgerc,
				"edgerc-section":            section,
				"base-url":                  baseURL,
				"host-env":                  hostEnv,
				"client-token-env":          clientTokenEnv,
				"access-token-env":          accessTokenEnv,
				"client-secret-env":         clientSecretEnv,
				"account-switch-key-env":    accountSwitchKeyEnv,
				"account-switch-key":        accountSwitchKey,
				"contract-id":               contractID,
				"group-id":                  groupID,
				"zones":                     strings.Join(zones, "\n"),
				"property-ids":              strings.Join(propertyIDs, "\n"),
				"security-config-ids":       strings.Join(securityConfigIDs, "\n"),
				"discover-properties":       strconv.FormatBool(discoverProperties),
				"discover-security-configs": strconv.FormatBool(discoverSecurityConfigs),
				"security-version":          strconv.Itoa(securityVersion),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringArrayVar(&zones, "zone", nil, "Akamai Edge DNS zone to inventory; repeatable")
	cmd.Flags().StringArrayVar(&propertyIDs, "property-id", nil, "Akamai Property Manager property ID to inventory; repeatable")
	cmd.Flags().StringVar(&contractID, "contract-id", "", "Optional Akamai contract ID for Property Manager calls")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Optional Akamai group ID for Property Manager calls")
	cmd.Flags().BoolVar(&discoverProperties, "discover-properties", false, "List accessible Property Manager properties before collection")
	cmd.Flags().StringArrayVar(&securityConfigIDs, "security-config-id", nil, "Akamai AppSec config ID or ID:version to inventory; repeatable")
	cmd.Flags().IntVar(&securityVersion, "security-version", 0, "Default AppSec config version when --security-config-id omits one")
	cmd.Flags().BoolVar(&discoverSecurityConfigs, "discover-security-configs", false, "List accessible AppSec security configs before collection")
	cmd.Flags().StringVar(&edgerc, "edgerc", "~/.edgerc", "Akamai EdgeGrid credential file")
	cmd.Flags().StringVar(&section, "edgerc-section", "default", "Akamai EdgeGrid credential section")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Akamai API base URL; defaults to the host in EdgeGrid credentials")
	cmd.Flags().StringVar(&accountSwitchKey, "account-switch-key", "", "Optional Akamai account switch key")
	cmd.Flags().StringVar(&hostEnv, "host-env", "AKAMAI_HOST", "Environment variable containing the Akamai API host")
	cmd.Flags().StringVar(&clientTokenEnv, "client-token-env", "AKAMAI_CLIENT_TOKEN", "Environment variable containing the Akamai client token")
	cmd.Flags().StringVar(&accessTokenEnv, "access-token-env", "AKAMAI_ACCESS_TOKEN", "Environment variable containing the Akamai access token")
	cmd.Flags().StringVar(&clientSecretEnv, "client-secret-env", "AKAMAI_CLIENT_SECRET", "Environment variable containing the Akamai client secret")
	cmd.Flags().StringVar(&accountSwitchKeyEnv, "account-switch-key-env", "AKAMAI_ACCOUNT_KEY", "Environment variable containing an optional Akamai account switch key")
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

func newCollectOpenAICommand() *cobra.Command {
	var project, adminKeyEnv, baseURL, workspace, organizationID, projectID string
	var days, peakDays, defaultP95Ms, defaultTimeoutMs, fallbackMaxRetries int
	var owners []string
	var streamingRequired, fallbackManualQueue bool
	var fallbackStrategy string
	cmd := &cobra.Command{
		Use:   "openai",
		Short: "Collect read-only OpenAI aggregate usage inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "ai-provider", aiprovider.LiveCollector{}, map[string]string{
				"admin-key-env":         adminKeyEnv,
				"base-url":              baseURL,
				"days":                  strconv.Itoa(days),
				"peak-days":             strconv.Itoa(peakDays),
				"workspace":             workspace,
				"organization-id":       organizationID,
				"project-id":            projectID,
				"owners":                strings.Join(owners, "\n"),
				"default-p95-ms":        strconv.Itoa(defaultP95Ms),
				"default-timeout-ms":    strconv.Itoa(defaultTimeoutMs),
				"streaming-required":    strconv.FormatBool(streamingRequired),
				"fallback-strategy":     fallbackStrategy,
				"fallback-manual-queue": strconv.FormatBool(fallbackManualQueue),
				"fallback-max-retries":  strconv.Itoa(fallbackMaxRetries),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&adminKeyEnv, "admin-key-env", "OPENAI_ADMIN_KEY", "Environment variable containing an OpenAI admin key")
	cmd.Flags().StringVar(&baseURL, "base-url", "https://api.openai.com/v1", "OpenAI API base URL")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days of aggregate usage to collect")
	cmd.Flags().IntVar(&peakDays, "peak-days", 7, "Number of recent days to use for hourly peak estimates")
	cmd.Flags().StringVar(&workspace, "workspace", "organization", "Workspace label to record in inventory")
	cmd.Flags().StringVar(&organizationID, "organization-id", "", "Optional OpenAI organization ID header")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Optional OpenAI project ID header")
	cmd.Flags().StringArrayVar(&owners, "owner", nil, "Owner for collected AI usage classes; repeatable")
	cmd.Flags().IntVar(&defaultP95Ms, "default-p95-ms", 0, "Optional default p95 latency expectation in milliseconds")
	cmd.Flags().IntVar(&defaultTimeoutMs, "default-timeout-ms", 0, "Optional default timeout expectation in milliseconds")
	cmd.Flags().BoolVar(&streamingRequired, "streaming-required", false, "Mark collected usage classes as requiring streaming")
	cmd.Flags().StringVar(&fallbackStrategy, "fallback-strategy", "", "Optional fallback strategy to attach to collected usage classes")
	cmd.Flags().BoolVar(&fallbackManualQueue, "fallback-manual-queue", false, "Mark fallback strategy as using a manual queue")
	cmd.Flags().IntVar(&fallbackMaxRetries, "fallback-max-retries", 0, "Optional maximum retry count for fallback behavior")
	return cmd
}

func newCollectAnthropicCommand() *cobra.Command {
	var project, adminKeyEnv, baseURL, anthropicVersion, workspace string
	var days, peakDays, defaultP95Ms, defaultTimeoutMs, fallbackMaxRetries int
	var owners, workspaceIDs, apiKeyIDs, models []string
	var streamingRequired, fallbackManualQueue bool
	var fallbackStrategy string
	cmd := &cobra.Command{
		Use:   "anthropic",
		Short: "Collect read-only Anthropic aggregate usage inventory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollector(cmd.Context(), cmd, project, "ai-provider", aiprovider.AnthropicCollector{}, map[string]string{
				"admin-key-env":         adminKeyEnv,
				"base-url":              baseURL,
				"anthropic-version":     anthropicVersion,
				"days":                  strconv.Itoa(days),
				"peak-days":             strconv.Itoa(peakDays),
				"workspace":             workspace,
				"owners":                strings.Join(owners, "\n"),
				"workspace-ids":         strings.Join(workspaceIDs, "\n"),
				"api-key-ids":           strings.Join(apiKeyIDs, "\n"),
				"models":                strings.Join(models, "\n"),
				"default-p95-ms":        strconv.Itoa(defaultP95Ms),
				"default-timeout-ms":    strconv.Itoa(defaultTimeoutMs),
				"streaming-required":    strconv.FormatBool(streamingRequired),
				"fallback-strategy":     fallbackStrategy,
				"fallback-manual-queue": strconv.FormatBool(fallbackManualQueue),
				"fallback-max-retries":  strconv.Itoa(fallbackMaxRetries),
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
	cmd.Flags().StringVar(&adminKeyEnv, "admin-key-env", "ANTHROPIC_ADMIN_KEY", "Environment variable containing an Anthropic Admin API key")
	cmd.Flags().StringVar(&baseURL, "base-url", "https://api.anthropic.com", "Anthropic API base URL")
	cmd.Flags().StringVar(&anthropicVersion, "anthropic-version", "2023-06-01", "Anthropic API version header")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days of aggregate usage to collect")
	cmd.Flags().IntVar(&peakDays, "peak-days", 7, "Number of recent days to use for hourly peak estimates")
	cmd.Flags().StringVar(&workspace, "workspace", "organization", "Workspace label to record in inventory")
	cmd.Flags().StringArrayVar(&owners, "owner", nil, "Owner for collected AI usage classes; repeatable")
	cmd.Flags().StringArrayVar(&workspaceIDs, "workspace-id", nil, "Limit collection to an Anthropic workspace ID; repeatable")
	cmd.Flags().StringArrayVar(&apiKeyIDs, "api-key-id", nil, "Limit collection to an Anthropic API key ID; repeatable")
	cmd.Flags().StringArrayVar(&models, "model", nil, "Limit collection to an Anthropic model ID; repeatable")
	cmd.Flags().IntVar(&defaultP95Ms, "default-p95-ms", 0, "Optional default p95 latency expectation in milliseconds")
	cmd.Flags().IntVar(&defaultTimeoutMs, "default-timeout-ms", 0, "Optional default timeout expectation in milliseconds")
	cmd.Flags().BoolVar(&streamingRequired, "streaming-required", false, "Mark collected usage classes as requiring streaming")
	cmd.Flags().StringVar(&fallbackStrategy, "fallback-strategy", "", "Optional fallback strategy to attach to collected usage classes")
	cmd.Flags().BoolVar(&fallbackManualQueue, "fallback-manual-queue", false, "Mark fallback strategy as using a manual queue")
	cmd.Flags().IntVar(&fallbackMaxRetries, "fallback-max-retries", 0, "Optional maximum retry count for fallback behavior")
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
			result, err := runAssessment(cmd.Context(), project, target)
			if err != nil {
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

func runAssessment(ctx context.Context, project, target string) (*assessment.Assessment, error) {
	cfg, err := LoadProjectConfig(project)
	if err != nil {
		return nil, err
	}
	if target == "" {
		target = cfg.Target.Type
	}
	inv, err := loadInventoryManifest(project)
	if err != nil {
		return nil, err
	}
	if cfg.Source.Type != inv.Source.Type {
		return nil, fmt.Errorf("project source %q does not match inventory source %q; collect inventory for the configured source or update openexit.yaml", cfg.Source.Type, inv.Source.Type)
	}
	if target != cfg.Target.Type {
		return nil, fmt.Errorf("assessment target %q does not match project target %q; use --target %s or update openexit.yaml", target, cfg.Target.Type, cfg.Target.Type)
	}
	result, err := assessment.Run(ctx, inv, target, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := writeAssessmentManifest(project, result); err != nil {
		return nil, err
	}
	return result, nil
}

func newMapCommand() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Map source inventory to deterministic target candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := writeMapping(project); err != nil {
				return err
			}
			result, err := loadMappingManifest(project)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mapping written: %d dashboard drafts, %d alert rule drafts, %d manual review items\n", len(result.DashboardDrafts), len(result.AlertRuleDrafts), len(result.ManualReview))
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "OpenExit project directory")
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

func writeMapping(project string) error {
	cfg, err := LoadProjectConfig(project)
	if err != nil {
		return err
	}
	inv, err := loadInventoryManifest(project)
	if err != nil {
		return err
	}
	a, err := loadAssessmentManifest(project)
	if err != nil {
		return err
	}
	if cfg.Source.Type != inv.Source.Type {
		return fmt.Errorf("project source %q does not match inventory source %q; collect inventory for the configured source or update openexit.yaml", cfg.Source.Type, inv.Source.Type)
	}
	if inv.Source.Type != a.Source.Type {
		return fmt.Errorf("inventory source %q does not match assessment source %q", inv.Source.Type, a.Source.Type)
	}
	if cfg.Target.Type != a.Target.Type {
		return fmt.Errorf("project target %q does not match assessment target %q", cfg.Target.Type, a.Target.Type)
	}
	result, err := mapping.Build(inv, a, time.Now().UTC())
	if err != nil {
		return err
	}
	return mapping.Write(project, result)
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

func newVerifyBundleCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "verify-bundle <bundle.zip>",
		Short: "Verify an exported OpenExit evidence bundle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := openexport.Verify(openexport.VerifyOptions{BundlePath: args[0]})
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(report); encErr != nil {
					return encErr
				}
				return err
			}
			writeBundleVerification(cmd.OutOrStdout(), report)
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable bundle verification report")
	return cmd
}

func writeBundleVerification(w io.Writer, report *openexport.VerificationReport) {
	if report == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "bundle: %s\n", report.BundlePath)
	_, _ = fmt.Fprintf(w, "status: %s\n", report.Status)
	if report.Project.Name != "" || report.Project.Source != "" || report.Project.Target != "" {
		_, _ = fmt.Fprintf(w, "project: %s (%s -> %s)\n", report.Project.Name, report.Project.Source, report.Project.Target)
	}
	if report.Build.Version != "" || report.Build.Commit != "" || report.Build.Date != "" {
		_, _ = fmt.Fprintf(w, "build: version=%s commit=%s date=%s\n", report.Build.Version, report.Build.Commit, report.Build.Date)
	}
	if report.Validation.Status != "" {
		_, _ = fmt.Fprintf(w, "validation: %s (checks=%d passed=%d failed=%d warnings=%d)\n", report.Validation.Status, report.Validation.Checks, report.Validation.Passed, report.Validation.Failed, report.Validation.Warnings)
	}
	_, _ = fmt.Fprintf(w, "files: archive=%d manifest=%d checksums=%d\n", report.ArchiveFiles, report.ManifestFiles, report.ChecksumEntries)
	for _, message := range report.Errors {
		_, _ = fmt.Fprintf(w, "error: %s\n", message)
	}
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

func loadAssessmentManifest(project string) (*assessment.Assessment, error) {
	data, err := os.ReadFile(filepath.Join(project, "assessment", "openexit.assessment.yaml"))
	if err != nil {
		return nil, err
	}
	var a assessment.Assessment
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	if err := assessment.Validate(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

func loadMappingManifest(project string) (*mapping.MappingResult, error) {
	data, err := os.ReadFile(filepath.Join(project, "mapping", "openexit.mapping.yaml"))
	if err != nil {
		return nil, err
	}
	var result mapping.MappingResult
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if err := mapping.Validate(&result); err != nil {
		return nil, err
	}
	return &result, nil
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
