package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	publicschemas "github.com/RamazanKara/openexit/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

type schemaManifest struct {
	name        string
	schemaFile  string
	manifestRel string
	required    bool
}

var schemaManifests = []schemaManifest{
	{name: "project", schemaFile: "openexit.project.schema.json", manifestRel: "openexit.yaml", required: true},
	{name: "inventory-yaml", schemaFile: "openexit.inventory.schema.json", manifestRel: "inventory/openexit.inventory.yaml", required: true},
	{name: "inventory-json", schemaFile: "openexit.inventory.schema.json", manifestRel: "inventory/openexit.inventory.json"},
	{name: "assessment-yaml", schemaFile: "openexit.assessment.schema.json", manifestRel: "assessment/openexit.assessment.yaml", required: true},
	{name: "assessment-json", schemaFile: "openexit.assessment.schema.json", manifestRel: "assessment/openexit.assessment.json"},
	{name: "mapping-yaml", schemaFile: "openexit.mapping.schema.json", manifestRel: "mapping/openexit.mapping.yaml", required: true},
	{name: "mapping-json", schemaFile: "openexit.mapping.schema.json", manifestRel: "mapping/openexit.mapping.json"},
	{name: "migration-plan-yaml", schemaFile: "openexit.plan.schema.json", manifestRel: "assessment/openexit.migration-plan.yaml"},
	{name: "migration-plan-json", schemaFile: "openexit.plan.schema.json", manifestRel: "assessment/openexit.migration-plan.json"},
}

type schemaValidator struct {
	schemas map[string]*jsonschema.Schema
}

func newSchemaValidator() (*schemaValidator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	entries, err := publicschemas.FS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		data, err := publicschemas.FS.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse schema %s: %w", entry.Name(), err)
		}
		if err := compiler.AddResource(entry.Name(), document); err != nil {
			return nil, fmt.Errorf("load schema %s: %w", entry.Name(), err)
		}
	}
	out := &schemaValidator{schemas: map[string]*jsonschema.Schema{}}
	for _, name := range schemaFileNames() {
		schema, err := compiler.Compile(name)
		if err != nil {
			return nil, fmt.Errorf("compile schema %s: %w", name, err)
		}
		out.schemas[name] = schema
	}
	return out, nil
}

func schemaFileNames() []string {
	seen := map[string]struct{}{
		"openexit.validation.schema.json":         {},
		"openexit.datadog-inventory.schema.json":  {},
		"openexit.datadog-plan.schema.json":       {},
		"openexit.datadog-validation.schema.json": {},
		"openexit.migration-bundle.schema.json":   {},
	}
	for _, manifest := range schemaManifests {
		seen[manifest.schemaFile] = struct{}{}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func addSchemaChecks(projectDir string, validator *schemaValidator, add func(string, string, string, bool)) {
	for _, manifest := range schemaManifests {
		path := filepath.Join(projectDir, filepath.FromSlash(manifest.manifestRel))
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) && !manifest.required {
				continue
			}
			add("jsonschema-"+manifest.name, "failed", err.Error(), true)
			continue
		}
		if err := validator.ValidateFile(manifest.schemaFile, path); err != nil {
			add("jsonschema-"+manifest.name, "failed", manifest.manifestRel+": "+formatSchemaError(err), true)
			continue
		}
		add("jsonschema-"+manifest.name, "passed", "", true)
	}
}

func addValidationReportSchemaCheck(validator *schemaValidator, report *Report, add func(string, string, string, bool)) {
	if err := validator.ValidateValue("openexit.validation.schema.json", report); err != nil {
		add("jsonschema-validation-report", "failed", formatSchemaError(err), true)
		return
	}
	add("jsonschema-validation-report", "passed", "", true)
}

func (v *schemaValidator) ValidateFile(schemaFile, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var instance any
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		instance, err = yamlDocumentToJSONValue(data)
	default:
		instance, err = jsonschema.UnmarshalJSON(bytes.NewReader(data))
	}
	if err != nil {
		return err
	}
	return v.validate(schemaFile, instance)
}

func (v *schemaValidator) ValidateValue(schemaFile string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return v.validate(schemaFile, instance)
}

func (v *schemaValidator) validate(schemaFile string, instance any) error {
	schema, ok := v.schemas[schemaFile]
	if !ok {
		return fmt.Errorf("schema %s is not compiled", schemaFile)
	}
	return schema.Validate(instance)
}

func yamlDocumentToJSONValue(data []byte) (any, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized, err := normalizeYAMLValue(raw)
	if err != nil {
		return nil, err
	}
	jsonData, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(jsonData))
}

func normalizeYAMLValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			keyText, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("yaml object key %v is not a string", key)
			}
			normalized, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			out[keyText] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			normalized, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case time.Time:
		return typed.UTC().Format(time.RFC3339), nil
	default:
		return value, nil
	}
}

func formatSchemaError(err error) string {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return err.Error()
	}
	var messages []string
	collectSchemaOutputMessages(validationErr.BasicOutput(), &messages)
	if len(messages) == 0 {
		return validationErr.Error()
	}
	const maxMessages = 3
	if len(messages) > maxMessages {
		messages = append(messages[:maxMessages], fmt.Sprintf("%d more schema error(s)", len(messages)-maxMessages))
	}
	return strings.Join(messages, "; ")
}

func collectSchemaOutputMessages(unit *jsonschema.OutputUnit, messages *[]string) {
	if unit == nil {
		return
	}
	if unit.Error != nil {
		location := unit.InstanceLocation
		if location == "" {
			location = "/"
		}
		*messages = append(*messages, location+": "+unit.Error.String())
	}
	for i := range unit.Errors {
		collectSchemaOutputMessages(&unit.Errors[i], messages)
	}
}
