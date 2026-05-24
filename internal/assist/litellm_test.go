package assist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiteLLMProviderComplete(t *testing.T) {
	var sawAuth bool
	var payload chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer test-key" {
			sawAuth = true
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Summarized risks."}}]}`))
	}))
	defer server.Close()

	provider := LiteLLMProvider{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client()}
	resp, err := provider.Complete(context.Background(), Request{
		Task:         "summarize",
		Model:        "qwen",
		SystemPrompt: SummarySystemPrompt,
		Input:        map[string]any{"content": Redact("token=supersecretvalue")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawAuth {
		t.Fatal("expected Authorization header")
	}
	if payload.Model != "qwen" || len(payload.Messages) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if strings.Contains(payload.Messages[1].Content, "supersecretvalue") {
		t.Fatalf("payload was not redacted: %s", payload.Messages[1].Content)
	}
	if !strings.HasPrefix(resp.Text, "> AI-assisted draft.") {
		t.Fatalf("review header missing: %s", resp.Text)
	}
	if !resp.WasRedacted {
		t.Fatal("expected WasRedacted")
	}
}

func TestLiteLLMProviderRequiresModel(t *testing.T) {
	provider := LiteLLMProvider{BaseURL: "http://localhost:4000"}
	if _, err := provider.Complete(context.Background(), Request{}); err == nil {
		t.Fatal("expected model error")
	}
}

func TestEnsureReviewHeader(t *testing.T) {
	text := EnsureReviewHeader("hello")
	if strings.Count(text, "AI-assisted draft") != 1 {
		t.Fatalf("unexpected header count: %s", text)
	}
	again := EnsureReviewHeader(text)
	if strings.Count(again, "AI-assisted draft") != 1 {
		t.Fatalf("header duplicated: %s", again)
	}
}
