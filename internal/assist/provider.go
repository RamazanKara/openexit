package assist

import "context"

type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
	Task         string            `json:"task"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"systemPrompt"`
	Input        map[string]any    `json:"input"`
	Metadata     map[string]string `json:"metadata"`
}

type Response struct {
	Text        string            `json:"text"`
	Metadata    map[string]string `json:"metadata"`
	Warnings    []string          `json:"warnings"`
	WasRedacted bool              `json:"wasRedacted"`
}
