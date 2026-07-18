package datadogplan

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	secretKeyPattern   = regexp.MustCompile(`(?i)(api.?key|app.?key|token|password|secret|private.?key|access.?key|client.?secret|credential)`)
	secretValuePattern = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9_\-./+=]{12,}|dd[a-z0-9]{30,}|-----BEGIN [A-Z ]*PRIVATE KEY-----|\b(api[_-]?key|app[_-]?key|access[_-]?key|client[_-]?secret|password|token|secret)\s*[:=]\s*[^\s&]{4,}`)
)

func RedactValue(value any) any {
	return redactValue(value, false)
}

func redactValue(value any, secretContext bool) any {
	switch typed := value.(type) {
	case map[string]any:
		context := secretContext || boolValue(typed["is_secret"]) || boolValue(typed["secure"])
		if kind, _ := typed["type"].(string); strings.Contains(strings.ToLower(kind), "secret") {
			context = true
		}
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			redact := secretKeyPattern.MatchString(key) || (context && secretPayloadKey(key))
			if _, metadata := child.(bool); metadata && (strings.EqualFold(key, "is_secret") || strings.EqualFold(key, "secure")) {
				redact = false
			}
			if redact && child != nil {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactValue(child, context)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = redactValue(child, secretContext)
		}
		return out
	case string:
		if secretValuePattern.MatchString(typed) {
			return "[REDACTED]"
		}
		return typed
	default:
		return value
	}
}

func secretPayloadKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	switch key {
	case "value", "default", "default_value", "content", "payload":
		return true
	default:
		return false
	}
}

func RedactJSON(data []byte) ([]byte, error) {
	var value any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(RedactValue(value), "", "  ")
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
