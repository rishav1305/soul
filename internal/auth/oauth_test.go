package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rishav1305/soul-v2/internal/metrics"
)

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
	metricsDir := t.TempDir()

	logger, err := metrics.NewEventLogger(metricsDir)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer logger.Close()

	expiry := futureMs(1 * time.Hour)
	path := writeCredFile(t, dir, validCredJSON(t, expiry), 0600)

	src := NewOAuthTokenSource(path, logger)
	_, err = src.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify the metrics file was written to (at least one line).
	metricsPath := filepath.Join(metricsDir, "metrics.jsonl")
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected oauth.reload event in metrics log, got empty file")
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
