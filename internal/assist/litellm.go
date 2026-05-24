package assist

import (
	"context"
	"fmt"
)

type LiteLLMProvider struct{}

func (LiteLLMProvider) Name() string { return "litellm" }

func (LiteLLMProvider) Complete(context.Context, Request) (*Response, error) {
	return nil, fmt.Errorf("litellm provider is intentionally not implemented in v0.1; use provider noop")
}
