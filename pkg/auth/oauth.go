package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/pkg/events"
)

// OAuth constants — used by token refresh in Step 1.4.
const (
	OAuthTokenURL   = "https://platform.claude.com/v1/oauth/token"
	OAuthClientID   = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	OAuthBetaHeader = "oauth-2025-04-20"

	// refreshWindow is the duration before expiry when NeedsRefresh returns true.
	refreshWindow = 5 * time.Minute

	// maxFilePerms is the most permissive acceptable file mode for credentials.
	maxFilePerms fs.FileMode = 0600
)

// OAuthCredentials holds Claude Max/Pro OAuth token data.
type OAuthCredentials struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"` // Unix milliseconds (matches Claude CLI format)
	OrgID        string `json:"orgId,omitempty"`
}

// IsExpired returns true if ExpiresAt (as milliseconds) is in the past.
func (c *OAuthCredentials) IsExpired() bool {
	return c.ExpiresAt <= time.Now().UnixMilli()
}

// ExpiresIn returns the duration until the token expires.
// Returns a negative duration if already expired.
func (c *OAuthCredentials) ExpiresIn() time.Duration {
	expiryTime := time.UnixMilli(c.ExpiresAt)
	return time.Until(expiryTime)
}

// NeedsRefresh returns true if the token expires within 5 minutes.
func (c *OAuthCredentials) NeedsRefresh() bool {
	return c.ExpiresIn() < refreshWindow
}

// Valid returns true if AccessToken is non-empty and the token is not expired.
func (c *OAuthCredentials) Valid() bool {
	return c.AccessToken != "" && !c.IsExpired()
}

// credentialsFile matches the structure of ~/.claude/.credentials.json.
type credentialsFile struct {
	ClaudeAIOAuth *OAuthCredentials `json:"claudeAiOauth"`
}

// AuthStatus represents the current authentication state.
type AuthStatus struct {
	State     string    `json:"state"`             // "connected", "expired", "missing", "error"
	ExpiresAt time.Time `json:"expiresAt"`         // zero if missing
	Error     string    `json:"error,omitempty"`
}

// OAuthTokenSource manages OAuth tokens with loading, validation, and refresh.
type OAuthTokenSource struct {
	credPath         string
	creds            *OAuthCredentials
	mu               sync.RWMutex
	refreshMu        sync.Mutex
	httpClient       *http.Client
	logger           events.Logger
	tokenURLOverride string // for testing — overrides OAuthTokenURL
}

// NewOAuthTokenSource creates a token source that loads credentials from credPath.
// The logger parameter may be nil — a NopLogger is used when nil.
func NewOAuthTokenSource(credPath string, logger events.Logger) *OAuthTokenSource {
	if logger == nil {
		logger = events.NopLogger{}
	}
	return &OAuthTokenSource{
		credPath:   credPath,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// Load reads the credentials file, parses JSON, validates fields, checks file
// permissions (rejects if world-readable), and returns the credentials.
// Token values never appear in error messages.
func (s *OAuthTokenSource) Load() (*OAuthCredentials, error) {
	// Check file permissions before reading content.
	info, err := os.Stat(s.credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found: %s", s.credPath)
		}
		return nil, fmt.Errorf("stat credentials file: %w", err)
	}

	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return nil, fmt.Errorf("credentials file %s has permissions %04o, want %04o or stricter", s.credPath, mode, maxFilePerms)
	}

	data, err := os.ReadFile(s.credPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse credentials file: %w", err)
	}

	if cf.ClaudeAIOAuth == nil {
		return nil, fmt.Errorf("credentials file missing claudeAiOauth field")
	}

	creds := cf.ClaudeAIOAuth

	if creds.AccessToken == "" {
		return nil, fmt.Errorf("credentials file has empty accessToken")
	}
	if creds.RefreshToken == "" {
		return nil, fmt.Errorf("credentials file has empty refreshToken")
	}

	// Store loaded credentials.
	s.mu.Lock()
	s.creds = creds
	s.mu.Unlock()

	// Log the reload event.
	_ = s.logger.Log(events.EventOAuthReload, map[string]interface{}{
		"path":    s.credPath,
		"expired": creds.IsExpired(),
	})

	return creds, nil
}

// Status returns the current authentication state based on loaded credentials.
func (s *OAuthTokenSource) Status() AuthStatus {
	s.mu.RLock()
	creds := s.creds
	s.mu.RUnlock()

	if creds == nil {
		return AuthStatus{State: "missing"}
	}

	if creds.IsExpired() {
		return AuthStatus{
			State:     "expired",
			ExpiresAt: time.UnixMilli(creds.ExpiresAt),
		}
	}

	return AuthStatus{
		State:     "connected",
		ExpiresAt: time.UnixMilli(creds.ExpiresAt),
	}
}

// --- Refresh types (internal) ---

type oauthRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

type oauthRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
}

// refreshBackoff defines the retry schedule for transient refresh failures.
var refreshBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// Token returns a valid access token. It refreshes if the cached token is
// nearing expiry, and falls back to disk if refresh fails.
// Token values are never logged.
func (s *OAuthTokenSource) Token() (string, error) {
	s.mu.RLock()
	creds := s.creds
	s.mu.RUnlock()

	if creds == nil {
		return "", fmt.Errorf("no credentials loaded — call Load() first")
	}

	// Fast path: valid and not near expiry.
	if creds.Valid() && !creds.NeedsRefresh() {
		return creds.AccessToken, nil
	}

	// Slow path: needs refresh.
	err := s.Refresh()
	if err == nil {
		s.mu.RLock()
		tok := s.creds.AccessToken
		s.mu.RUnlock()
		return tok, nil
	}

	// Refresh failed — try disk fallback.
	if s.ReloadFromDisk() {
		s.mu.RLock()
		diskCreds := s.creds
		s.mu.RUnlock()
		if diskCreds.Valid() && !diskCreds.NeedsRefresh() {
			return diskCreds.AccessToken, nil
		}
	}

	// If the original cached token is still technically valid (just near expiry),
	// return it rather than failing hard.
	if creds.Valid() {
		return creds.AccessToken, nil
	}

	return "", fmt.Errorf("token refresh failed and no valid fallback: %w", err)
}

// Refresh exchanges the refresh token for a new access token via the OAuth endpoint.
// It retries with exponential backoff (1s, 2s, 4s) on transient failures.
// Only one refresh is in-flight at a time (serialized via refreshMu).
func (s *OAuthTokenSource) Refresh() error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	// Re-check under lock — another goroutine may have refreshed while we waited.
	s.mu.RLock()
	creds := s.creds
	s.mu.RUnlock()
	if creds != nil && creds.Valid() && !creds.NeedsRefresh() {
		return nil
	}

	s.mu.RLock()
	refreshToken := s.creds.RefreshToken
	s.mu.RUnlock()

	start := time.Now()
	var lastErr error

	for attempt, backoff := range refreshBackoff {
		if attempt > 0 {
			time.Sleep(backoff)
		}

		err := s.doRefresh(refreshToken)
		if err == nil {
			// Log success (never log token values).
			_ = s.logger.Log(events.EventOAuthRefresh, map[string]interface{}{
				"success":  true,
				"duration": time.Since(start).Milliseconds(),
				"attempts": attempt + 1,
			})
			return nil
		}
		lastErr = err
	}

	// Log failure.
	_ = s.logger.Log(events.EventOAuthRefresh, map[string]interface{}{
		"success":  false,
		"duration": time.Since(start).Milliseconds(),
		"attempts": len(refreshBackoff),
		"error":    lastErr.Error(),
	})

	return fmt.Errorf("refresh failed after %d attempts: %w", len(refreshBackoff), lastErr)
}

// doRefresh performs a single refresh token exchange against the OAuth endpoint.
func (s *OAuthTokenSource) doRefresh(refreshToken string) error {
	reqBody := oauthRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     OAuthClientID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal refresh request: %w", err)
	}

	// Use the tokenURL field if set (for testing), otherwise use the constant.
	url := s.tokenURL()

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-beta", OAuthBetaHeader)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var tokenResp oauthRefreshResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("refresh response has empty access_token")
	}

	// ExpiresIn is in seconds — convert to milliseconds for ExpiresAt.
	newExpiresAt := time.Now().UnixMilli() + tokenResp.ExpiresIn*1000

	s.mu.Lock()
	s.creds = &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    newExpiresAt,
		OrgID:        s.creds.OrgID, // preserve OrgID
	}
	s.mu.Unlock()

	// Persist new creds to disk.
	if err := s.Persist(); err != nil {
		// Log but don't fail — the in-memory creds are still valid.
		_ = s.logger.Log(events.EventOAuthRefresh, map[string]interface{}{
			"persist_error": err.Error(),
		})
	}

	return nil
}

// tokenURL returns the OAuth token endpoint URL. It checks for an override
// (used in tests) and falls back to the production constant.
func (s *OAuthTokenSource) tokenURL() string {
	if s.tokenURLOverride != "" {
		return s.tokenURLOverride
	}
	return OAuthTokenURL
}

// Persist writes the current credentials to disk, preserving other fields
// in the credentials file. The file is written with 0600 permissions.
func (s *OAuthTokenSource) Persist() error {
	s.mu.RLock()
	creds := s.creds
	s.mu.RUnlock()

	if creds == nil {
		return fmt.Errorf("no credentials to persist")
	}

	// Read existing file to preserve other fields.
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(s.credPath)
	if err == nil {
		// Parse existing — ignore errors, we'll write a fresh file.
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}

	// Marshal the OAuth creds.
	oauthBytes, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	raw["claudeAiOauth"] = oauthBytes

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials file: %w", err)
	}

	if err := os.WriteFile(s.credPath, out, 0600); err != nil {
		return fmt.Errorf("write credentials file: %w", err)
	}

	return nil
}

// ReloadFromDisk re-reads credentials from the disk file and updates the
// cached credentials if the file contains a newer/different token.
// Returns true if credentials were updated.
func (s *OAuthTokenSource) ReloadFromDisk() bool {
	data, err := os.ReadFile(s.credPath)
	if err != nil {
		return false
	}

	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return false
	}

	if cf.ClaudeAIOAuth == nil || cf.ClaudeAIOAuth.AccessToken == "" {
		return false
	}

	s.mu.Lock()
	old := s.creds
	s.creds = cf.ClaudeAIOAuth
	s.mu.Unlock()

	updated := old == nil || old.AccessToken != cf.ClaudeAIOAuth.AccessToken

	if updated {
		_ = s.logger.Log(events.EventOAuthReload, map[string]interface{}{
			"path":    s.credPath,
			"expired": cf.ClaudeAIOAuth.IsExpired(),
		})
	}

	return updated
}

// truncate returns s truncated to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
