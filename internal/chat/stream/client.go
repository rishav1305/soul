package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// DefaultModel is the default Claude model used for API requests.
	DefaultModel = "claude-sonnet-4-6"

	// DefaultBaseURL is the default Claude API base URL.
	DefaultBaseURL = "https://api.anthropic.com"

	// DefaultAPIVersion is the default anthropic-version header value.
	DefaultAPIVersion = "2023-06-01"

	// DefaultTimeout is the default HTTP client timeout for non-streaming requests.
	// Streaming uses no timeout — cancellation is via request context.
	DefaultTimeout = 5 * time.Minute
)

// TokenSource abstracts token retrieval for API authentication.
// Satisfied by auth.OAuthTokenSource.
type TokenSource interface {
	Token() (string, error)
}

// Client is the Claude API streaming client.
type Client struct {
	auth       TokenSource
	model      string
	httpClient *http.Client
	baseURL    string
	apiVersion string
	betaHeader string
}

// Option configures a Client.
type Option func(*Client)

// WithModel sets the Claude model identifier.
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// WithBaseURL sets the API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithAPIVersion sets the anthropic-version header value.
func WithAPIVersion(v string) Option {
	return func(c *Client) {
		c.apiVersion = v
	}
}

// WithBetaHeader sets the anthropic-beta header value.
// Multiple features can be comma-separated (e.g. "prompt-caching-2024-07-31,oauth-2025-04-20").
func WithBetaHeader(h string) Option {
	return func(c *Client) {
		c.betaHeader = h
	}
}

// NewClient creates a new Claude API client with the given token source and options.
func NewClient(auth TokenSource, opts ...Option) *Client {
	c := &Client{
		auth:       auth,
		model:      DefaultModel,
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		betaHeader: "prompt-caching-2024-07-31",
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Stream sends a streaming request to the Claude API and returns a channel
// of SSE events. The channel is closed when the stream ends (message_stop,
// error, or context cancellation). The HTTP response body is closed automatically.
func (c *Client) Stream(ctx context.Context, req *Request) (<-chan SSEEvent, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Copy to avoid mutating the caller's request.
	r := *req
	r.Stream = true
	if r.Model == "" {
		r.Model = c.model
	}
	req = &r

	token, err := c.auth.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", c.apiVersion)
	httpReq.Header.Set("anthropic-beta", c.betaHeader)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleErrorResponse(resp)
	}

	ch := make(chan SSEEvent, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		parser := NewSSEParser(resp.Body)
		for {
			evt, err := parser.Next()
			if err != nil {
				if err != io.EOF {
					ch <- SSEEvent{Type: "error", Error: &APIError{Message: err.Error()}}
				}
				return
			}
			select {
			case ch <- *evt:
			case <-ctx.Done():
				return
			}
			if evt.IsTerminal() {
				return
			}
		}
	}()
	return ch, nil
}

// Send makes a non-streaming request to the Claude API and returns the complete response.
func (c *Client) Send(ctx context.Context, req *Request) (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Copy to avoid mutating the caller's request.
	r := *req
	r.Stream = false
	if r.Model == "" {
		r.Model = c.model
	}
	req = &r

	token, err := c.auth.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", c.apiVersion)
	httpReq.Header.Set("anthropic-beta", c.betaHeader)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// handleErrorResponse converts an HTTP error response into the appropriate error type.
func (c *Client) handleErrorResponse(resp *http.Response) error {
	// Claude API errors have the format: {"type": "error", "error": {"type": "...", "message": "..."}}
	var wrapper struct {
		Type  string   `json:"type"`
		Error APIError `json:"error"`
	}
	var apiErr APIError
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		apiErr = APIError{Type: "unknown", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	} else if wrapper.Error.Type != "" {
		apiErr = wrapper.Error
	} else {
		// Fallback: maybe the error was at the top level
		apiErr = APIError{Type: wrapper.Type, Message: fmt.Sprintf("HTTP %d (no error detail)", resp.StatusCode)}
	}
	apiErr.StatusCode = resp.StatusCode

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthError{Err: &apiErr}
	case http.StatusTooManyRequests:
		retryAfter := time.Duration(0)
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		return &RateLimitError{RetryAfter: retryAfter, Err: &apiErr}
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return &ServerError{StatusCode: resp.StatusCode, Err: &apiErr}
	default:
		return &apiErr
	}
}
