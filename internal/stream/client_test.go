package stream

import (
	"net/http"
	"testing"
	"time"
)

// mockTokenSource implements TokenSource for testing.
type mockTokenSource struct {
	token string
	err   error
}

func (m *mockTokenSource) Token() (string, error) {
	return m.token, m.err
}

func TestNewClient_Defaults(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts)

	if c.model != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, c.model)
	}
	if c.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, c.baseURL)
	}
	if c.apiVersion != DefaultAPIVersion {
		t.Fatalf("expected default API version %q, got %q", DefaultAPIVersion, c.apiVersion)
	}
	if c.httpClient == nil {
		t.Fatal("expected non-nil HTTP client")
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Fatalf("expected timeout %v, got %v", DefaultTimeout, c.httpClient.Timeout)
	}
	if c.auth == nil {
		t.Fatal("expected non-nil auth")
	}
}

func TestNewClient_WithModel(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithModel("claude-opus-4-20250514"))

	if c.model != "claude-opus-4-20250514" {
		t.Fatalf("expected model claude-opus-4-20250514, got %q", c.model)
	}
}

func TestNewClient_WithBaseURL(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithBaseURL("http://localhost:8080"))

	if c.baseURL != "http://localhost:8080" {
		t.Fatalf("expected base URL http://localhost:8080, got %q", c.baseURL)
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	custom := &http.Client{Timeout: 60 * time.Second}
	c := NewClient(ts, WithHTTPClient(custom))

	if c.httpClient != custom {
		t.Fatal("expected custom HTTP client")
	}
	if c.httpClient.Timeout != 60*time.Second {
		t.Fatalf("expected 60s timeout, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_WithAPIVersion(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts, WithAPIVersion("2024-01-01"))

	if c.apiVersion != "2024-01-01" {
		t.Fatalf("expected API version 2024-01-01, got %q", c.apiVersion)
	}
}

func TestNewClient_MultipleOptions(t *testing.T) {
	ts := &mockTokenSource{token: "test-token"}
	c := NewClient(ts,
		WithModel("claude-opus-4-20250514"),
		WithBaseURL("http://localhost:9090"),
		WithAPIVersion("2024-06-01"),
	)

	if c.model != "claude-opus-4-20250514" {
		t.Fatalf("expected model claude-opus-4-20250514, got %q", c.model)
	}
	if c.baseURL != "http://localhost:9090" {
		t.Fatalf("expected base URL http://localhost:9090, got %q", c.baseURL)
	}
	if c.apiVersion != "2024-06-01" {
		t.Fatalf("expected API version 2024-06-01, got %q", c.apiVersion)
	}
}

func TestTokenSourceInterface(t *testing.T) {
	// Verify that mockTokenSource satisfies the TokenSource interface.
	var _ TokenSource = (*mockTokenSource)(nil)
}
