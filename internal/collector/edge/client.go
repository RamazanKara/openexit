package edge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/inventory"
	"github.com/RamazanKara/openexit/internal/version"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type apiError struct {
	Path       string
	Status     string
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("cloudflare API %s returned %s: %s", e.Path, e.Status, e.Body)
}

type cloudflareMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareEnvelope[T any] struct {
	Success    bool                `json:"success"`
	Result     T                   `json:"result"`
	Errors     []cloudflareMessage `json:"errors"`
	Messages   []cloudflareMessage `json:"messages"`
	ResultInfo struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func NewClient(baseURL, token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("cloudflare token is required")
	}
	normalized, err := cloudflareBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: normalized,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func cloudflareBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "https://api.cloudflare.com/client/v4"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("cloudflare base URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("cloudflare base URL requires a host")
	}
	return strings.TrimRight(raw, "/"), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	if query == nil {
		query = url.Values{}
	}
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", version.Name+"/"+version.Version)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &apiError{Path: path, Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
			time.Sleep(retryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &apiError{Path: path, Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
		}
		if out != nil {
			if err := json.Unmarshal(body, out); err != nil {
				return nil, err
			}
		}
		return body, nil
	}
	return nil, lastErr
}

func (c *Client) getResult(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	var envelope cloudflareEnvelope[json.RawMessage]
	body, err := c.get(ctx, path, query, &envelope)
	if err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, fmt.Errorf("cloudflare API %s returned unsuccessful response: %s", path, cloudflareMessages(envelope.Errors))
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return nil, err
		}
	}
	return body, nil
}

func getPagedResult[T any](ctx context.Context, client *Client, path string, query url.Values) ([]T, error) {
	const pageSize = 100
	var out []T
	for page := 1; ; page++ {
		q := cloneValues(query)
		q.Set("per_page", strconv.Itoa(pageSize))
		q.Set("page", strconv.Itoa(page))
		var envelope cloudflareEnvelope[[]T]
		if _, err := client.get(ctx, path, q, &envelope); err != nil {
			return nil, err
		}
		if !envelope.Success {
			return nil, fmt.Errorf("cloudflare API %s returned unsuccessful response: %s", path, cloudflareMessages(envelope.Errors))
		}
		out = append(out, envelope.Result...)
		if envelope.ResultInfo.TotalPages > 0 {
			if page >= envelope.ResultInfo.TotalPages {
				break
			}
			continue
		}
		if len(envelope.Result) < pageSize {
			break
		}
	}
	return out, nil
}

func cloneValues(input url.Values) url.Values {
	out := url.Values{}
	for key, values := range input {
		out[key] = append([]string{}, values...)
	}
	return out
}

func retryAfter(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(attempt+1) * 2 * time.Second
}

func apiStatus(err error) int {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

func cloudflareMessages(messages []cloudflareMessage) string {
	if len(messages) == 0 {
		return "no error details"
	}
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		parts = append(parts, fmt.Sprintf("%d %s", message.Code, message.Message))
	}
	return strings.Join(parts, "; ")
}

func sanitizeBody(body []byte) string {
	body = bytes.TrimSpace(inventory.RedactBytes(body))
	if len(body) > 300 {
		body = body[:300]
	}
	return string(body)
}

func envSecret(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("environment variable name is required")
	}
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is not set", name)
	}
	return value, nil
}
