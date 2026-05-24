package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RamazanKara/openexit/internal/assist"
)

func TestEnsureAssistAllowedRequiresExplicitExternalOptIn(t *testing.T) {
	cfg := DefaultProjectConfig("demo")
	if err := ensureAssistAllowed(&cfg, "noop"); err != nil {
		t.Fatal(err)
	}
	if err := ensureAssistAllowed(&cfg, "litellm"); err == nil {
		t.Fatal("expected litellm to require explicit opt-in")
	}
	cfg.Policy.AllowAI = true
	cfg.Assist.Enabled = true
	cfg.Assist.AllowExternalProvider = true
	cfg.Assist.Provider = "litellm"
	if err := ensureAssistAllowed(&cfg, "litellm"); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAssistAuditInputDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "summary.ai.md")
	req := assist.Request{
		Task:  "summarize",
		Model: "noop",
		Input: map[string]any{"content": assist.Redact("password=supersecretvalue")},
	}
	if err := writeAssistAuditInput(out, "noop", req); err != nil {
		t.Fatal(err)
	}
	auditPath := filepath.Join(dir, "summary.input.redacted.json")
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || assist.Redact(string(data)) != string(data) {
		t.Fatalf("audit file is empty or not redacted: %s", string(data))
	}
	if err := writeAssistAuditInput(out, "noop", req); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}
