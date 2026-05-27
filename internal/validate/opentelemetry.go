package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/openexit/internal/inventory"
	"gopkg.in/yaml.v3"
)

type otelCollectorConfig struct {
	Receivers  map[string]any       `yaml:"receivers"`
	Processors map[string]any       `yaml:"processors"`
	Exporters  map[string]any       `yaml:"exporters"`
	Service    otelCollectorService `yaml:"service"`
	rawText    string               `yaml:"-"`
}

type otelCollectorService struct {
	Pipelines map[string]otelPipeline `yaml:"pipelines"`
}

type otelPipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

func addOpenTelemetryCandidateChecks(projectDir string, inv *inventory.Inventory, add func(string, string, string, bool)) {
	path := filepath.Join(projectDir, "generated-config", "opentelemetry", "collector.candidate.yaml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		add("opentelemetry-candidate", "failed", err.Error(), true)
		return
	}
	config, err := readOpenTelemetryCandidate(path)
	if err != nil {
		add("opentelemetry-candidate", "failed", err.Error(), true)
		return
	}
	var problems []string
	problems = append(problems, validateOpenTelemetryCandidateConfig(config, inv)...)
	problems = append(problems, validateOpenTelemetryCandidateREADME(projectDir)...)
	if len(problems) > 0 {
		sort.Strings(problems)
		add("opentelemetry-candidate", "failed", strings.Join(problems, "; "), true)
		return
	}
	add("opentelemetry-candidate", "passed", "", true)
}

func readOpenTelemetryCandidate(path string) (*otelCollectorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config otelCollectorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	config.rawText = string(data)
	return &config, nil
}

func validateOpenTelemetryCandidateConfig(config *otelCollectorConfig, inv *inventory.Inventory) []string {
	var problems []string
	if config == nil {
		return []string{"collector candidate is empty"}
	}
	if !strings.Contains(config.rawText, "OpenExit candidate") {
		problems = append(problems, "collector.candidate.yaml must identify itself as an OpenExit candidate")
	}
	if !strings.Contains(config.rawText, "before production use") {
		problems = append(problems, "collector.candidate.yaml must warn that production review is required")
	}
	if inventory.ContainsSecret(config.rawText) {
		problems = append(problems, "collector.candidate.yaml contains secret-like content")
	}
	problems = append(problems, requireMapKeys("receivers", config.Receivers, "otlp")...)
	problems = append(problems, requireMapKeys("processors", config.Processors, "memory_limiter", "batch")...)
	problems = append(problems, requireMapKeys("exporters", config.Exporters, "prometheusremotewrite", "otlp/tempo")...)
	if !hasOTLPProtocol(config.Receivers, "grpc") {
		problems = append(problems, "receivers.otlp.protocols.grpc is required")
	}
	if !hasOTLPProtocol(config.Receivers, "http") {
		problems = append(problems, "receivers.otlp.protocols.http is required")
	}
	if endpoint := nestedString(config.Exporters, "prometheusremotewrite", "endpoint"); endpoint == "" {
		problems = append(problems, "exporters.prometheusremotewrite.endpoint is required")
	} else if !strings.Contains(endpoint, "mimir") {
		problems = append(problems, "exporters.prometheusremotewrite.endpoint should point at the Mimir placeholder")
	}
	if endpoint := nestedString(config.Exporters, "otlp/tempo", "endpoint"); endpoint == "" {
		problems = append(problems, "exporters.otlp/tempo.endpoint is required")
	} else if !strings.Contains(endpoint, "tempo") {
		problems = append(problems, "exporters.otlp/tempo.endpoint should point at the Tempo placeholder")
	}
	if inv == nil || shouldRequireMetricsPipeline(inv) {
		problems = append(problems, validateOTelPipeline("metrics", config.Service.Pipelines["metrics"], otelPipelineExpectation{
			Receivers:  []string{"otlp"},
			Processors: []string{"memory_limiter", "batch"},
			Exporters:  []string{"prometheusremotewrite"},
		})...)
	}
	if inv == nil || shouldRequireTracesPipeline(inv) {
		problems = append(problems, validateOTelPipeline("traces", config.Service.Pipelines["traces"], otelPipelineExpectation{
			Receivers:  []string{"otlp"},
			Processors: []string{"memory_limiter", "batch"},
			Exporters:  []string{"otlp/tempo"},
		})...)
	}
	return problems
}

type otelPipelineExpectation struct {
	Receivers  []string
	Processors []string
	Exporters  []string
}

func validateOTelPipeline(name string, pipeline otelPipeline, expected otelPipelineExpectation) []string {
	var problems []string
	if len(pipeline.Receivers) == 0 && len(pipeline.Processors) == 0 && len(pipeline.Exporters) == 0 {
		return []string{"service.pipelines." + name + " is required"}
	}
	for _, receiver := range expected.Receivers {
		if !containsString(pipeline.Receivers, receiver) {
			problems = append(problems, fmt.Sprintf("service.pipelines.%s.receivers must include %s", name, receiver))
		}
	}
	for _, processor := range expected.Processors {
		if !containsString(pipeline.Processors, processor) {
			problems = append(problems, fmt.Sprintf("service.pipelines.%s.processors must include %s", name, processor))
		}
	}
	for _, exporter := range expected.Exporters {
		if !containsString(pipeline.Exporters, exporter) {
			problems = append(problems, fmt.Sprintf("service.pipelines.%s.exporters must include %s", name, exporter))
		}
	}
	return problems
}

func validateOpenTelemetryCandidateREADME(projectDir string) []string {
	path := filepath.Join(projectDir, "generated-config", "opentelemetry", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"generated-config/opentelemetry/README.md: " + err.Error()}
	}
	readme := string(data)
	for _, required := range []string{"sketch only", "placeholder endpoints", "authentication", "validate sizing", "production use"} {
		if !strings.Contains(readme, required) {
			return []string{fmt.Sprintf("generated-config/opentelemetry/README.md: missing %q", required)}
		}
	}
	return nil
}

func requireMapKeys(name string, values map[string]any, keys ...string) []string {
	var problems []string
	if values == nil {
		return []string{name + " section is required"}
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			problems = append(problems, fmt.Sprintf("%s.%s is required", name, key))
		}
	}
	return problems
}

func hasOTLPProtocol(receivers map[string]any, protocol string) bool {
	otlp, ok := receivers["otlp"].(map[string]any)
	if !ok {
		return false
	}
	protocols, ok := otlp["protocols"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = protocols[protocol]
	return ok
}

func nestedString(values map[string]any, key, nestedKey string) string {
	child, ok := values[key].(map[string]any)
	if !ok {
		return ""
	}
	value, ok := child[nestedKey].(string)
	if !ok {
		return ""
	}
	return value
}

func shouldRequireMetricsPipeline(inv *inventory.Inventory) bool {
	return inv.Summary.UniqueMetrics > 0 || inv.Summary.Dashboards > 0 || inv.Summary.Monitors > 0 || len(inv.Assets.Metrics) > 0
}

func shouldRequireTracesPipeline(inv *inventory.Inventory) bool {
	if inv.Summary.Dashboards > 0 || inv.Summary.Monitors > 0 {
		return true
	}
	for _, metric := range inv.Assets.Metrics {
		if telemetryMetricLooksTraceRelated(metric.Name) {
			return true
		}
	}
	for _, dashboard := range inv.Assets.Dashboards {
		for _, query := range dashboard.Queries {
			if telemetryMetricLooksTraceRelated(query.Raw) {
				return true
			}
		}
	}
	for _, monitor := range inv.Assets.Monitors {
		if telemetryMetricLooksTraceRelated(monitor.Query) {
			return true
		}
	}
	return false
}

func telemetryMetricLooksTraceRelated(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "trace.") || strings.Contains(value, "traces.") || strings.Contains(value, "apm")
}
