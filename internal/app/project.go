package app

import (
	"fmt"
	"os"
	"path/filepath"
)

var requiredProjectDirs = []string{
	"inventory",
	"assessment",
	"generated-config",
	"evidence",
	"validation",
}

type ProjectStatus struct {
	ProjectDir string
	ConfigOK   bool
	Missing    []string
}

func InitProject(projectDir string) error {
	return InitProjectWithEndpoints(projectDir, defaultSource, defaultTargetForSource(defaultSource))
}

func InitProjectWithEndpoints(projectDir, source, target string) error {
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	for _, dir := range requiredProjectDirs {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	configPath := filepath.Join(projectDir, projectFileName)
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", configPath, err)
	}
	cfg := DefaultProjectConfig(projectDir)
	if source != "" {
		cfg.Source.Type = source
	}
	if target != "" {
		cfg.Target.Type = target
	} else {
		cfg.Target.Type = defaultTargetForSource(cfg.Source.Type)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid project options: %w", err)
	}
	return WriteProjectConfig(projectDir, cfg)
}

func CheckProject(projectDir string) (*ProjectStatus, error) {
	status := &ProjectStatus{ProjectDir: projectDir}
	if _, err := LoadProjectConfig(projectDir); err != nil {
		return status, err
	}
	status.ConfigOK = true
	for _, dir := range requiredProjectDirs {
		path := filepath.Join(projectDir, dir)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			status.Missing = append(status.Missing, dir)
		}
	}
	return status, nil
}

func RequiredProjectDirs() []string {
	out := make([]string, len(requiredProjectDirs))
	copy(out, requiredProjectDirs)
	return out
}
