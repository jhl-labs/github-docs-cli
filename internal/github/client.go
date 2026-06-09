// Package github is a thin, dependency-free client for the GitHub REST API
// (api.github.com or a GitHub Enterprise Server base) and the GraphQL API
// (used for Discussions). It handles Bearer auth, retries on transient
// failures (429/5xx and secondary rate limits), and structured errors.
package github

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github-docs-cli/internal/config"
)

// Client talks to a single GitHub (or GHE) instance.
type Client struct {
	restURL    string // e.g. https://api.github.com
	graphqlURL string // e.g. https://api.github.com/graphql
	httpClient *http.Client
	token      string
	userAgent  string

	// MaxRetries is the number of additional attempts on transient errors.
	MaxRetries int
	// RetryWait is the base backoff between attempts.
	RetryWait time.Duration

	// lastRetryAfter carries a server-provided Retry-After hint into the next
	// backoff. It is only meaningful between attempts of one doJSON call.
	lastRetryAfter time.Duration
}

// APIError is a non-2xx response from GitHub.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("github API %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("github API %d", e.StatusCode)
}

// NotFound reports whether err is a 404 APIError.
func NotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// New builds a Client from config. It returns an error if the config is
// invalid or has no usable credentials.
func New(cfg config.Config, timeout time.Duration) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		restURL:    cfg.APIURL,
		graphqlURL: graphqlURL(cfg.APIURL),
		httpClient: &http.Client{Timeout: timeout, Transport: transport},
		token:      cfg.Token,
		userAgent:  "github-docs-cli",
		MaxRetries: 3,
		RetryWait:  500 * time.Millisecond,
	}, nil
}

// graphqlURL derives the GraphQL endpoint from the REST base URL.
//
//	https://api.github.com            -> https://api.github.com/graphql
//	https://ghe.example.com/api/v3    -> https://ghe.example.com/api/graphql
func graphqlURL(restURL string) string {
	restURL = strings.TrimRight(restURL, "/")
	if restURL == config.DefaultAPIURL {
		return config.DefaultAPIURL + "/graphql"
	}
	if strings.HasSuffix(restURL, "/api/v3") {
		return strings.TrimSuffix(restURL, "/api/v3") + "/api/graphql"
	}
	return restURL + "/graphql"
}

// doJSON performs a REST API request, decoding a JSON response into out (which
// may be nil). path is relative to the API root, e.g. "/repos/o/r/issues".
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	endpoint := c.restURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	data, _, err := c.do(ctx, method, endpoint, headers, payload)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// graphQLRequest is the POST body for a GraphQL call.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLError is one error entry in a GraphQL response.
type graphQLError struct {
	Message string `json:"message"`
}

// doGraphQL executes a GraphQL query/mutation, decoding the "data" field into
// out. GraphQL-level errors are returned even on HTTP 200.
func (c *Client) doGraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("encoding graphql request: %w", err)
	}
	headers := map[string]string{"Accept": "application/json"}
	data, _, err := c.do(ctx, http.MethodPost, c.graphqlURL, headers, payload)
	if err != nil {
		return err
	}

	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decoding graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, 0, len(resp.Errors))
		for _, e := range resp.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("graphql error: %s", strings.Join(msgs, "; "))
	}
	if out == nil || len(resp.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return fmt.Errorf("decoding graphql data: %w", err)
	}
	return nil
}

// do issues a single HTTP request with retries, returning the raw response
// body and status on success (2xx).
func (c *Client) do(ctx context.Context, method, endpoint string, headers map[string]string, payload []byte) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(wait):
			}
		}

		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
		if err != nil {
			return nil, 0, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", c.userAgent)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.MaxRetries {
				continue // network errors are retryable
			}
			return nil, 0, fmt.Errorf("request failed: %w", err)
		}

		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return data, resp.StatusCode, nil
		}

		apiErr := parseAPIError(resp.StatusCode, data)
		if c.shouldRetry(resp, attempt) {
			lastErr = apiErr
			c.lastRetryAfter = retryAfter(resp.Header)
			continue
		}
		return nil, resp.StatusCode, apiErr
	}

	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, errors.New("request failed after retries")
}

// shouldRetry decides whether a non-2xx response is worth retrying.
func (c *Client) shouldRetry(resp *http.Response, attempt int) bool {
	if attempt >= c.MaxRetries {
		return false
	}
	if isRetryable(resp.StatusCode) {
		return true
	}
	// GitHub secondary rate limits answer 403 with a Retry-After hint.
	if resp.StatusCode == http.StatusForbidden && retryAfter(resp.Header) > 0 {
		return true
	}
	return false
}

func (c *Client) backoff(attempt int) time.Duration {
	if c.lastRetryAfter > 0 {
		d := c.lastRetryAfter
		c.lastRetryAfter = 0
		return d
	}
	// Exponential backoff: base * 2^(attempt-1).
	return c.RetryWait * time.Duration(1<<(attempt-1))
}

func isRetryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	}
	return false
}

func retryAfter(h http.Header) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return 0
}

func parseAPIError(status int, data []byte) *APIError {
	e := &APIError{StatusCode: status, Body: string(data)}
	// GitHub error shape: {"message":"...","errors":[{"message":"..."}]}.
	var parsed struct {
		Message string `json:"message"`
		Errors  []struct {
			Message  string `json:"message"`
			Field    string `json:"field"`
			Code     string `json:"code"`
			Resource string `json:"resource"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &parsed) == nil && parsed.Message != "" {
		e.Message = parsed.Message
		var detail []string
		for _, d := range parsed.Errors {
			switch {
			case d.Message != "":
				detail = append(detail, d.Message)
			case d.Field != "" && d.Code != "":
				detail = append(detail, fmt.Sprintf("%s: %s", d.Field, d.Code))
			}
		}
		if len(detail) > 0 {
			e.Message += " (" + strings.Join(detail, "; ") + ")"
		}
	} else if len(data) > 0 && len(data) < 300 {
		e.Message = strings.TrimSpace(string(data))
	}
	return e
}
