package githubenterprise

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
	return fmt.Sprintf("github API %s returned %s: %s", e.Path, e.Status, e.Body)
}

func NewClient(baseURL, token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("github token is required")
	}
	normalized, err := githubBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: normalized,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func githubBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "https://api.github.com"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("github base URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("github base URL requires a host")
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
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
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
			time.Sleep(githubRetryAfter(resp.Header.Get("Retry-After"), attempt))
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

func githubRetryAfter(value string, attempt int) time.Duration {
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
