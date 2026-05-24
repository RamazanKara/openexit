package generate

var markdownArtifacts = []string{
	"assessment",
	"risk-register",
	"manual-review",
	"cost-drivers",
	"target-architecture",
	"acceptance-criteria",
	"rollback-plan",
	"runbook",
	"restore-drill-checklist",
	"alert-shadowing-plan",
}

var githubMarkdownArtifacts = []string{
	"forgejo-migration-assessment",
	"ci-compatibility-report",
	"branch-protection-mapping",
	"runner-migration-plan",
	"repository-ownership-report",
}

var identityMarkdownArtifacts = []string{
	"identity-migration-risk-register",
	"break-glass-checklist",
	"identity-cutover-plan",
	"identity-rollback-plan",
}

var edgeMarkdownArtifacts = []string{
	"cache-parity-report",
	"waf-enforcement-risk-report",
}

func MarkdownArtifacts() []string {
	out := make([]string, len(markdownArtifacts))
	copy(out, markdownArtifacts)
	return out
}
