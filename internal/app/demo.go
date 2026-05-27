package app

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RamazanKara/openexit/internal/collector"
	"github.com/RamazanKara/openexit/internal/collector/aiprovider"
	"github.com/RamazanKara/openexit/internal/collector/datadog"
	"github.com/RamazanKara/openexit/internal/collector/edge"
	"github.com/RamazanKara/openexit/internal/collector/githubenterprise"
	"github.com/RamazanKara/openexit/internal/collector/identity"
	"github.com/spf13/cobra"
)

//go:embed demo-fixtures/*.json
var demoFixtures embed.FS

type demoOptions struct {
	ProjectDir string
	Source     string
	Out        string
	Force      bool
}

func newDemoCommand() *cobra.Command {
	var source, out string
	var force bool
	cmd := &cobra.Command{
		Use:   "demo <project-dir>",
		Short: "Create a complete built-in demo project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDemo(cmd.Context(), cmd.OutOrStdout(), demoOptions{
				ProjectDir: args[0],
				Source:     source,
				Out:        out,
				Force:      force,
			})
		},
	}
	cmd.Flags().StringVar(&source, "source", defaultSource, "Built-in demo source: datadog, github-enterprise, identity, edge, or ai-provider")
	cmd.Flags().StringVar(&out, "out", "", "Output bundle path; defaults to <project-dir>/openexit-demo.zip")
	cmd.Flags().BoolVar(&force, "force", false, "Remove the demo project directory first if it already exists")
	return cmd
}

func runDemo(ctx context.Context, w io.Writer, opts demoOptions) error {
	opts.Source = strings.TrimSpace(opts.Source)
	if opts.Source == "" {
		opts.Source = defaultSource
	}
	target := defaultTargetForSource(opts.Source)
	if target == "" {
		return fmt.Errorf("unsupported demo source %q", opts.Source)
	}
	if err := prepareDemoProjectDir(opts.ProjectDir, opts.Force); err != nil {
		return err
	}
	if err := InitProjectWithEndpoints(opts.ProjectDir, opts.Source, target); err != nil {
		return err
	}
	fixturePath, cleanup, err := writeDemoFixture(opts.Source)
	if err != nil {
		return err
	}
	defer cleanup()

	c, err := demoCollector(opts.Source)
	if err != nil {
		return err
	}
	cfg, err := LoadProjectConfig(opts.ProjectDir)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "demo: %s -> %s\n", opts.Source, target)
	inv, err := c.Collect(ctx, collector.CollectRequest{
		ProjectDir: opts.ProjectDir,
		Project:    cfg.Metadata.Name,
		Source:     cfg.Source.Type,
		Options:    map[string]string{"input": fixturePath},
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "collected %s inventory: %s\n", c.Name(), inventorySummary(inv))
	if opts.Out == "" {
		opts.Out = filepath.Join(opts.ProjectDir, "openexit-demo.zip")
	}
	_, err = runProjectWorkflow(ctx, w, workflowOptions{ProjectDir: opts.ProjectDir, Export: true, Out: opts.Out})
	return err
}

func prepareDemoProjectDir(projectDir string, force bool) error {
	if strings.TrimSpace(projectDir) == "" {
		return fmt.Errorf("project directory is required")
	}
	clean := filepath.Clean(projectDir)
	if force {
		if clean == "." || clean == string(filepath.Separator) {
			return fmt.Errorf("refusing to remove unsafe demo directory %q", projectDir)
		}
		if err := os.RemoveAll(projectDir); err != nil {
			return err
		}
		return nil
	}
	entries, err := os.ReadDir(projectDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("demo project directory %s is not empty; choose another directory or pass --force", projectDir)
	}
	return nil
}

func writeDemoFixture(source string) (string, func(), error) {
	data, err := demoFixtures.ReadFile("demo-fixtures/" + source + ".json")
	if err != nil {
		return "", func() {}, fmt.Errorf("read built-in demo fixture: %w", err)
	}
	file, err := os.CreateTemp("", "openexit-demo-"+source+"-*.json")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return file.Name(), cleanup, nil
}

func demoCollector(source string) (collector.Collector, error) {
	switch source {
	case "datadog":
		return datadog.FixtureCollector{}, nil
	case "github-enterprise":
		return githubenterprise.FixtureCollector{}, nil
	case "identity":
		return identity.FixtureCollector{}, nil
	case "edge":
		return edge.FixtureCollector{}, nil
	case "ai-provider":
		return aiprovider.FixtureCollector{}, nil
	default:
		return nil, fmt.Errorf("unsupported demo source %q", source)
	}
}
