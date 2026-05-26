package identity

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
	baseURL    string
	token      string
	authScheme string
	http       *http.Client
}

type apiError struct {
	Path       string
	Status     string
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("okta API %s returned %s: %s", e.Path, e.Status, e.Body)
}

func NewClient(orgURL, token, authScheme string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("okta token is required")
	}
	normalized, err := oktaBaseURL(orgURL)
	if err != nil {
		return nil, err
	}
	authScheme = strings.TrimSpace(authScheme)
	if authScheme == "" {
		authScheme = "SSWS"
	}
	if !strings.EqualFold(authScheme, "SSWS") && !strings.EqualFold(authScheme, "Bearer") {
		return nil, fmt.Errorf("--auth-scheme must be SSWS or Bearer")
	}
	return &Client{
		baseURL:    normalized,
		token:      token,
		authScheme: authScheme,
		http:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func oktaBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("--org-url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("okta org URL must use https, except loopback test URLs")
	}
	if u.Host == "" {
		return "", fmt.Errorf("okta org URL requires a host")
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

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) ([]byte, http.Header, error) {
	endpoint := c.baseURL + path
	if query != nil {
		if encoded := query.Encode(); encoded != "" {
			endpoint += "?" + encoded
		}
	}
	return c.getURL(ctx, endpoint, out)
}

func (c *Client) getURL(ctx context.Context, endpoint string, out any) ([]byte, http.Header, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", c.authScheme+" "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
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
			return nil, nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &apiError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
			time.Sleep(retryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, nil, &apiError{Path: requestPath(endpoint), Status: resp.Status, StatusCode: resp.StatusCode, Body: sanitizeBody(body)}
		}
		if out != nil {
			if err := json.Unmarshal(body, out); err != nil {
				return nil, nil, err
			}
		}
		return body, resp.Header.Clone(), nil
	}
	return nil, nil, lastErr
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
