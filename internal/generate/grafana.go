package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type grafanaDashboard struct {
	Title         string            `json:"title"`
	Tags          []string          `json:"tags"`
	Timezone      string            `json:"timezone"`
	SchemaVersion int               `json:"schemaVersion"`
	Panels        []grafanaPanel    `json:"panels"`
	Templating    map[string]any    `json:"templating"`
	Annotations   map[string]any    `json:"annotations"`
	OpenExit      map[string]string `json:"openexit"`
}

type grafanaPanel struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	GridPos     map[string]int `json:"gridPos"`
	Description string         `json:"description"`
	Targets     []any          `json:"targets"`
}

func Grafana(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "grafana", "dashboards")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var unsupported []string
	for _, dashboard := range ctx.Inventory.Assets.Dashboards {
		gd := grafanaFromDashboard(dashboard)
		data, err := json.MarshalIndent(gd, "", "  ")
		if err != nil {
			return err
		}
		name := inventory.Slug(dashboard.Title) + ".candidate.json"
		if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0o644); err != nil {
			return err
		}
		if dashboard.Widgets.Unsupported > 0 {
			unsupported = append(unsupported, fmt.Sprintf("- %s (%s): %d unsupported widget(s)", dashboard.Title, dashboard.ID, dashboard.Widgets.Unsupported))
		}
	}
	readme := "# Grafana Dashboard Candidates\n\nGenerated dashboards are candidates only and are not production-ready. Review source metadata, layout, queries, and unsupported widgets before import.\n"
	if len(unsupported) > 0 {
		readme += "\n## Unsupported Widgets\n" + stringsJoin(unsupported, "\n") + "\n"
	}
	return os.WriteFile(filepath.Join(ctx.ProjectDir, "generated-config", "grafana", "README.md"), []byte(readme), 0o644)
}

func grafanaFromDashboard(d inventory.Dashboard) grafanaDashboard {
	tags := append([]string{"openexit", "candidate", "source:datadog"}, d.Tags...)
	panels := make([]grafanaPanel, 0, len(d.Queries))
	for i, query := range d.Queries {
		panels = append(panels, grafanaPanel{
			Type:  "timeseries",
			Title: fmt.Sprintf("Candidate panel %d", i+1),
			GridPos: map[string]int{
				"h": 8,
				"w": 12,
				"x": (i % 2) * 12,
				"y": (i / 2) * 8,
			},
			Description: "OpenExit candidate panel. Review Datadog query before production use: " + query.Raw,
			Targets: []any{
				map[string]any{
					"refId":                 fmt.Sprintf("A%d", i+1),
					"expr":                  "vector(0)",
					"legendFormat":          "manual review required",
					"openexit_source_query": query.Raw,
				},
			},
		})
	}
	return grafanaDashboard{
		Title:         d.Title,
		Tags:          tags,
		Timezone:      "browser",
		SchemaVersion: 39,
		Panels:        panels,
		Templating:    map[string]any{"list": []any{}},
		Annotations:   map[string]any{"list": []any{}},
		OpenExit: map[string]string{
			"candidate":         "true",
			"productionReady":   "false",
			"source":            "datadog",
			"sourceDashboardID": d.ID,
			"evidenceRef":       d.EvidenceRef,
		},
	}
}

func stringsJoin(values []string, sep string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += sep
		}
		out += value
	}
	return out
}
