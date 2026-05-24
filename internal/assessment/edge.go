package assessment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type EdgeAnalyzer struct{}

func (EdgeAnalyzer) Name() string { return "edge-varnish-haproxy-coraza" }

func (EdgeAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	if inv.Source.Type != "edge" && len(inv.Assets.DNSRecords) == 0 {
		return nil, nil
	}
	var findings []Finding
	for _, record := range inv.Assets.DNSRecords {
		if strings.Contains(record.Name, "*") {
			findings = append(findings, edgeFinding(record.EvidenceRef, "edge.dns.wildcard-record.001", "medium", "Wildcard DNS record needs routing review", fmt.Sprintf("%s is a wildcard DNS record.", record.Name), []string{"dns-record:" + record.ID}, "Map wildcard routing explicitly in HAProxy and confirm no unintended hostnames are exposed."))
		}
		if record.TTL > 3600 {
			findings = append(findings, edgeFinding(record.EvidenceRef, "edge.dns.ttl-high.001", "medium", "DNS TTL is high for cutover", fmt.Sprintf("%s has TTL %d.", record.Name, record.TTL), []string{"dns-record:" + record.ID}, "Lower TTL before cutover or document the rollback delay."))
		}
		if !record.Proxied {
			findings = append(findings, edgeFinding(record.EvidenceRef, "edge.dns.unproxied.001", "low", "DNS record bypasses edge controls", fmt.Sprintf("%s is not proxied by the source edge provider.", record.Name), []string{"dns-record:" + record.ID}, "Confirm whether the hostname should be included in target edge routing."))
		}
	}
	for _, rule := range inv.Assets.WAFRules {
		if !rule.Enabled {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.waf.disabled-rule.001", "medium", "WAF rule is disabled", fmt.Sprintf("%s is disabled.", rule.Name), []string{"waf-rule:" + rule.ID}, "Confirm whether the rule should be dropped or re-enabled in Coraza."))
		}
		if isObserveOnlyAction(rule.Action) {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.waf.observe-only.001", "high", "WAF rule is observe-only", fmt.Sprintf("%s uses action %s.", rule.Name, rule.Action), []string{"waf-rule:" + rule.ID}, "Decide whether the target Coraza rule should block, allow, or remain in detection mode."))
		}
		if rule.Managed {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.waf.managed-rule.001", "medium", "Managed WAF rule needs translation review", fmt.Sprintf("%s is marked as provider-managed.", rule.Name), []string{"waf-rule:" + rule.ID}, "Map the managed rule to a Coraza/OWASP CRS equivalent and test false-positive behavior."))
		}
		if isChallengeAction(rule.Action) {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.waf.challenge-action.001", "medium", "Challenge action lacks direct Coraza equivalent", fmt.Sprintf("%s uses challenge-like action %s.", rule.Name, rule.Action), []string{"waf-rule:" + rule.ID}, "Replace challenge behavior with an explicit allow/block/rate-limit design."))
		}
	}
	for _, rule := range inv.Assets.CacheRules {
		if strings.EqualFold(rule.Action, "bypass") {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.cache.bypass-rule.001", "medium", "Cache bypass rule requires parity review", fmt.Sprintf("%s bypasses cache.", rule.Name), []string{"cache-rule:" + rule.ID}, "Preserve bypass behavior in VCL and confirm dynamic routes are not cached."))
		}
		if len(rule.CacheKey) > 2 {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.cache.custom-key.001", "medium", "Custom cache key needs VCL review", fmt.Sprintf("%s has cache key parts %s.", rule.Name, strings.Join(rule.CacheKey, ", ")), []string{"cache-rule:" + rule.ID}, "Translate cache-key dimensions explicitly and test cache-hit parity."))
		}
		if rule.EdgeTTL == 0 && strings.EqualFold(rule.Action, "cache") {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.cache.ttl-missing.001", "medium", "Cache rule has no edge TTL", fmt.Sprintf("%s has no edge TTL.", rule.Name), []string{"cache-rule:" + rule.ID}, "Set a Varnish TTL candidate or document origin-driven caching."))
		}
	}
	for _, redirect := range inv.Assets.Redirects {
		if redirect.StatusCode == 302 || redirect.StatusCode == 307 {
			findings = append(findings, edgeFinding(redirect.EvidenceRef, "edge.redirect.temporary.001", "low", "Temporary redirect needs intent review", fmt.Sprintf("%s uses HTTP %d.", redirect.Name, redirect.StatusCode), []string{"redirect:" + redirect.ID}, "Confirm whether the redirect should remain temporary in HAProxy."))
		}
		if strings.Contains(redirect.Source, "*") || strings.Contains(redirect.Target, "$") {
			findings = append(findings, edgeFinding(redirect.EvidenceRef, "edge.redirect.dynamic-pattern.001", "medium", "Dynamic redirect pattern needs mapping review", fmt.Sprintf("%s maps %s to %s.", redirect.Name, redirect.Source, redirect.Target), []string{"redirect:" + redirect.ID}, "Translate wildcard capture behavior into HAProxy ACLs and redirect rules."))
		}
	}
	for _, origin := range inv.Assets.Origins {
		if !strings.EqualFold(origin.Scheme, "https") {
			findings = append(findings, edgeFinding(origin.EvidenceRef, "edge.origin.non-https.001", "high", "Origin uses non-HTTPS scheme", fmt.Sprintf("%s uses scheme %s.", origin.Hostname, origin.Scheme), []string{"origin:" + origin.ID}, "Prefer HTTPS origins or document why cleartext origin traffic is acceptable."))
		}
		if !origin.TLSVerify {
			findings = append(findings, edgeFinding(origin.EvidenceRef, "edge.origin.tls-verify-disabled.001", "high", "Origin TLS verification is disabled", fmt.Sprintf("%s does not verify origin TLS.", origin.Hostname), []string{"origin:" + origin.ID}, "Enable certificate verification in HAProxy or document a temporary exception."))
		}
		if !origin.HealthCheck {
			findings = append(findings, edgeFinding(origin.EvidenceRef, "edge.origin.health-check-missing.001", "medium", "Origin health check is missing", fmt.Sprintf("%s has no captured health check.", origin.Hostname), []string{"origin:" + origin.ID}, "Add an HAProxy health check before production traffic is migrated."))
		}
	}
	for _, setting := range inv.Assets.TLSSettings {
		if strings.EqualFold(setting.Mode, "flexible") {
			findings = append(findings, edgeFinding(setting.EvidenceRef, "edge.tls.flexible-mode.001", "high", "TLS mode is flexible", fmt.Sprintf("%s uses flexible TLS mode.", setting.Hostname), []string{"tls-setting:" + setting.ID}, "Use full end-to-end TLS with origin verification in the target design."))
		}
		if tlsVersionBelow(setting.MinVersion, "1.2") {
			findings = append(findings, edgeFinding(setting.EvidenceRef, "edge.tls.minimum-version-low.001", "high", "TLS minimum version is below 1.2", fmt.Sprintf("%s allows TLS %s.", setting.Hostname, setting.MinVersion), []string{"tls-setting:" + setting.ID}, "Raise the target minimum TLS version to at least 1.2 unless a legacy exception is approved."))
		}
		if !setting.HSTSEnabled {
			findings = append(findings, edgeFinding(setting.EvidenceRef, "edge.tls.hsts-disabled.001", "medium", "HSTS is disabled", fmt.Sprintf("%s does not have HSTS enabled.", setting.Hostname), []string{"tls-setting:" + setting.ID}, "Decide whether HSTS should be enabled in HAProxy response headers after compatibility review."))
		}
	}
	for _, rule := range inv.Assets.BotRules {
		if !rule.Enabled {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.bot.disabled-rule.001", "medium", "Bot rule is disabled", fmt.Sprintf("%s is disabled.", rule.Name), []string{"bot-rule:" + rule.ID}, "Confirm whether bot handling should be removed or replaced."))
		}
		if isChallengeAction(rule.Action) {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.bot.challenge-action.001", "medium", "Bot challenge action needs replacement", fmt.Sprintf("%s uses action %s.", rule.Name, rule.Action), []string{"bot-rule:" + rule.ID}, "Replace provider challenge behavior with explicit rate limiting, block rules, or application controls."))
		}
	}
	for _, rule := range inv.Assets.PageRules {
		if !rule.Enabled {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.page-rule.disabled.001", "low", "Page rule is disabled", fmt.Sprintf("Page rule %s is disabled.", rule.ID), []string{"page-rule:" + rule.ID}, "Decide whether the disabled page rule should be ignored."))
		}
		if len(rule.Actions) > 1 {
			findings = append(findings, edgeFinding(rule.EvidenceRef, "edge.page-rule.multi-action.001", "medium", "Page rule has multiple actions", fmt.Sprintf("Page rule %s has actions %s.", rule.ID, strings.Join(rule.Actions, ", ")), []string{"page-rule:" + rule.ID}, "Split coupled page-rule behavior into explicit VCL and HAProxy candidates."))
		}
	}
	return findings, nil
}

func edgeFinding(evidenceRef, id, severity, title, description string, affected []string, recommendation string) Finding {
	return Finding{
		ID:             id,
		Severity:       severity,
		Title:          title,
		Description:    description,
		AffectedAssets: affected,
		EvidenceRefs:   []string{evidenceRef},
		Recommendation: recommendation,
	}
}

func isObserveOnlyAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	return action == "log" || action == "count" || action == "simulate"
}

func isChallengeAction(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	return strings.Contains(action, "challenge") || action == "captcha"
}

func tlsVersionBelow(actual, minimum string) bool {
	actualFloat, err := strconv.ParseFloat(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(actual)), "tls"), 64)
	if err != nil {
		return false
	}
	minimumFloat, err := strconv.ParseFloat(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(minimum)), "tls"), 64)
	if err != nil {
		return false
	}
	return actualFloat < minimumFloat
}
