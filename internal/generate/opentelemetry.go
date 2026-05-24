package generate

import (
	"os"
	"path/filepath"
)

func OpenTelemetry(ctx *Context) error {
	dir := filepath.Join(ctx.ProjectDir, "generated-config", "opentelemetry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	config := `# OpenExit candidate OpenTelemetry Collector configuration.
# Review endpoints, TLS, authentication, sizing, and deployment topology before production use.
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  batch:

exporters:
  prometheusremotewrite:
    endpoint: "http://mimir-nginx/api/v1/push"
  otlp/tempo:
    endpoint: "tempo-distributor:4317"
    tls:
      insecure: true

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheusremotewrite]
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/tempo]
`
	if err := os.WriteFile(filepath.Join(dir, "collector.candidate.yaml"), []byte(config), 0o644); err != nil {
		return err
	}
	readme := "# OpenTelemetry Collector Candidate\n\nThis is a sketch only. Replace placeholder endpoints, add authentication through your secret manager, and validate sizing before production use.\n"
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644)
}
