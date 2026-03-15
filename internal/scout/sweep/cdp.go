package sweep

import (
	"net/http"
	"time"
)

// CDPClient connects to Chrome DevTools Protocol for browser automation.
type CDPClient struct {
	endpoint string // ws://127.0.0.1:9222
}

// NewCDPClient creates a new CDP client pointing at the given endpoint.
func NewCDPClient(endpoint string) *CDPClient {
	return &CDPClient{endpoint: endpoint}
}

// Available checks if the CDP endpoint is reachable by querying /json/version.
// Returns false gracefully if the endpoint is not available.
func (c *CDPClient) Available() bool {
	if c.endpoint == "" {
		return false
	}
	// Convert ws:// to http:// for the version check.
	httpURL := c.endpoint
	if len(httpURL) > 5 && httpURL[:5] == "ws://" {
		httpURL = "http://" + httpURL[5:]
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(httpURL + "/json/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Endpoint returns the configured CDP endpoint URL.
func (c *CDPClient) Endpoint() string {
	return c.endpoint
}
