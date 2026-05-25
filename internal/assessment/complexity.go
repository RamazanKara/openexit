package assessment

import (
	"fmt"

	"github.com/RamazanKara/openexit/internal/inventory"
)

func ScoreInventory(inv *inventory.Inventory, findings []Finding) Score {
	value := 0
	var drivers []string
	if inv.Summary.Dashboards > 50 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d dashboards", inv.Summary.Dashboards))
	}
	if inv.Summary.Monitors > 100 {
		value += 15
		drivers = append(drivers, fmt.Sprintf("%d monitors", inv.Summary.Monitors))
	}
	if inv.Summary.Repositories > 100 {
		value += 15
		drivers = append(drivers, fmt.Sprintf("%d repositories", inv.Summary.Repositories))
	}
	if inv.Summary.ActionsWorkflows > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d Actions workflows require compatibility review", inv.Summary.ActionsWorkflows))
	}
	if inv.Summary.Secrets > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d secret metadata entries require migration", inv.Summary.Secrets))
	}
	if inv.Summary.IdentityApps > 20 {
		value += 15
		drivers = append(drivers, fmt.Sprintf("%d identity applications", inv.Summary.IdentityApps))
	}
	if inv.Summary.IdentityPolicies > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d identity policies require mapping", inv.Summary.IdentityPolicies))
	}
	if inv.Summary.BreakGlassAccounts == 0 && inv.Source.Type == "identity" {
		value += 10
		drivers = append(drivers, "break-glass account metadata missing")
	}
	if inv.Summary.WAFRules > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d WAF rules require Coraza mapping", inv.Summary.WAFRules))
	}
	if inv.Summary.CacheRules > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d cache rules require VCL parity review", inv.Summary.CacheRules))
	}
	if inv.Summary.Origins > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d origins require HAProxy routing review", inv.Summary.Origins))
	}
	if inv.Summary.AIModelUsageClasses > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d AI model usage classes require evaluation", inv.Summary.AIModelUsageClasses))
	}
	if inv.Summary.AITokenVolumes > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d AI token volume profiles require vLLM sizing", inv.Summary.AITokenVolumes))
	}
	if inv.Summary.AISensitivePromptCategories > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d sensitive prompt categories require controls review", inv.Summary.AISensitivePromptCategories))
	}
	if inv.Summary.AIToolUsages > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d AI tool usages require gateway policy review", inv.Summary.AIToolUsages))
	}
	if inv.Summary.AIFallbackBehaviors == 0 && inv.Source.Type == "ai-provider" {
		value += 10
		drivers = append(drivers, "AI fallback behavior metadata missing")
	}
	unsupported := 0
	for _, dashboard := range inv.Assets.Dashboards {
		unsupported += dashboard.Widgets.Unsupported
	}
	if unsupported > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d unsupported dashboard widgets", unsupported))
	}
	manual := severityCount(findings, "high")
	if manual > 0 {
		value += 20
		drivers = append(drivers, fmt.Sprintf("%d high-severity manual review findings", manual))
	}
	missingOwner := 0
	for _, finding := range findings {
		if finding.ID == "dd.ownership.missing.001" || finding.ID == "dd.monitor.missing-owner.001" || finding.ID == "identity.application.owner-missing.001" || finding.ID == "identity.group.owner-missing.001" || finding.ID == "ai.ownership.missing.001" {
			missingOwner++
		}
	}
	if missingOwner > 0 {
		value += 15
		drivers = append(drivers, fmt.Sprintf("%d assets missing clear ownership", missingOwner))
	}
	for _, finding := range findings {
		if finding.ID == "dd.cost.high-cardinality.001" {
			value += 15
			drivers = append(drivers, "high-cardinality tags detected")
			break
		}
	}
	if inv.Summary.SLOs > 0 {
		value += 10
		drivers = append(drivers, fmt.Sprintf("%d SLOs require migration review", inv.Summary.SLOs))
	}
	if hasFinding(findings, "dd.cost.retention-unknown.001") {
		value += 10
		drivers = append(drivers, "retention requirements unknown")
	}
	if hasFinding(findings, "dd.cost.log-volume-unknown.001") || hasFinding(findings, "dd.cost.trace-volume-unknown.001") {
		value += 10
		drivers = append(drivers, "logs or traces volume requirements unknown")
	}
	if hasFinding(findings, "dd.dashboard.unknown-data-source.001") || hasFinding(findings, "dd.dashboard.template-variable-complexity.001") {
		value += 10
		drivers = append(drivers, "dashboard data source or variable mapping requires review")
	}
	if hasFinding(findings, "migration.notification-routing-review-required.001") {
		value += 5
		drivers = append(drivers, "notification routing requires review")
	}
	if value > 100 {
		value = 100
	}
	level := "low"
	if value > 70 {
		level = "high"
	} else if value > 30 {
		level = "medium"
	}
	automation := "high"
	manualReview := "low"
	if level == "medium" {
		automation = "medium"
		manualReview = "medium"
	}
	if level == "high" {
		automation = "medium"
		manualReview = "high"
	}
	if len(drivers) == 0 {
		drivers = append(drivers, "small inventory with no high-risk findings")
	}
	return Score{
		Value:               value,
		Level:               level,
		AutomationPotential: automation,
		ManualReview:        manualReview,
		Drivers:             drivers,
	}
}

func hasFinding(findings []Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
