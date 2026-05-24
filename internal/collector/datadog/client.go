package datadog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	appKey  string
	http    *http.Client
}

func NewClient(site, apiKey, appKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(appKey) == "" {
		return nil, fmt.Errorf("datadog API and app keys are required")
	}
	baseURL, err := datadogBaseURL(site)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		appKey:  appKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func datadogBaseURL(site string) (string, error) {
	site = strings.TrimSpace(site)
	if site == "" {
		site = "datadoghq.com"
	}
	if strings.HasPrefix(site, "http://") || strings.HasPrefix(site, "https://") {
		u, err := url.Parse(site)
		if err != nil {
			return "", err
		}
		if u.Scheme != "https" {
			return "", fmt.Errorf("datadog site URL must use https")
		}
		return strings.TrimRight(site, "/"), nil
	}
	allowed := map[string]bool{
		"datadoghq.com":     true,
		"datadoghq.eu":      true,
		"us3.datadoghq.com": true,
		"us5.datadoghq.com": true,
		"ap1.datadoghq.com": true,
		"ap2.datadoghq.com": true,
		"ddog-gov.com":      true,
	}
	if !allowed[site] {
		return "", fmt.Errorf("unsupported Datadog site %q", site)
	}
	return "https://api." + site, nil
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
		req.Header.Set("DD-API-KEY", c.apiKey)
		req.Header.Set("DD-APPLICATION-KEY", c.appKey)
		req.Header.Set("Accept", "application/json")
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
		if resp.StatusCode == http.StatusTooManyRequests {
			sleep := retryAfter(resp.Header.Get("Retry-After"), attempt)
			time.Sleep(sleep)
			lastErr = fmt.Errorf("datadog rate limited request to %s", path)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("datadog API %s returned %s: %s", path, resp.Status, sanitizeBody(body))
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

func sanitizeBody(body []byte) string {
	body = bytes.TrimSpace(body)
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
