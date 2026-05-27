package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/version"
	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

type DoctorReport struct {
	Status string        `json:"status"`
	Checks []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func newDoctorCommand() *cobra.Command {
	var jsonOutput, strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local OpenExit runtime readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := runDoctor()
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				writeDoctorReport(cmd.OutOrStdout(), report)
			}
			if report.Status == "failed" || (strict && report.Status == "warning") {
				return fmt.Errorf("doctor status: %s", report.Status)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable doctor report")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as failures")
	return cmd
}

func runDoctor() DoctorReport {
	report := DoctorReport{Status: "passed"}
	report.Checks = append(report.Checks,
		versionMetadataCheck(),
		schemaBundleCheck(),
		optionalToolCheck("promtool", "Prometheus rule syntax validation"),
		optionalToolCheck("kubeconform", "Kubernetes manifest validation"),
	)
	sort.SliceStable(report.Checks, func(i, j int) bool { return report.Checks[i].Name < report.Checks[j].Name })
	for _, check := range report.Checks {
		switch check.Status {
		case "failed":
			report.Status = "failed"
		case "warning":
			if report.Status == "passed" {
				report.Status = "warning"
			}
		}
	}
	return report
}

func versionMetadataCheck() DoctorCheck {
	var missing []string
	for key, value := range map[string]string{
		"name":    version.Name,
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return DoctorCheck{Name: "version-metadata", Status: "failed", Message: "missing " + strings.Join(missing, ", ")}
	}
	var unstamped []string
	if version.Commit == "unknown" {
		unstamped = append(unstamped, "commit")
	}
	if version.Date == "unknown" {
		unstamped = append(unstamped, "date")
	}
	if len(unstamped) > 0 {
		return DoctorCheck{
			Name:    "version-metadata",
			Status:  "warning",
			Message: fmt.Sprintf("unstamped %s in %s %s %s/%s", strings.Join(unstamped, ", "), version.Name, version.Version, runtime.GOOS, runtime.GOARCH),
		}
	}
	return DoctorCheck{
		Name:    "version-metadata",
		Status:  "passed",
		Message: fmt.Sprintf("%s %s %s/%s commit=%s date=%s", version.Name, version.Version, runtime.GOOS, runtime.GOARCH, version.Commit, version.Date),
	}
}

func schemaBundleCheck() DoctorCheck {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	entries, err := publicschemas.FS.ReadDir(".")
	if err != nil {
		return DoctorCheck{Name: "schema-bundle", Status: "failed", Message: err.Error()}
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		data, err := publicschemas.FS.ReadFile(entry.Name())
		if err != nil {
			return DoctorCheck{Name: "schema-bundle", Status: "failed", Message: err.Error()}
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return DoctorCheck{Name: "schema-bundle", Status: "failed", Message: fmt.Sprintf("%s: %v", entry.Name(), err)}
		}
		if err := compiler.AddResource(entry.Name(), document); err != nil {
			return DoctorCheck{Name: "schema-bundle", Status: "failed", Message: fmt.Sprintf("%s: %v", entry.Name(), err)}
		}
		names = append(names, entry.Name())
	}
	for _, name := range names {
		if _, err := compiler.Compile(name); err != nil {
			return DoctorCheck{Name: "schema-bundle", Status: "failed", Message: fmt.Sprintf("%s: %v", name, err)}
		}
	}
	return DoctorCheck{Name: "schema-bundle", Status: "passed", Message: fmt.Sprintf("%d embedded schema(s) compiled", len(names))}
}

func optionalToolCheck(name, purpose string) DoctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return DoctorCheck{Name: name, Status: "warning", Message: purpose + " unavailable; install " + name + " for stronger validation"}
	}
	return DoctorCheck{Name: name, Status: "passed", Message: path}
}

func writeDoctorReport(w io.Writer, report DoctorReport) {
	_, _ = fmt.Fprintf(w, "status: %s\n", report.Status)
	for _, check := range report.Checks {
		if check.Message == "" {
			_, _ = fmt.Fprintf(w, "%s: %s\n", check.Name, check.Status)
			continue
		}
		_, _ = fmt.Fprintf(w, "%s: %s - %s\n", check.Name, check.Status, check.Message)
	}
}
