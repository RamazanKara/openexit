# GitHub Enterprise To Forgejo

This is a fixture-complete assessment path. It makes no live API calls.

Collected fixture metadata:

- repositories
- teams
- branch protections
- Actions workflows
- secrets metadata only
- runners
- deploy keys metadata
- GitHub Apps metadata

Generated reports:

- `forgejo-migration-assessment.md`
- `ci-compatibility-report.md`
- `branch-protection-mapping.md`
- `runner-migration-plan.md`
- `repository-ownership-report.md`

The assessment flags branch protection gaps, GitHub-hosted runner dependency, GitHub-specific Actions usage, unknown secret consumers, offline runners, write-capable deploy keys, GitHub Pages/Packages/Discussions usage, and GitHub App webhook review.
