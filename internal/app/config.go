package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion        = "openexit.dev/v1alpha1"
	ProjectKind       = "Project"
	defaultSource     = "datadog"
	defaultTarget     = "grafana-lgtm"
	projectFileName   = "openexit.yaml"
	defaultAIProvider = "noop"
)

var supportedTargetBySource = map[string]string{
	"datadog":           "grafana-lgtm",
	"github-enterprise": "forgejo",
	"identity":          "keycloak-zitadel",
	"edge":              "varnish-haproxy-coraza",
	"ai-provider":       "vllm-litellm",
}

type ProjectConfig struct {
	APIVersion string          `json:"apiVersion" yaml:"apiVersion"`
	Kind       string          `json:"kind" yaml:"kind"`
	Metadata   ProjectMetadata `json:"metadata" yaml:"metadata"`
	Source     Endpoint        `json:"source" yaml:"source"`
	Target     Endpoint        `json:"target" yaml:"target"`
	Policy     ProjectPolicy   `json:"policy" yaml:"policy"`
	Assist     AssistConfig    `json:"assist,omitempty" yaml:"assist,omitempty"`
}

type ProjectMetadata struct {
	Name string `json:"name" yaml:"name"`
}

type Endpoint struct {
	Type string `json:"type" yaml:"type"`
	Site string `json:"site,omitempty" yaml:"site,omitempty"`
}

type ProjectPolicy struct {
	AllowNetworkWrites    bool `json:"allowNetworkWrites" yaml:"allowNetworkWrites"`
	AllowProductionWrites bool `json:"allowProductionWrites" yaml:"allowProductionWrites"`
	AllowAI               bool `json:"allowAI" yaml:"allowAI"`
}

type AssistConfig struct {
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	Provider              string `json:"provider" yaml:"provider"`
	Model                 string `json:"model" yaml:"model"`
	AllowExternalProvider bool   `json:"allowExternalProvider" yaml:"allowExternalProvider"`
}

func DefaultProjectConfig(projectDir string) ProjectConfig {
	name := filepath.Base(projectDir)
	if name == "." || name == string(filepath.Separator) {
		name = "openexit-project"
	}
	return ProjectConfig{
		APIVersion: APIVersion,
		Kind:       ProjectKind,
		Metadata: ProjectMetadata{
			Name: name,
		},
		Source: Endpoint{Type: defaultSource},
		Target: Endpoint{Type: defaultTarget},
		Policy: ProjectPolicy{
			AllowNetworkWrites:    false,
			AllowProductionWrites: false,
			AllowAI:               false,
		},
		Assist: AssistConfig{
			Enabled:               false,
			Provider:              defaultAIProvider,
			Model:                 "",
			AllowExternalProvider: false,
		},
	}
}

func defaultTargetForSource(source string) string {
	source = strings.TrimSpace(source)
	if target, ok := supportedTargetBySource[source]; ok {
		return target
	}
	return ""
}

func LoadProjectConfig(projectDir string) (*ProjectConfig, error) {
	path := filepath.Join(projectDir, projectFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}
	return &cfg, nil
}

func WriteProjectConfig(projectDir string, cfg ProjectConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectDir, projectFileName), data, 0o644)
}

func (cfg ProjectConfig) Validate() error {
	var problems []string
	sourceType := strings.TrimSpace(cfg.Source.Type)
	targetType := strings.TrimSpace(cfg.Target.Type)
	if cfg.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if cfg.Kind != ProjectKind {
		problems = append(problems, fmt.Sprintf("kind must be %q", ProjectKind))
	}
	if strings.TrimSpace(cfg.Metadata.Name) == "" {
		problems = append(problems, "metadata.name is required")
	}
	if sourceType == "" {
		problems = append(problems, "source.type is required")
	}
	if targetType == "" {
		problems = append(problems, "target.type is required")
	}
	if sourceType != "" && targetType != "" {
		expectedTarget, ok := supportedTargetBySource[sourceType]
		if !ok {
			problems = append(problems, fmt.Sprintf("source.type %q is not supported", sourceType))
		} else if targetType != expectedTarget {
			problems = append(problems, fmt.Sprintf("target.type must be %q for source.type %q", expectedTarget, sourceType))
		}
	}
	if cfg.Policy.AllowNetworkWrites {
		problems = append(problems, "policy.allowNetworkWrites must remain false")
	}
	if cfg.Policy.AllowProductionWrites {
		problems = append(problems, "policy.allowProductionWrites must remain false")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
