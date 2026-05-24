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
	for _, repo := range inv.Assets.Repositories {
		if strings.TrimSpace(repo.Name) == "" {
			problems = append(problems, "each repository requires name")
		}
		if strings.TrimSpace(repo.EvidenceRef) == "" {
			problems = append(problems, "each repository requires evidenceRef")
		}
	}
	for _, app := range inv.Assets.IdentityApps {
		if strings.TrimSpace(app.ID) == "" || strings.TrimSpace(app.Name) == "" || strings.TrimSpace(app.Protocol) == "" {
			problems = append(problems, "each identity application requires id, name, and protocol")
		}
		if strings.TrimSpace(app.EvidenceRef) == "" {
			problems = append(problems, "each identity application requires evidenceRef")
		}
	}
	for _, group := range inv.Assets.IdentityGroups {
		if strings.TrimSpace(group.ID) == "" || strings.TrimSpace(group.Name) == "" {
			problems = append(problems, "each identity group requires id and name")
		}
		if strings.TrimSpace(group.EvidenceRef) == "" {
			problems = append(problems, "each identity group requires evidenceRef")
		}
	}
	for _, policy := range inv.Assets.IdentityPolicies {
		if strings.TrimSpace(policy.ID) == "" || strings.TrimSpace(policy.Name) == "" {
			problems = append(problems, "each identity policy requires id and name")
		}
		if strings.TrimSpace(policy.EvidenceRef) == "" {
			problems = append(problems, "each identity policy requires evidenceRef")
		}
	}
	for _, setting := range inv.Assets.MFASettings {
		if strings.TrimSpace(setting.Name) == "" {
			problems = append(problems, "each MFA setting requires name")
		}
		if strings.TrimSpace(setting.EvidenceRef) == "" {
			problems = append(problems, "each MFA setting requires evidenceRef")
		}
	}
	for _, account := range inv.Assets.BreakGlassAccounts {
		if strings.TrimSpace(account.Username) == "" {
			problems = append(problems, "each break-glass account requires username")
		}
		if strings.TrimSpace(account.EvidenceRef) == "" {
			problems = append(problems, "each break-glass account requires evidenceRef")
		}
	}
	for _, record := range inv.Assets.DNSRecords {
		if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Type) == "" {
			problems = append(problems, "each DNS record requires id, name, and type")
		}
		if strings.TrimSpace(record.EvidenceRef) == "" {
			problems = append(problems, "each DNS record requires evidenceRef")
		}
	}
	for _, rule := range inv.Assets.WAFRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Name) == "" {
			problems = append(problems, "each WAF rule requires id and name")
		}
		if strings.TrimSpace(rule.EvidenceRef) == "" {
			problems = append(problems, "each WAF rule requires evidenceRef")
		}
	}
	for _, rule := range inv.Assets.CacheRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Name) == "" {
			problems = append(problems, "each cache rule requires id and name")
		}
		if strings.TrimSpace(rule.EvidenceRef) == "" {
			problems = append(problems, "each cache rule requires evidenceRef")
		}
	}
	for _, redirect := range inv.Assets.Redirects {
		if strings.TrimSpace(redirect.ID) == "" || strings.TrimSpace(redirect.Source) == "" || strings.TrimSpace(redirect.Target) == "" {
			problems = append(problems, "each redirect requires id, source, and target")
		}
		if strings.TrimSpace(redirect.EvidenceRef) == "" {
			problems = append(problems, "each redirect requires evidenceRef")
		}
	}
	for _, origin := range inv.Assets.Origins {
		if strings.TrimSpace(origin.ID) == "" || strings.TrimSpace(origin.Hostname) == "" {
			problems = append(problems, "each origin requires id and hostname")
		}
		if strings.TrimSpace(origin.EvidenceRef) == "" {
			problems = append(problems, "each origin requires evidenceRef")
		}
	}
	for _, setting := range inv.Assets.TLSSettings {
		if strings.TrimSpace(setting.ID) == "" || strings.TrimSpace(setting.Hostname) == "" {
			problems = append(problems, "each TLS setting requires id and hostname")
		}
		if strings.TrimSpace(setting.EvidenceRef) == "" {
			problems = append(problems, "each TLS setting requires evidenceRef")
		}
	}
	for _, rule := range inv.Assets.BotRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Name) == "" {
			problems = append(problems, "each bot rule requires id and name")
		}
		if strings.TrimSpace(rule.EvidenceRef) == "" {
			problems = append(problems, "each bot rule requires evidenceRef")
		}
	}
	for _, rule := range inv.Assets.PageRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Target) == "" {
			problems = append(problems, "each page rule requires id and target")
		}
		if strings.TrimSpace(rule.EvidenceRef) == "" {
			problems = append(problems, "each page rule requires evidenceRef")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
