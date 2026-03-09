package stream

import (
	"net/http"
	"time"
)

const (
	// DefaultModel is the default Claude model used for API requests.
	DefaultModel = "claude-sonnet-4-20250514"

	// DefaultBaseURL is the default Claude API base URL.
	DefaultBaseURL = "https://api.anthropic.com"

	// DefaultAPIVersion is the default anthropic-version header value.
	DefaultAPIVersion = "2023-06-01"

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second
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

// NewClient creates a new Claude API client with the given token source and options.
func NewClient(auth TokenSource, opts ...Option) *Client {
	c := &Client{
		auth:       auth,
		model:      DefaultModel,
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
