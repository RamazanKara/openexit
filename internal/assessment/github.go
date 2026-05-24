package assessment

import (
	"context"
	"fmt"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type GitHubEnterpriseAnalyzer struct{}

func (GitHubEnterpriseAnalyzer) Name() string { return "github-enterprise-forgejo" }

func (GitHubEnterpriseAnalyzer) Analyze(_ context.Context, inv *inventory.Inventory) ([]Finding, error) {
	if inv.Source.Type != "github-enterprise" && len(inv.Assets.Repositories) == 0 {
		return nil, nil
	}
	var findings []Finding
	protected := protectedBranches(inv)
	workflowsByRepo := workflowsByRepository(inv)
	for _, repo := range inv.Assets.Repositories {
		if !repo.Archived && repo.DefaultBranch != "" && !protected[repo.Name+"/"+repo.DefaultBranch] {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.branch-protection.missing.001", "high", "Default branch protection is missing", fmt.Sprintf("%s does not have captured branch protection for %s.", repo.Name, repo.DefaultBranch), []string{"repository:" + repo.Name}, "Create an equivalent Forgejo branch protection rule before migration."))
		}
		if !repo.HasCODEOWNERS {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.repository.codeowners-missing.001", "medium", "Repository has no CODEOWNERS metadata", fmt.Sprintf("%s does not have captured CODEOWNERS metadata.", repo.Name), []string{"repository:" + repo.Name}, "Add or map repository ownership before migration sign-off."))
		}
		if repo.ActionsEnabled && len(workflowsByRepo[repo.Name]) == 0 {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.actions.workflow-inventory-missing.001", "medium", "Actions are enabled but workflows were not captured", fmt.Sprintf("%s has Actions enabled but no workflow metadata in inventory.", repo.Name), []string{"repository:" + repo.Name}, "Collect workflow metadata before CI compatibility review."))
		}
		if repo.UsesGitHubPages {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.repository.pages-needs-review.001", "medium", "GitHub Pages usage needs replacement planning", fmt.Sprintf("%s uses GitHub Pages.", repo.Name), []string{"repository:" + repo.Name}, "Plan Forgejo Pages or an external static hosting replacement."))
		}
		if repo.UsesGitHubPackages {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.repository.packages-needs-review.001", "medium", "GitHub Packages usage needs registry planning", fmt.Sprintf("%s uses GitHub Packages.", repo.Name), []string{"repository:" + repo.Name}, "Map package publishing and consumption to the target registry."))
		}
		if repo.UsesGitHubDiscussions {
			findings = append(findings, githubFinding(repo.EvidenceRef, "ghe.repository.discussions-needs-review.001", "low", "GitHub Discussions usage needs feature review", fmt.Sprintf("%s uses GitHub Discussions.", repo.Name), []string{"repository:" + repo.Name}, "Decide whether discussions are migrated, archived, or replaced."))
		}
	}
	for _, protection := range inv.Assets.BranchProtections {
		if protection.AllowsForcePushes {
			findings = append(findings, githubFinding(protection.EvidenceRef, "ghe.branch-protection.force-push.001", "high", "Branch protection allows force pushes", fmt.Sprintf("%s/%s allows force pushes.", protection.Repository, protection.Branch), []string{"branch-protection:" + protection.Repository + "/" + protection.Branch}, "Disable force pushes or document why this exception must be preserved."))
		}
		if protection.RequiredReviews == 0 {
			findings = append(findings, githubFinding(protection.EvidenceRef, "ghe.branch-protection.review-gap.001", "medium", "Branch protection has no review requirement", fmt.Sprintf("%s/%s has no required review count.", protection.Repository, protection.Branch), []string{"branch-protection:" + protection.Repository + "/" + protection.Branch}, "Map review requirements into Forgejo branch protection policy."))
		}
		if !protection.RequireCodeOwnerReview {
			findings = append(findings, githubFinding(protection.EvidenceRef, "ghe.branch-protection.codeowner-review-gap.001", "medium", "Code owner review is not required", fmt.Sprintf("%s/%s does not require code owner review.", protection.Repository, protection.Branch), []string{"branch-protection:" + protection.Repository + "/" + protection.Branch}, "Review whether Forgejo protection should require code-owner-equivalent approval."))
		}
	}
	for _, workflow := range inv.Assets.ActionsWorkflows {
		if workflow.UsesGitHubHosted {
			findings = append(findings, githubFinding(workflow.EvidenceRef, "ghe.actions.github-hosted-runner.001", "high", "Workflow depends on GitHub-hosted runners", fmt.Sprintf("%s uses GitHub-hosted runners.", workflow.Path), []string{"workflow:" + workflow.Repository + ":" + workflow.Path}, "Plan Forgejo runner capacity and labels before migration."))
		}
		for _, action := range workflow.Actions {
			if isGitHubSpecificAction(action) {
				findings = append(findings, githubFinding(workflow.EvidenceRef, "ghe.actions.github-specific-action.001", "medium", "Workflow uses GitHub-specific action", fmt.Sprintf("%s references %s.", workflow.Path, action), []string{"workflow:" + workflow.Repository + ":" + workflow.Path}, "Review action compatibility under Forgejo Actions or replace the step."))
				break
			}
		}
	}
	for _, secret := range inv.Assets.Secrets {
		if len(secret.Consumers) == 0 {
			findings = append(findings, githubFinding(secret.EvidenceRef, "ghe.secrets.consumer-unknown.001", "medium", "Secret consumer metadata is missing", fmt.Sprintf("Secret metadata %s has no captured consumers.", secret.Name), []string{"secret-metadata:" + secret.Scope + ":" + secret.Name}, "Identify consuming workflows before migrating secret metadata."))
		}
	}
	for _, runner := range inv.Assets.Runners {
		if !runner.Online {
			findings = append(findings, githubFinding(runner.EvidenceRef, "ghe.runner.offline.001", "medium", "Runner is offline", fmt.Sprintf("Runner %s is offline.", runner.Name), []string{"runner:" + runner.Name}, "Confirm runner replacement or remove the dependency before Forgejo cutover."))
		}
	}
	for _, key := range inv.Assets.DeployKeys {
		if !key.ReadOnly {
			findings = append(findings, githubFinding(key.EvidenceRef, "ghe.deploy-key.write-access.001", "high", "Deploy key has write access", fmt.Sprintf("Deploy key %s on %s has write access.", key.Title, key.Repository), []string{"deploy-key:" + key.Repository + ":" + key.Title}, "Rotate or replace write-capable deploy keys during migration."))
		}
	}
	for _, app := range inv.Assets.GitHubApps {
		if app.WebhookEnabled {
			findings = append(findings, githubFinding(app.EvidenceRef, "ghe.github-app.webhook-review.001", "medium", "GitHub App webhook integration needs review", fmt.Sprintf("GitHub App %s has webhooks enabled.", app.Name), []string{"github-app:" + app.Name}, "Map app permissions and webhook endpoints to Forgejo-compatible integrations."))
		}
	}
	return findings, nil
}

func githubFinding(evidenceRef, id, severity, title, description string, affected []string, recommendation string) Finding {
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

func protectedBranches(inv *inventory.Inventory) map[string]bool {
	out := map[string]bool{}
	for _, protection := range inv.Assets.BranchProtections {
		out[protection.Repository+"/"+protection.Branch] = true
	}
	return out
}

func workflowsByRepository(inv *inventory.Inventory) map[string][]inventory.ActionsWorkflow {
	out := map[string][]inventory.ActionsWorkflow{}
	for _, workflow := range inv.Assets.ActionsWorkflows {
		out[workflow.Repository] = append(out[workflow.Repository], workflow)
	}
	return out
}

func isGitHubSpecificAction(action string) bool {
	switch action {
	case "actions/cache", "actions/upload-artifact", "actions/download-artifact", "github/codeql-action/init", "github/codeql-action/analyze":
		return true
	default:
		return false
	}
}
