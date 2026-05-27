# GitHub Enterprise To Forgejo

This path supports local fixture assessment and read-only live GitHub/GitHub Enterprise inventory collection.

Collected metadata:

- repositories
- teams
- branch protections
- Actions workflows
- secrets metadata only
- runners
- deploy keys metadata
- GitHub Apps metadata

The live collector gathers repository, team, branch protection, Actions workflow, secret metadata, runner, deploy key, and GitHub App installation metadata. It does not collect secret values and it does not write to GitHub. GitHub App installation collection is best-effort because GitHub requires an organization owner token with organization administration read access.

Live collection:

```bash
export GITHUB_TOKEN=<read-only-token>
openexit init ./ghe-live --source github-enterprise --target forgejo
openexit collect github --project ./ghe-live --owner acme --token-env GITHUB_TOKEN
```

For GitHub Enterprise Server:

```bash
openexit collect github --project ./ghe-live --owner acme --base-url https://github.example.com/api/v3
```

Use `--repo` one or more times to limit collection to selected repositories. Repository names can be passed as `name` or `owner/name`.

Generated reports:

- `forgejo-migration-assessment.md`
- `ci-compatibility-report.md`
- `branch-protection-mapping.md`
- `runner-migration-plan.md`
- `repository-ownership-report.md`

Generated target candidate:

- `generated-config/forgejo/migration-candidate.yaml`

The assessment flags branch protection gaps, GitHub-hosted runner dependency, GitHub-specific Actions usage, unknown secret consumers, offline runners, write-capable deploy keys, GitHub Pages/Packages/Discussions usage, and GitHub App webhook review.
