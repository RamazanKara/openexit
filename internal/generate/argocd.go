package generate

import (
	"os"
	"path/filepath"
)

func ArgoCD(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "argocd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	app := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: grafana-stack
  namespace: argocd
  labels:
    generated-by: openexit
    openexit-candidate: "true"
spec:
  project: observability
  source:
    repoURL: https://git.example.invalid/platform/manifests
    targetRevision: main
    path: clusters/production/observability/grafana-stack
  destination:
    server: https://kubernetes.default.svc
    namespace: observability
  syncPolicy: {}
`
	if err := os.WriteFile(filepath.Join(dir, "grafana-stack-application.candidate.yaml"), []byte(app), 0o644); err != nil {
		return err
	}
	readme := "# ArgoCD Candidate\n\nThis manifest is intentionally unsynced and uses a placeholder repository URL. Review namespaces, project policy, and sync strategy before use.\n"
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}
