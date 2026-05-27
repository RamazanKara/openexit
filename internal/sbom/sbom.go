package sbom

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

const (
	BOMFormat   = "CycloneDX"
	SpecVersion = "1.5"
)

type Options struct {
	Name      string
	Version   string
	Commit    string
	Date      string
	Timestamp time.Time
}

type BOM struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Metadata    Metadata    `json:"metadata"`
	Components  []Component `json:"components"`
}

type Metadata struct {
	Timestamp  string     `json:"timestamp"`
	Tools      []Tool     `json:"tools"`
	Component  Component  `json:"component"`
	Properties []Property `json:"properties,omitempty"`
}

type Tool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Component struct {
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	BOMRef     string     `json:"bom-ref,omitempty"`
	PURL       string     `json:"purl,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func Generate(opts Options) (*BOM, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, errors.New("build info unavailable")
	}
	return FromBuildInfo(info, opts), nil
}

func FromBuildInfo(info *debug.BuildInfo, opts Options) *BOM {
	name := nonEmpty(opts.Name, "openexit")
	version := nonEmpty(opts.Version, moduleVersion(info.Main.Version))
	timestamp := opts.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	main := Component{
		Type:    "application",
		Name:    name,
		Version: version,
		BOMRef:  "pkg:golang/" + nonEmpty(info.Main.Path, name),
		PURL:    purl(nonEmpty(info.Main.Path, name), version),
	}
	var properties []Property
	if opts.Commit != "" {
		properties = append(properties, Property{Name: "openexit.build.commit", Value: opts.Commit})
	}
	if opts.Date != "" {
		properties = append(properties, Property{Name: "openexit.build.date", Value: opts.Date})
	}
	if info.GoVersion != "" {
		properties = append(properties, Property{Name: "go.version", Value: info.GoVersion})
	}
	main.Properties = append(main.Properties, properties...)

	components := make([]Component, 0, len(info.Deps))
	for _, dep := range info.Deps {
		if dep == nil || strings.TrimSpace(dep.Path) == "" {
			continue
		}
		module := *dep
		properties := []Property{}
		if module.Replace != nil {
			properties = append(properties,
				Property{Name: "go.module.replace.path", Value: module.Replace.Path},
				Property{Name: "go.module.replace.version", Value: module.Replace.Version},
			)
		}
		if module.Sum != "" {
			properties = append(properties, Property{Name: "go.module.sum", Value: module.Sum})
		}
		components = append(components, Component{
			Type:       "library",
			Name:       module.Path,
			Version:    moduleVersion(module.Version),
			BOMRef:     bomRef(module.Path, moduleVersion(module.Version)),
			PURL:       purl(module.Path, moduleVersion(module.Version)),
			Properties: properties,
		})
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Name == components[j].Name {
			return components[i].Version < components[j].Version
		}
		return components[i].Name < components[j].Name
	})
	return &BOM{
		BOMFormat:   BOMFormat,
		SpecVersion: SpecVersion,
		Version:     1,
		Metadata: Metadata{
			Timestamp:  timestamp.Format(time.RFC3339),
			Tools:      []Tool{{Vendor: "OpenExit", Name: "openexit", Version: version}},
			Component:  main,
			Properties: properties,
		},
		Components: components,
	}
}

func Write(path string, bom *BOM) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("SBOM output path is required")
	}
	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func purl(path, version string) string {
	if version == "" {
		return "pkg:golang/" + path
	}
	return "pkg:golang/" + path + "@" + version
}

func bomRef(path, version string) string {
	sum := sha256.Sum256([]byte(path + "@" + version))
	return "pkg:golang/" + path + "#" + hex.EncodeToString(sum[:8])
}

func moduleVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "(devel)" {
		return ""
	}
	return version
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func Summary(bom *BOM) string {
	if bom == nil {
		return ""
	}
	return fmt.Sprintf("%s %s with %d component(s)", bom.BOMFormat, bom.SpecVersion, len(bom.Components))
}
