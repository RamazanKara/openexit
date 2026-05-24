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

func MarkdownArtifacts() []string {
	out := make([]string, len(markdownArtifacts))
	copy(out, markdownArtifacts)
	return out
}
