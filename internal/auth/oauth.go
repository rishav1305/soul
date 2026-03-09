package auth

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rishav1305/soul-v2/internal/metrics"
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

// OAuthTokenSource manages OAuth tokens with loading, validation, and (in Step 1.4) refresh.
type OAuthTokenSource struct {
	credPath   string
	creds      *OAuthCredentials
	mu         sync.RWMutex
	refreshMu  sync.Mutex
	httpClient *http.Client
	logger     *metrics.EventLogger
}

// NewOAuthTokenSource creates a token source that loads credentials from credPath.
// The logger parameter may be nil — logging is skipped when nil.
func NewOAuthTokenSource(credPath string, logger *metrics.EventLogger) *OAuthTokenSource {
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

	// Log the reload event (if logger provided).
	if s.logger != nil {
		_ = s.logger.Log(metrics.EventOAuthReload, map[string]interface{}{
			"path":    s.credPath,
			"expired": creds.IsExpired(),
		})
	}

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
