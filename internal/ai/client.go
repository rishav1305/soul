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

// Client wraps the Claude Messages API with streaming support.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new Claude API client.
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey:     apiKey,
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

// Request represents a Claude Messages API request.
type Request struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []Message    `json:"messages"`
	Tools     []ClaudeTool `json:"tools,omitempty"`
	Stream    bool         `json:"stream"`
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
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

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
