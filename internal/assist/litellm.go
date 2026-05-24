package assist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultLiteLLMBaseURL = "http://localhost:4000"

type LiteLLMProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func (LiteLLMProvider) Name() string { return "litellm" }

func NewLiteLLMProviderFromEnv() LiteLLMProvider {
	baseURL := strings.TrimSpace(os.Getenv("OPENEXIT_LITELLM_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultLiteLLMBaseURL
	}
	return LiteLLMProvider{
		BaseURL: baseURL,
		APIKey:  os.Getenv("OPENEXIT_LITELLM_API_KEY"),
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p LiteLLMProvider) Complete(ctx context.Context, req Request) (*Response, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("litellm model is required")
	}
	endpoint, err := p.endpoint()
	if err != nil {
		return nil, err
	}
	input, err := json.MarshalIndent(req.Input, "", "  ")
	if err != nil {
		return nil, err
	}
	payload := chatCompletionRequest{
		Model: req.Model,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: string(input)},
		},
		Temperature: 0.2,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpClient := p.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("litellm request failed with %s: %s", resp.Status, sanitizeResponseBody(respBody))
	}
	var completion chatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("litellm response did not include assistant content")
	}
	return &Response{
		Text: EnsureReviewHeader(completion.Choices[0].Message.Content),
		Metadata: map[string]string{
			"provider": "litellm",
			"model":    req.Model,
		},
		WasRedacted: true,
	}, nil
}

func (p LiteLLMProvider) endpoint() (string, error) {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = defaultLiteLLMBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("litellm base URL must use http or https")
	}
	trimmed := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(trimmed, "/chat/completions"):
		return trimmed, nil
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/chat/completions", nil
	default:
		return trimmed + "/v1/chat/completions", nil
	}
}

func sanitizeResponseBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		text = text[:500]
	}
	return Redact(text)
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
