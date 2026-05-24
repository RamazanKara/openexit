package assist

import "context"

type NoopProvider struct{}

func (NoopProvider) Name() string { return "noop" }

func (NoopProvider) Complete(_ context.Context, req Request) (*Response, error) {
	return &Response{
		Text: "> AI-assisted draft.\n> Source of truth: structured OpenExit inventory and assessment.\n> Review required before operational use.\n\nNo external AI provider was used. This deterministic placeholder confirms the assist pipeline is disabled by default.",
		Metadata: map[string]string{
			"provider": "noop",
			"task":     req.Task,
		},
		Warnings:    []string{"noop provider does not summarize or transform content"},
		WasRedacted: true,
	}, nil
}
