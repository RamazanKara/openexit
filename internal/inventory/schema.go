package inventory

import (
	"errors"
	"fmt"
	"strings"
)

func Validate(inv *Inventory) error {
	var problems []string
	if inv.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if inv.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if strings.TrimSpace(inv.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}
	if strings.TrimSpace(inv.Source.Type) == "" {
		problems = append(problems, "source.type is required")
	}
	for _, dashboard := range inv.Assets.Dashboards {
		if strings.TrimSpace(dashboard.ID) == "" || strings.TrimSpace(dashboard.Title) == "" {
			problems = append(problems, "each dashboard requires id and title")
		}
		if strings.TrimSpace(dashboard.EvidenceRef) == "" {
			problems = append(problems, "each dashboard requires evidenceRef")
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if strings.TrimSpace(monitor.ID) == "" || strings.TrimSpace(monitor.Name) == "" {
			problems = append(problems, "each monitor requires id and name")
		}
		if strings.TrimSpace(monitor.EvidenceRef) == "" {
			problems = append(problems, "each monitor requires evidenceRef")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
