package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/auth"
	"github.com/rishav1305/soul-v2/internal/session"
)

// newTestServer creates a Server suitable for testing (no metrics, no auth).
func newTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	return New(opts...)
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)
	// Set startTime so uptime is deterministic.
	srv.startTime = time.Now().Add(-5 * time.Second)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
	if body["version"] != version {
		t.Errorf("expected version=%s, got %v", version, body["version"])
	}
	if body["uptime"] == nil || body["uptime"] == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestCSPHeaders(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP missing script-src: %s", csp)
	}
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP missing style-src: %s", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' ws://localhost:* ws://127.0.0.1:*") {
		t.Errorf("CSP missing connect-src: %s", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors: %s", csp)
	}
	if !strings.Contains(csp, "base-uri 'self'") {
		t.Errorf("CSP missing base-uri: %s", csp)
	}
}

func TestXContentTypeOptions(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
}

func TestXFrameOptions(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %q", got)
	}
}

func TestRateLimiterRejectsAfterThreshold(t *testing.T) {
	// Create server with low RPM for testing.
	srv := New()
	// We'll craft a handler with a very low limit to test quickly.
	// Since the default is 60, we need to either make 61 requests or
	// use a custom rate limit. Let's replace the middleware.

	// Instead, let's test the rateLimitMiddleware directly.
	handler := rateLimitMiddleware(5)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 requests should succeed.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// 6th request should be rate-limited.
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Errorf("expected Retry-After=60, got %q", got)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "rate limit exceeded" {
		t.Errorf("expected rate limit error, got %v", body["error"])
	}

	// Different IP should not be rate-limited.
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("different IP should not be rate-limited, got %d", rec2.Code)
	}

	_ = srv // keep the variable used for context
}

func TestRateLimiterSkipsStaticFiles(t *testing.T) {
	handler := rateLimitMiddleware(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Static file requests should never be rate-limited even after exceeding API limit.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/index.html", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("static request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("expected internal server error, got %v", body["error"])
	}
}

func TestSPAFallbackServesIndexHTML(t *testing.T) {
	// Create a temp directory with an index.html.
	dir := t.TempDir()
	indexContent := `<!DOCTYPE html><html><body>SPA</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, WithStaticDir(dir))

	// Request a path that doesn't exist as a file → should get index.html.
	req := httptest.NewRequest("GET", "/some/client/route", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "SPA") {
		t.Errorf("expected SPA content, got %q", body)
	}

	// Check no-cache header on index.html.
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("expected Cache-Control=no-cache for SPA fallback, got %q", got)
	}
}

func TestSPADoesNotServeIndexForAPIPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, WithStaticDir(dir))

	// Unknown API path should get 404 JSON, not index.html.
	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "not found" {
		t.Errorf("expected 'not found' error, got %v", body["error"])
	}
}

func TestStaticFileServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create assets subdirectory.
	assetsDir := filepath.Join(dir, "assets")
	if err := os.Mkdir(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsContent := `console.log("hello");`
	if err := os.WriteFile(filepath.Join(assetsDir, "main.a1b2c3d4.js"), []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/assets/main.a1b2c3d4.js", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("expected javascript content type, got %q", ct)
	}

	// Hashed asset should get long cache.
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("expected long cache for hashed asset, got %q", cc)
	}
}

func TestAuthStatusMissing(t *testing.T) {
	srv := newTestServer(t) // no auth configured

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["state"] != "missing" {
		t.Errorf("expected state=missing, got %v", body["state"])
	}
}

func TestRequestIDHeader(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected X-Request-ID header to be present")
	}

	// Make a second request — should get a different ID.
	req2 := httptest.NewRequest("GET", "/api/health", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	rid2 := rec2.Header().Get("X-Request-ID")
	if rid2 == "" {
		t.Fatal("expected X-Request-ID header on second request")
	}
	if rid == rid2 {
		t.Errorf("expected different request IDs, got same: %s", rid)
	}
}

func TestCSPHeadersOnStaticFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, WithStaticDir(dir))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if csp := rec.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("expected CSP header on static file response")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected nosniff on static response, got %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected DENY on static response, got %q", got)
	}
}

func TestDefaultsFromEnv(t *testing.T) {
	t.Setenv("SOUL_V2_PORT", "9999")
	t.Setenv("SOUL_V2_HOST", "0.0.0.0")

	srv := New()
	if srv.port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", srv.port)
	}
	if srv.host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0 from env, got %s", srv.host)
	}
}

func TestOptionOverridesEnv(t *testing.T) {
	t.Setenv("SOUL_V2_PORT", "9999")

	srv := New(WithPort(8080))
	if srv.port != 8080 {
		t.Errorf("expected port 8080 from option, got %d", srv.port)
	}
}

func TestNoStaticDirReturns404(t *testing.T) {
	srv := newTestServer(t) // no staticDir set

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHealthEndpointContentType(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %q", ct)
	}
}

func TestClientIPExtraction(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{
			name:       "from RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For single",
			remoteAddr: "10.0.0.1:1234",
			xff:        "203.0.113.50",
			want:       "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For chain",
			remoteAddr: "10.0.0.1:1234",
			xff:        "203.0.113.50, 70.41.3.18, 150.172.238.178",
			want:       "203.0.113.50",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(req)
			if got != tc.want {
				t.Errorf("clientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsHashedAsset(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"main.a1b2c3.js", true},
		{"style.abc123.css", true},
		{"index.html", false},
		{"favicon.ico", false},
		{"robots.txt", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsHashedAsset(tc.name); got != tc.want {
				t.Errorf("IsHashedAsset(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// --- Reauth endpoint tests ---

func TestReauth_ReturnsAuthStatusWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	expiry := time.Now().Add(1 * time.Hour).UnixMilli()
	credJSON := `{"claudeAiOauth":{"accessToken":"test-tok","refreshToken":"test-refresh","expiresAt":` + intToStr(expiry) + `}}`
	credPath := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		t.Fatal(err)
	}

	src := auth.NewOAuthTokenSource(credPath, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	srv := newTestServer(t, WithAuth(src))

	req := httptest.NewRequest("POST", "/api/reauth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %q", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}

	authMap, ok := body["auth"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected auth to be a map, got %T", body["auth"])
	}
	if authMap["state"] != "connected" {
		t.Errorf("expected auth.state=connected, got %v", authMap["state"])
	}
}

func TestReauth_ReloadsCredentialsFromDisk(t *testing.T) {
	dir := t.TempDir()
	// Start with token A.
	expiry := time.Now().Add(1 * time.Hour).UnixMilli()
	credJSON := `{"claudeAiOauth":{"accessToken":"token-A","refreshToken":"refresh-A","expiresAt":` + intToStr(expiry) + `}}`
	credPath := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		t.Fatal(err)
	}

	src := auth.NewOAuthTokenSource(credPath, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	srv := newTestServer(t, WithAuth(src))

	// Verify initial state via /api/auth/status.
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var statusBefore map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&statusBefore)
	if statusBefore["state"] != "connected" {
		t.Fatalf("expected initial state=connected, got %v", statusBefore["state"])
	}

	// Externally update credentials file to expired token.
	expiredAt := time.Now().Add(-1 * time.Hour).UnixMilli()
	newCredJSON := `{"claudeAiOauth":{"accessToken":"token-B","refreshToken":"refresh-B","expiresAt":` + intToStr(expiredAt) + `}}`
	if err := os.WriteFile(credPath, []byte(newCredJSON), 0600); err != nil {
		t.Fatal(err)
	}

	// Call reauth to reload from disk.
	req2 := httptest.NewRequest("POST", "/api/reauth", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	var reauthBody map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&reauthBody)

	authMap := reauthBody["auth"].(map[string]interface{})
	// After reload, the token is expired, so state should be "expired".
	if authMap["state"] != "expired" {
		t.Errorf("expected auth.state=expired after reloading expired token, got %v", authMap["state"])
	}
}

func TestReauth_ReturnsErrorWhenAuthNil(t *testing.T) {
	srv := newTestServer(t) // no auth configured

	req := httptest.NewRequest("POST", "/api/reauth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["error"] != "authentication not configured" {
		t.Errorf("expected error='authentication not configured', got %v", body["error"])
	}
}

func TestAuthStatus_CorrectStateAfterReauth(t *testing.T) {
	dir := t.TempDir()
	expiry := time.Now().Add(1 * time.Hour).UnixMilli()
	credJSON := `{"claudeAiOauth":{"accessToken":"initial-tok","refreshToken":"initial-refresh","expiresAt":` + intToStr(expiry) + `}}`
	credPath := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		t.Fatal(err)
	}

	src := auth.NewOAuthTokenSource(credPath, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	srv := newTestServer(t, WithAuth(src))

	// Update credentials file to a new valid token with different expiry.
	newExpiry := time.Now().Add(2 * time.Hour).UnixMilli()
	newCredJSON := `{"claudeAiOauth":{"accessToken":"new-tok","refreshToken":"new-refresh","expiresAt":` + intToStr(newExpiry) + `}}`
	if err := os.WriteFile(credPath, []byte(newCredJSON), 0600); err != nil {
		t.Fatal(err)
	}

	// Trigger reauth.
	req := httptest.NewRequest("POST", "/api/reauth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reauth expected 200, got %d", rec.Code)
	}

	// Now check /api/auth/status reflects the reloaded state.
	req2 := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("auth/status expected 200, got %d", rec2.Code)
	}

	var status map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&status)
	if status["state"] != "connected" {
		t.Errorf("expected state=connected, got %v", status["state"])
	}
}

func TestReauth_NonPOSTMethodReturns405(t *testing.T) {
	dir := t.TempDir()
	expiry := time.Now().Add(1 * time.Hour).UnixMilli()
	credJSON := `{"claudeAiOauth":{"accessToken":"tok","refreshToken":"ref","expiresAt":` + intToStr(expiry) + `}}`
	credPath := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(credJSON), 0600); err != nil {
		t.Fatal(err)
	}

	src := auth.NewOAuthTokenSource(credPath, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	srv := newTestServer(t, WithAuth(src))

	// GET should not match the POST route.
	methods := []string{"GET", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/reauth", nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			// Go 1.22+ method-based routing returns 405 for wrong method on matched path.
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /api/reauth: expected 405, got %d", method, rec.Code)
			}
		})
	}
}

// intToStr converts an int64 to a string for JSON embedding.
func intToStr(n int64) string {
	return fmt.Sprintf("%d", n)
}

// --- Session endpoint tests ---

// newTestSessionStore creates a session store using a temp directory for tests.
func newTestSessionStore(t *testing.T) *session.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test-sessions.db")
	store, err := session.Open(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestListSessions_EmptyInitially(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	sessions, ok := body["sessions"].([]interface{})
	if !ok {
		t.Fatalf("expected sessions to be an array, got %T", body["sessions"])
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestCreateSession_DefaultTitle(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %q", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	sess, ok := body["session"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected session to be an object, got %T", body["session"])
	}
	if sess["title"] != "New Session" {
		t.Errorf("expected default title 'New Session', got %v", sess["title"])
	}
	if sess["id"] == nil || sess["id"] == "" {
		t.Error("expected non-empty session ID")
	}
	if sess["status"] != "idle" {
		t.Errorf("expected status=idle, got %v", sess["status"])
	}
}

func TestCreateSession_CustomTitle(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"title":"My Chat"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	sess := body["session"].(map[string]interface{})
	if sess["title"] != "My Chat" {
		t.Errorf("expected title 'My Chat', got %v", sess["title"])
	}
}

func TestListSessions_OrderedByUpdatedAtDesc(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	// Create sessions with small pauses to ensure different timestamps.
	titles := []string{"First", "Second", "Third"}
	for _, title := range titles {
		req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"title":"`+title+`"}`))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: expected 201, got %d", title, rec.Code)
		}
		time.Sleep(10 * time.Millisecond) // ensure distinct timestamps
	}

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	sessions := body["sessions"].([]interface{})
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Most recently updated should be first (descending order).
	first := sessions[0].(map[string]interface{})
	last := sessions[2].(map[string]interface{})
	if first["title"] != "Third" {
		t.Errorf("expected first session to be 'Third', got %v", first["title"])
	}
	if last["title"] != "First" {
		t.Errorf("expected last session to be 'First', got %v", last["title"])
	}
}

func TestDeleteSession_Success(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	// Create a session first.
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{"title":"To Delete"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var createBody map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&createBody)
	sess := createBody["session"].(map[string]interface{})
	id := sess["id"].(string)

	// Delete the session.
	req2 := httptest.NewRequest("DELETE", "/api/sessions/"+id, nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	var delBody map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&delBody)
	if delBody["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", delBody["deleted"])
	}

	// Verify it's gone.
	req3 := httptest.NewRequest("GET", "/api/sessions", nil)
	rec3 := httptest.NewRecorder()
	srv.ServeHTTP(rec3, req3)

	var listBody map[string]interface{}
	json.NewDecoder(rec3.Body).Decode(&listBody)
	sessions := listBody["sessions"].([]interface{})
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	// Use a valid UUID format that doesn't exist.
	req := httptest.NewRequest("DELETE", "/api/sessions/00000000-0000-4000-8000-000000000000", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "session not found" {
		t.Errorf("expected 'session not found', got %q", body["error"])
	}
}

func TestDeleteSession_InvalidUUID(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("DELETE", "/api/sessions/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "invalid session ID" {
		t.Errorf("expected 'invalid session ID', got %q", body["error"])
	}
}

func TestGetMessages_EmptyForNewSession(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	// Create a session.
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var createBody map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&createBody)
	sess := createBody["session"].(map[string]interface{})
	id := sess["id"].(string)

	// Get messages.
	req2 := httptest.NewRequest("GET", "/api/sessions/"+id+"/messages", nil)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&body)
	messages := body["messages"].([]interface{})
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestGetMessages_NotFoundForNonexistentSession(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("GET", "/api/sessions/00000000-0000-4000-8000-000000000000/messages", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "session not found" {
		t.Errorf("expected 'session not found', got %q", body["error"])
	}
}

func TestCreateSession_InvalidJSON(t *testing.T) {
	store := newTestSessionStore(t)
	srv := newTestServer(t, WithSessionStore(store))

	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(`{invalid json`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "invalid JSON body" {
		t.Errorf("expected 'invalid JSON body', got %q", body["error"])
	}
}

func TestSessionEndpoints_Return503WhenStoreNil(t *testing.T) {
	srv := newTestServer(t) // no session store

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/sessions"},
		{"POST", "/api/sessions"},
		{"DELETE", "/api/sessions/00000000-0000-4000-8000-000000000000"},
		{"GET", "/api/sessions/00000000-0000-4000-8000-000000000000/messages"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var body *strings.Reader
			if ep.method == "POST" {
				body = strings.NewReader(`{}`)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(ep.method, ep.path, body)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d", rec.Code)
			}

			var respBody map[string]string
			json.NewDecoder(rec.Body).Decode(&respBody)
			if respBody["error"] != "session store not configured" {
				t.Errorf("expected 'session store not configured', got %q", respBody["error"])
			}
		})
	}
}
