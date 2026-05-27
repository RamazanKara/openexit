package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

type argoCDApplication struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   argoCDMetadata `yaml:"metadata"`
	Spec       argoCDAppSpec  `yaml:"spec"`
	rawText    string         `yaml:"-"`
}

type argoCDMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
}

type argoCDAppSpec struct {
	Project     string         `yaml:"project"`
	Source      argoCDSource   `yaml:"source"`
	Destination argoCDDest     `yaml:"destination"`
	SyncPolicy  map[string]any `yaml:"syncPolicy"`
}

type argoCDSource struct {
	RepoURL        string `yaml:"repoURL"`
	TargetRevision string `yaml:"targetRevision"`
	Path           string `yaml:"path"`
}

type argoCDDest struct {
	Server    string `yaml:"server"`
	Namespace string `yaml:"namespace"`
}

func addArgoCDCandidateChecks(projectDir string, add func(string, string, string, bool)) {
	path := filepath.Join(projectDir, "generated-config", "argocd", "grafana-stack-application.candidate.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		add("argocd-candidate", "failed", err.Error(), true)
		return
	}
	app, err := readArgoCDCandidate(path)
	if err != nil {
		add("argocd-candidate", "failed", err.Error(), true)
		return
	}
	var problems []string
	problems = append(problems, validateArgoCDCandidateApplication(app)...)
	problems = append(problems, validateArgoCDCandidateREADME(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("argocd-candidate", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("argocd-candidate", "passed", "", true)
}

func readArgoCDCandidate(path string) (*argoCDApplication, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var app argoCDApplication
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	app.rawText = string(data)
	return &app, nil
}

func validateArgoCDCandidateApplication(app *argoCDApplication) []string {
	var problems []string
	if app == nil {
		return []string{"ArgoCD candidate application is empty"}
	}
	if inventory.ContainsSecret(app.rawText) {
		problems = append(problems, "grafana-stack-application.candidate.yaml contains secret-like content")
	}
	if app.APIVersion != "argoproj.io/v1alpha1" {
		problems = append(problems, fmt.Sprintf("apiVersion must be argoproj.io/v1alpha1, got %q", app.APIVersion))
	}
	if app.Kind != "Application" {
		problems = append(problems, fmt.Sprintf("kind must be Application, got %q", app.Kind))
	}
	if app.Metadata.Name == "" {
		problems = append(problems, "metadata.name is required")
	}
	if app.Metadata.Namespace != "argocd" {
		problems = append(problems, fmt.Sprintf("metadata.namespace must be argocd, got %q", app.Metadata.Namespace))
	}
	expectedLabels := map[string]string{
		"generated-by":       "openexit",
		"openexit-candidate": "true",
	}
	for key, want := range expectedLabels {
		if got := app.Metadata.Labels[key]; got != want {
			problems = append(problems, fmt.Sprintf("metadata.labels.%s must be %q, got %q", key, want, got))
		}
	}
	if app.Spec.Project == "" {
		problems = append(problems, "spec.project is required")
	}
	if app.Spec.Source.RepoURL == "" {
		problems = append(problems, "spec.source.repoURL is required")
	} else if !strings.Contains(app.Spec.Source.RepoURL, "example.invalid") {
		problems = append(problems, "spec.source.repoURL must remain a placeholder unless project-specific configuration is supported")
	}
	if app.Spec.Source.TargetRevision == "" {
		problems = append(problems, "spec.source.targetRevision is required")
	}
	if app.Spec.Source.Path == "" {
		problems = append(problems, "spec.source.path is required")
	}
	if app.Spec.Destination.Server != "https://kubernetes.default.svc" {
		problems = append(problems, fmt.Sprintf("spec.destination.server must be https://kubernetes.default.svc, got %q", app.Spec.Destination.Server))
	}
	if app.Spec.Destination.Namespace == "" {
		problems = append(problems, "spec.destination.namespace is required")
	}
	if app.Spec.SyncPolicy == nil {
		problems = append(problems, "spec.syncPolicy must be present and empty by default")
	} else if _, ok := app.Spec.SyncPolicy["automated"]; ok {
		problems = append(problems, "spec.syncPolicy.automated must not be set on generated candidates")
	}
	return problems
}

func validateArgoCDCandidateREADME(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "argocd", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/argocd/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"intentionally unsynced", "placeholder repository URL", "Review namespaces", "sync strategy"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/argocd/README.md: missing %q", required)}
		}
	}
	return nil
}
