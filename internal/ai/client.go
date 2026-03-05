package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	claudeAPIURL     = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion = "2023-06-01"
)

// AuthMode indicates how the client authenticates with the API.
type AuthMode int

const (
	AuthAPIKey AuthMode = iota
	AuthOAuth
)

// Client wraps the Claude Messages API with streaming support.
type Client struct {
	authMode   AuthMode
	apiKey     string       // used when AuthMode == AuthAPIKey
	oauth      *OAuthTokenSource // used when AuthMode == AuthOAuth
	model      string
	httpClient *http.Client
}

// NewClient creates a new Claude API client using an API key.
func NewClient(apiKey, model string) *Client {
	return &Client{
		authMode:   AuthAPIKey,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// NewOAuthClient creates a new Claude API client using OAuth credentials.
// Tokens are automatically refreshed when they expire.
func NewOAuthClient(tokenSource *OAuthTokenSource, model string) *Client {
	return &Client{
		authMode:   AuthOAuth,
		oauth:      tokenSource,
		model:      model,
		httpClient: &http.Client{},
	}
}

// Message represents a conversation message for the Claude API.
// Content is typed as any to support both simple strings and
// structured content blocks (e.g., tool results).
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ThinkingConfig enables extended thinking for supported models.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// Request represents a Claude Messages API request.
type Request struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []Message       `json:"messages"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	Thinking  *ThinkingConfig `json:"thinking,omitempty"`
}

// SendStream sends a streaming request to the Claude Messages API.
// It returns the response body as an io.ReadCloser for SSE event parsing.
// The caller is responsible for closing the reader.
func (c *Client) SendStream(ctx context.Context, req Request) (io.ReadCloser, error) {
	// Force streaming on.
	req.Stream = true

	// Use the client's model if the request doesn't specify one.
	if req.Model == "" {
		req.Model = c.model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	// Set auth header based on mode.
	if err := c.setAuthHeader(httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai: API returned status %d: %s", resp.StatusCode, string(errBody))
	}

	return resp.Body, nil
}

// CompleteSimple makes a non-streaming API call and returns the text response.
// Useful for quick, lightweight tasks like verification where streaming isn't needed.
func (c *Client) CompleteSimple(ctx context.Context, model, prompt string) (string, error) {
	if model == "" {
		model = c.model
	}

	apiReq := Request{
		Model:     model,
		MaxTokens: 1024,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return "", fmt.Errorf("ai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

	if err := c.setAuthHeader(httpReq); err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the Messages API response to extract text content.
	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("ai: failed to parse response: %w", err)
	}

	var result string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}
	return result, nil
}

// AuthMode returns how this client authenticates.
func (c *Client) GetAuthMode() AuthMode { return c.authMode }

// OAuthSource returns the OAuth token source, or nil if using API key auth.
func (c *Client) OAuthSource() *OAuthTokenSource {
	if c.authMode == AuthOAuth {
		return c.oauth
	}
	return nil
}

func (c *Client) setAuthHeader(req *http.Request) error {
	switch c.authMode {
	case AuthOAuth:
		token, err := c.oauth.AccessToken()
		if err != nil {
			return fmt.Errorf("ai: failed to get OAuth token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("anthropic-beta", oauthBetaHeader)
	default:
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return nil
}
