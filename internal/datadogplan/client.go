package datadogplan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type apiClient struct {
	baseURL string
	apiKey  string
	appKey  string
	http    *http.Client
}

type apiError struct {
	StatusCode int
	Path       string
}

func (err *apiError) Error() string {
	return fmt.Sprintf("Datadog GET %s returned HTTP %d", err.Path, err.StatusCode)
}

func newAPIClient(site, baseURL, apiKey, appKey string, httpClient *http.Client) (*apiClient, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(appKey) == "" {
		return nil, fmt.Errorf("datadog API and application keys are required")
	}
	if baseURL == "" {
		var err error
		baseURL, err = datadogAPIBaseURL(site)
		if err != nil {
			return nil, err
		}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Datadog API base URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	safeHTTP := *httpClient
	// Do not let custom Datadog credential headers cross a redirect boundary.
	// A redirect is surfaced as a normal non-2xx API result instead.
	safeHTTP.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		appKey:  appKey,
		http:    &safeHTTP,
	}, nil
}

func datadogAPIBaseURL(site string) (string, error) {
	site = strings.TrimSpace(site)
	if site == "" {
		site = "datadoghq.com"
	}
	allowed := map[string]bool{
		"datadoghq.com":     true,
		"datadoghq.eu":      true,
		"us3.datadoghq.com": true,
		"us5.datadoghq.com": true,
		"ap1.datadoghq.com": true,
		"ap2.datadoghq.com": true,
		"uk1.datadoghq.com": true,
		"ddog-gov.com":      true,
		"us2.ddog-gov.com":  true,
	}
	if !allowed[site] {
		return "", fmt.Errorf("unsupported Datadog site %q", site)
	}
	return "https://api." + site, nil
}

func (c *apiClient) get(ctx context.Context, endpoint string) ([]byte, error) {
	requestURL, err := c.resolve(endpoint)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("DD-API-KEY", c.apiKey)
		req.Header.Set("DD-APPLICATION-KEY", c.appKey)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if err := waitForRetry(ctx, time.Duration(attempt+1)*200*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &apiError{StatusCode: resp.StatusCode, Path: requestPath(requestURL)}
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			if err := waitForRetry(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &apiError{StatusCode: resp.StatusCode, Path: requestPath(requestURL)}
		}
		return body, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("datadog GET failed")
	}
	return nil, lastErr
}

func (c *apiClient) resolve(endpoint string) (string, error) {
	base, _ := url.Parse(c.baseURL)
	next, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(next)
	if resolved.Scheme != base.Scheme || !strings.EqualFold(resolved.Host, base.Host) {
		return "", fmt.Errorf("datadog pagination URL changed host")
	}
	return resolved.String(), nil
}

func decodeJSON(data []byte) (any, error) {
	var value any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		if seconds > 30 {
			seconds = 30
		}
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(attempt+1) * 500 * time.Millisecond
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func requestPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "Datadog API"
	}
	return u.EscapedPath()
}
