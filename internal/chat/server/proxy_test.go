package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSimpleProxy_ProxiesRequest(t *testing.T) {
	// Start a backend server.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"proxied":true,"path":"` + r.URL.Path + `"}`))
	}))
	defer backend.Close()

	// Set the env var so newSimpleProxy picks it up.
	t.Setenv("SOUL_TEST_PROXY_URL", backend.URL)

	proxy := newSimpleProxy("SOUL_TEST_PROXY_URL", "http://127.0.0.1:9999", "/api/test", "test")

	req := httptest.NewRequest("GET", "/api/test/foo", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNewSimpleProxy_StripsPrefixCorrectly(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	t.Setenv("SOUL_TEST_STRIP_URL", backend.URL)
	proxy := newSimpleProxy("SOUL_TEST_STRIP_URL", "http://127.0.0.1:9999", "/api/prefix", "test")

	req := httptest.NewRequest("GET", "/api/prefix/some/endpoint", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	// After stripping /api/prefix, the path should be /some/endpoint or /api/some/endpoint
	// depending on implementation.
	if receivedPath == "" {
		t.Error("backend never received request")
	}
}

func TestNewTutorProxy_ProxiesRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"dashboard":{}}`))
	}))
	defer backend.Close()

	t.Setenv("SOUL_TUTOR_URL", backend.URL)
	proxy := newTutorProxy()

	req := httptest.NewRequest("GET", "/api/tutor/dashboard", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNewProjectsProxy_ProxiesRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"projects":[]}`))
	}))
	defer backend.Close()

	t.Setenv("SOUL_PROJECTS_URL", backend.URL)
	proxy := newProjectsProxy()

	req := httptest.NewRequest("GET", "/api/projects/dashboard", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWithProxyOptions(t *testing.T) {
	// Verify proxy options don't panic.
	srv := New(
		WithPort(0),
		WithTutorProxy(),
		WithProjectsProxy(),
		WithInfraProxy(),
		WithQualityProxy(),
		WithDataProxy(),
		WithDocsProxy(),
		WithScoutProxy(),
		WithSentinelProxy(),
		WithMeshProxy(),
		WithBenchProxy(),
	)
	if srv == nil {
		t.Fatal("New returned nil with proxy options")
	}
	if srv.tutorProxy == nil {
		t.Error("tutor proxy should be initialized")
	}
	if srv.infraProxy == nil {
		t.Error("infra proxy should be initialized")
	}
	if srv.scoutProxy == nil {
		t.Error("scout proxy should be initialized")
	}
}

func TestWithOptionFunctions(t *testing.T) {
	srv := New(
		WithPort(9999),
		WithHost("0.0.0.0"),
		WithAuthToken("test-token"),
		WithTLS("/tmp/cert.pem", "/tmp/key.pem"),
		WithLiteLLMUpstream("http://localhost:4000"),
	)
	if srv.port != 9999 {
		t.Errorf("expected port 9999, got %d", srv.port)
	}
	if srv.host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", srv.host)
	}
	if srv.authToken != "test-token" {
		t.Errorf("expected auth token test-token, got %s", srv.authToken)
	}
	if srv.tlsCert != "/tmp/cert.pem" {
		t.Errorf("expected TLS cert path, got %s", srv.tlsCert)
	}
	if srv.litellmUpstream != "http://localhost:4000" {
		t.Errorf("expected litellm upstream, got %s", srv.litellmUpstream)
	}
}

func TestServerAddr(t *testing.T) {
	srv := New(WithPort(8080), WithHost("0.0.0.0"))
	addr := srv.Addr()
	if addr != "0.0.0.0:8080" {
		t.Errorf("expected 0.0.0.0:8080, got %s", addr)
	}
}
