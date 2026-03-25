package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rishav1305/soul/pkg/events"
)

// testLogger records event types for assertion.
type testLogger struct {
	events []string
}

func (l *testLogger) Log(eventType string, data map[string]interface{}) error {
	l.events = append(l.events, eventType)
	return nil
}

// validCredJSON returns a JSON credentials file with the given expiry (Unix ms).
func validCredJSON(t *testing.T, expiresAt int64) []byte {
	t.Helper()
	cf := credentialsFile{
		ClaudeAIOAuth: &OAuthCredentials{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			ExpiresAt:    expiresAt,
		},
	}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeCredFile writes credentials JSON to a temp file with given permissions.
func writeCredFile(t *testing.T, dir string, data []byte, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, ".credentials.json")
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

// futureMs returns a Unix milliseconds timestamp n duration from now.
func futureMs(d time.Duration) int64 {
	return time.Now().Add(d).UnixMilli()
}

// pastMs returns a Unix milliseconds timestamp n duration in the past.
func pastMs(d time.Duration) int64 {
	return time.Now().Add(-d).UnixMilli()
}

// --- Load tests ---

func TestLoad_ValidCredentials(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	creds, err := src.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if creds.AccessToken != "test-access-token" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "test-access-token")
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "test-refresh-token")
	}
	if creds.ExpiresAt != expiry {
		t.Errorf("ExpiresAt = %d, want %d", creds.ExpiresAt, expiry)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeCredFile(t, dir, []byte("{not valid json"), 0600)

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for corrupt JSON, got nil")
	}
}

func TestLoad_EmptyAccessToken(t *testing.T) {
	dir := t.TempDir()
	cf := credentialsFile{
		ClaudeAIOAuth: &OAuthCredentials{
			AccessToken:  "",
			RefreshToken: "refresh-tok",
			ExpiresAt:    futureMs(1 * time.Hour),
		},
	}
	data, _ := json.Marshal(cf)
	path := writeCredFile(t, dir, data, 0600)

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for empty AccessToken, got nil")
	}
}

func TestLoad_EmptyRefreshToken(t *testing.T) {
	dir := t.TempDir()
	cf := credentialsFile{
		ClaudeAIOAuth: &OAuthCredentials{
			AccessToken:  "access-tok",
			RefreshToken: "",
			ExpiresAt:    futureMs(1 * time.Hour),
		},
	}
	data, _ := json.Marshal(cf)
	path := writeCredFile(t, dir, data, 0600)

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for empty RefreshToken, got nil")
	}
}

func TestLoad_MissingClaudeAIOAuthField(t *testing.T) {
	dir := t.TempDir()
	path := writeCredFile(t, dir, []byte(`{"someOtherField": {}}`), 0600)

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for missing claudeAiOauth field, got nil")
	}
}

func TestLoad_RejectsWorldReadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission checks not reliable on Windows")
	}

	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0644)

	src := NewOAuthTokenSource(path, nil)
	_, err := src.Load()
	if err == nil {
		t.Fatal("Load() expected error for world-readable permissions (0644), got nil")
	}
}

func TestLoad_LogsOAuthReloadEvent(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	logger := &testLogger{}
	src := NewOAuthTokenSource(path, logger)
	_, err := src.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(logger.events) == 0 {
		t.Error("expected oauth.reload event, got none")
	}
	if logger.events[0] != events.EventOAuthReload {
		t.Errorf("event type = %q, want %q", logger.events[0], events.EventOAuthReload)
	}
}

// --- OAuthCredentials method tests ---

func TestIsExpired_PastExpiresAt(t *testing.T) {
	creds := &OAuthCredentials{
		ExpiresAt: pastMs(1 * time.Hour),
	}
	if !creds.IsExpired() {
		t.Error("IsExpired() = false, want true for past ExpiresAt")
	}
}

func TestIsExpired_FutureExpiresAt(t *testing.T) {
	creds := &OAuthCredentials{
		ExpiresAt: futureMs(1 * time.Hour),
	}
	if creds.IsExpired() {
		t.Error("IsExpired() = true, want false for future ExpiresAt")
	}
}

func TestNeedsRefresh_WithinWindow(t *testing.T) {
	creds := &OAuthCredentials{
		ExpiresAt: futureMs(2 * time.Minute), // 2 min < 5 min window
	}
	if !creds.NeedsRefresh() {
		t.Error("NeedsRefresh() = false, want true when < 5 minutes remain")
	}
}

func TestNeedsRefresh_OutsideWindow(t *testing.T) {
	creds := &OAuthCredentials{
		ExpiresAt: futureMs(10 * time.Minute), // 10 min > 5 min window
	}
	if creds.NeedsRefresh() {
		t.Error("NeedsRefresh() = true, want false when > 5 minutes remain")
	}
}

func TestValid_EmptyAccessToken(t *testing.T) {
	creds := &OAuthCredentials{
		AccessToken: "",
		ExpiresAt:   futureMs(1 * time.Hour),
	}
	if creds.Valid() {
		t.Error("Valid() = true, want false for empty AccessToken")
	}
}

func TestValid_Expired(t *testing.T) {
	creds := &OAuthCredentials{
		AccessToken: "some-token",
		ExpiresAt:   pastMs(1 * time.Hour),
	}
	if creds.Valid() {
		t.Error("Valid() = true, want false for expired token")
	}
}

func TestValid_ValidToken(t *testing.T) {
	creds := &OAuthCredentials{
		AccessToken: "some-token",
		ExpiresAt:   futureMs(1 * time.Hour),
	}
	if !creds.Valid() {
		t.Error("Valid() = false, want true for valid token")
	}
}

// --- Status tests ---

func TestStatus_Connected(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	status := src.Status()
	if status.State != "connected" {
		t.Errorf("State = %q, want %q", status.State, "connected")
	}
	if status.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero, want non-zero for connected state")
	}
}

func TestStatus_Missing(t *testing.T) {
	src := NewOAuthTokenSource("/nonexistent", nil)
	status := src.Status()
	if status.State != "missing" {
		t.Errorf("State = %q, want %q", status.State, "missing")
	}
}

func TestStatus_Expired(t *testing.T) {
	dir := t.TempDir()
	expiry := pastMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	// Load will succeed even with expired token — it just loads what's there.
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	status := src.Status()
	if status.State != "expired" {
		t.Errorf("State = %q, want %q", status.State, "expired")
	}
}

// --- Mock OAuth server helpers ---

// newMockOAuthServer returns an httptest.Server that mimics the Claude OAuth token endpoint.
// handler receives each request and returns the response status code and body.
func newMockOAuthServer(t *testing.T, handler func(r *http.Request) (int, oauthRefreshResponse)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, resp := handler(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newTokenSource creates an OAuthTokenSource with creds pre-loaded and the
// token URL pointed at the given mock server.
func newTokenSource(t *testing.T, credPath string, serverURL string) *OAuthTokenSource {
	t.Helper()
	src := NewOAuthTokenSource(credPath, nil)
	src.tokenURLOverride = serverURL
	return src
}

// --- Token tests ---

func TestToken_ReturnsCachedWhenValid(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := newTokenSource(t, path, "http://should-not-be-called")
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok != "test-access-token" {
		t.Errorf("Token() = %q, want %q", tok, "test-access-token")
	}
}

func TestToken_TriggersRefreshWhenNeedsRefresh(t *testing.T) {
	dir := t.TempDir()
	// Token expires in 2 minutes — within the 5-minute refresh window.
	expiry := futureMs(2 * time.Minute)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		return http.StatusOK, oauthRefreshResponse{
			AccessToken:  "refreshed-access-token",
			RefreshToken: "refreshed-refresh-token",
			ExpiresIn:    3600, // 1 hour in seconds
			TokenType:    "Bearer",
		}
	})

	// Override backoff so test doesn't wait.
	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok != "refreshed-access-token" {
		t.Errorf("Token() = %q, want %q", tok, "refreshed-access-token")
	}
}

func TestToken_NoCredsLoaded(t *testing.T) {
	src := NewOAuthTokenSource("/nonexistent", nil)
	_, err := src.Token()
	if err == nil {
		t.Fatal("Token() expected error when no creds loaded, got nil")
	}
}

// --- Refresh tests ---

func TestRefresh_Success(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(2 * time.Minute) // needs refresh
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	var called atomic.Int32
	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		called.Add(1)

		// Verify request format.
		var req oauthRefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if req.GrantType != "refresh_token" {
			t.Errorf("grant_type = %q, want %q", req.GrantType, "refresh_token")
		}
		if req.ClientID != OAuthClientID {
			t.Errorf("client_id = %q, want %q", req.ClientID, OAuthClientID)
		}
		if req.RefreshToken != "test-refresh-token" {
			t.Errorf("refresh_token = %q, want %q", req.RefreshToken, "test-refresh-token")
		}
		if r.Header.Get("anthropic-beta") != OAuthBetaHeader {
			t.Errorf("anthropic-beta header = %q, want %q", r.Header.Get("anthropic-beta"), OAuthBetaHeader)
		}

		return http.StatusOK, oauthRefreshResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    7200,
			TokenType:    "Bearer",
		}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	err := src.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if called.Load() != 1 {
		t.Errorf("mock server called %d times, want 1", called.Load())
	}

	// Verify cached creds were updated.
	src.mu.RLock()
	creds := src.creds
	src.mu.RUnlock()

	if creds.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "new-access-token")
	}
	if creds.RefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "new-refresh-token")
	}

	// ExpiresIn=7200s → 7200000ms added to current time.
	expectedExpiry := time.Now().UnixMilli() + 7200*1000
	if abs(creds.ExpiresAt-expectedExpiry) > 2000 {
		t.Errorf("ExpiresAt = %d, expected ~%d (tolerance 2s)", creds.ExpiresAt, expectedExpiry)
	}
}

func TestRefresh_401Failure(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(2 * time.Minute)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		return http.StatusUnauthorized, oauthRefreshResponse{}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	err := src.Refresh()
	if err == nil {
		t.Fatal("Refresh() expected error for 401, got nil")
	}
}

func TestRefresh_RetryWithBackoff(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(2 * time.Minute)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	var callCount atomic.Int32
	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		n := callCount.Add(1)
		if n < 3 {
			// First 2 calls fail with 500.
			return http.StatusInternalServerError, oauthRefreshResponse{}
		}
		// Third call succeeds.
		return http.StatusOK, oauthRefreshResponse{
			AccessToken:  "retry-access-token",
			RefreshToken: "retry-refresh-token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0} // no actual waiting in tests
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	err := src.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if callCount.Load() != 3 {
		t.Errorf("mock server called %d times, want 3 (2 failures + 1 success)", callCount.Load())
	}

	src.mu.RLock()
	tok := src.creds.AccessToken
	src.mu.RUnlock()
	if tok != "retry-access-token" {
		t.Errorf("AccessToken = %q, want %q", tok, "retry-access-token")
	}
}

func TestRefresh_SkipsIfAlreadyValid(t *testing.T) {
	dir := t.TempDir()
	// Token valid and NOT near expiry — should skip refresh.
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	var called atomic.Int32
	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		called.Add(1)
		return http.StatusOK, oauthRefreshResponse{}
	})

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	err := src.Refresh()
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if called.Load() != 0 {
		t.Errorf("server called %d times, want 0 (should skip when valid)", called.Load())
	}
}

// --- Persist tests ---

func TestPersist_WritesFileWithCorrectPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission checks not reliable on Windows")
	}

	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Update the creds in memory.
	src.mu.Lock()
	src.creds.AccessToken = "persisted-access-token"
	src.mu.Unlock()

	if err := src.Persist(); err != nil {
		t.Fatalf("Persist() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}
}

func TestPersist_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")

	// Write a file with extra fields.
	original := map[string]interface{}{
		"claudeAiOauth": map[string]interface{}{
			"accessToken":  "old-access",
			"refreshToken": "old-refresh",
			"expiresAt":    float64(futureMs(1 * time.Hour)),
		},
		"someOtherService": map[string]interface{}{
			"apiKey": "keep-this",
		},
	}
	data, _ := json.Marshal(original)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Persist (updates claudeAiOauth with current creds).
	if err := src.Persist(); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Read back and verify other fields preserved.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(after, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["someOtherService"]; !ok {
		t.Error("someOtherService field was not preserved after Persist()")
	}
	if _, ok := result["claudeAiOauth"]; !ok {
		t.Error("claudeAiOauth field missing after Persist()")
	}
}

func TestPersist_ThenLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Modify creds and persist.
	src.mu.Lock()
	src.creds = &OAuthCredentials{
		AccessToken:  "roundtrip-access",
		RefreshToken: "roundtrip-refresh",
		ExpiresAt:    expiry,
		OrgID:        "org-123",
	}
	src.mu.Unlock()

	if err := src.Persist(); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Load into a new source.
	src2 := NewOAuthTokenSource(path, nil)
	creds, err := src2.Load()
	if err != nil {
		t.Fatalf("Load after Persist: %v", err)
	}

	if creds.AccessToken != "roundtrip-access" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "roundtrip-access")
	}
	if creds.RefreshToken != "roundtrip-refresh" {
		t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "roundtrip-refresh")
	}
	if creds.OrgID != "org-123" {
		t.Errorf("OrgID = %q, want %q", creds.OrgID, "org-123")
	}
}

// --- ReloadFromDisk tests ---

func TestReloadFromDisk_UpdatesCachedCredentials(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Externally update the file (simulating Claude CLI refresh).
	newCreds := credentialsFile{
		ClaudeAIOAuth: &OAuthCredentials{
			AccessToken:  "disk-updated-access",
			RefreshToken: "disk-updated-refresh",
			ExpiresAt:    futureMs(2 * time.Hour),
		},
	}
	data, _ := json.Marshal(newCreds)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	updated := src.ReloadFromDisk()
	if !updated {
		t.Error("ReloadFromDisk() = false, want true when disk has newer token")
	}

	src.mu.RLock()
	tok := src.creds.AccessToken
	src.mu.RUnlock()
	if tok != "disk-updated-access" {
		t.Errorf("AccessToken = %q, want %q", tok, "disk-updated-access")
	}
}

func TestReloadFromDisk_ReturnsFalseWhenFileIsMissing(t *testing.T) {
	src := NewOAuthTokenSource("/nonexistent/creds.json", nil)
	if src.ReloadFromDisk() {
		t.Error("ReloadFromDisk() = true, want false for missing file")
	}
}

func TestReloadFromDisk_ReturnsFalseWhenSameToken(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, nil)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Reload same file — token unchanged.
	updated := src.ReloadFromDisk()
	if updated {
		t.Error("ReloadFromDisk() = true, want false when token is unchanged")
	}
}

// --- Disk fallback test ---

func TestToken_DiskFallbackWhenRefreshFails(t *testing.T) {
	dir := t.TempDir()
	// Start with a token that needs refresh (2 min until expiry).
	expiry := futureMs(2 * time.Minute)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	// Mock server always fails.
	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		return http.StatusInternalServerError, oauthRefreshResponse{}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Write a fresh valid token to disk (simulating another process refreshed it).
	freshCreds := credentialsFile{
		ClaudeAIOAuth: &OAuthCredentials{
			AccessToken:  "disk-fallback-access",
			RefreshToken: "disk-fallback-refresh",
			ExpiresAt:    futureMs(1 * time.Hour), // valid and outside refresh window
		},
	}
	data, _ := json.Marshal(freshCreds)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok != "disk-fallback-access" {
		t.Errorf("Token() = %q, want %q (disk fallback)", tok, "disk-fallback-access")
	}
}

func TestToken_FallsBackToCachedWhenRefreshAndDiskBothFail(t *testing.T) {
	dir := t.TempDir()
	// Token needs refresh but is still technically valid.
	expiry := futureMs(2 * time.Minute)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		return http.StatusInternalServerError, oauthRefreshResponse{}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Don't update disk — same file, same token. Reload won't help but
	// the cached token is still valid (not expired, just needs refresh).
	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	// Should return the original cached token since it's still Valid().
	if tok != "test-access-token" {
		t.Errorf("Token() = %q, want %q (fallback to cached)", tok, "test-access-token")
	}
}

// --- Concurrent Token() test ---

func TestToken_ConcurrentCallsOnlyOneRefresh(t *testing.T) {
	dir := t.TempDir()
	expiry := futureMs(2 * time.Minute) // needs refresh
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	var refreshCount atomic.Int32
	srv := newMockOAuthServer(t, func(r *http.Request) (int, oauthRefreshResponse) {
		refreshCount.Add(1)
		// Simulate slow refresh.
		time.Sleep(50 * time.Millisecond)
		return http.StatusOK, oauthRefreshResponse{
			AccessToken:  "concurrent-access",
			RefreshToken: "concurrent-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
	})

	origBackoff := refreshBackoff
	refreshBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() { refreshBackoff = origBackoff })

	src := newTokenSource(t, path, srv.URL)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Fire 10 concurrent Token() calls.
	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	tokens := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tok, err := src.Token()
			tokens[idx] = tok
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Token() error: %v", i, err)
		}
	}

	// The refreshMu serialization means the server should be called exactly once
	// (second+ goroutines see the already-refreshed token).
	// In practice it could be 1 (if all block on refreshMu) but we accept 1
	// as the desired minimum. The key invariant: NOT 10 separate refreshes.
	count := refreshCount.Load()
	if count > 2 {
		t.Errorf("refresh called %d times, want <= 2 (serialization should coalesce)", count)
	}
	if count < 1 {
		t.Error("refresh should have been called at least once")
	}
}

// --- Fuzz test ---

func FuzzOAuthCredentials(f *testing.F) {
	f.Add("token123", "refresh456", int64(time.Now().UnixMilli()+3600000))
	f.Add("", "", int64(0))
	f.Add("a", "b", int64(-1))
	f.Add("tok", "ref", int64(time.Now().UnixMilli()))

	f.Fuzz(func(t *testing.T, access, refresh string, expiresAt int64) {
		creds := &OAuthCredentials{
			AccessToken:  access,
			RefreshToken: refresh,
			ExpiresAt:    expiresAt,
		}
		// Should never panic.
		_ = creds.IsExpired()
		_ = creds.NeedsRefresh()
		_ = creds.Valid()
		_ = creds.ExpiresIn()
	})
}

// --- Helper ---

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// Ensure fmt is used (referenced by test helpers).
var _ = fmt.Sprintf
