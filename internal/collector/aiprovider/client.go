package aiprovider

import (
	"bytes"
	"context"
	"encoding/json"
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

type OpenAIClient struct {
	baseURL        string
	token          string
	organizationID string
	projectID      string
	http           *http.Client
}

type openAIAPIError struct {
	Path       string
	Status     string
	StatusCode int
	Body       string
}

func (e *openAIAPIError) Error() string {
	return fmt.Sprintf("openai API %s returned %s: %s", e.Path, e.Status, e.Body)
}

func NewOpenAIClient(baseURL, token, organizationID, projectID string) (*OpenAIClient, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("openai admin key is required")
	}
	normalized, err := openAIBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &OpenAIClient{
		baseURL:        normalized,
		token:          token,
		organizationID: strings.TrimSpace(organizationID),
		projectID:      strings.TrimSpace(projectID),
		http:           &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func openAIBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "https://api.openai.com/v1"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("openai base URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("openai base URL requires a host")
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

func (c *OpenAIClient) get(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	endpoint := c.baseURL + path
	if query != nil {
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
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
		if c.organizationID != "" {
			req.Header.Set("OpenAI-Organization", c.organizationID)
		}
		if c.projectID != "" {
			req.Header.Set("OpenAI-Project", c.projectID)
		}

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
			lastErr = &openAIAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
			time.Sleep(retryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &openAIAPIError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
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

func retryAfter(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(attempt+1) * 2 * time.Second
}

func requestPath(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Path == "" {
		return endpoint
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
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
