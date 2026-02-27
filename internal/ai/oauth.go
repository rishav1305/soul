package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	oauthRefreshURL = "https://console.anthropic.com/api/oauth/token"
	oauthClientID   = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

// OAuthCredentials holds Claude Max/Pro OAuth token data.
type OAuthCredentials struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"` // Unix milliseconds
}

// credentialsFile matches the structure of ~/.claude/.credentials.json.
type credentialsFile struct {
	ClaudeAIOAuth *OAuthCredentials `json:"claudeAiOauth"`
}

// OAuthTokenSource manages OAuth tokens with automatic refresh.
type OAuthTokenSource struct {
	mu          sync.Mutex
	creds       OAuthCredentials
	credsPath   string
	httpClient  *http.Client
}

// LoadOAuthCredentials reads OAuth credentials from ~/.claude/.credentials.json.
// Returns nil if the file doesn't exist or has no OAuth credentials.
func LoadOAuthCredentials() *OAuthCredentials {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".claude", ".credentials.json")
	return loadOAuthFromPath(path)
}

func loadOAuthFromPath(path string) *OAuthCredentials {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil
	}

	if cf.ClaudeAIOAuth == nil || cf.ClaudeAIOAuth.AccessToken == "" {
		return nil
	}

	return cf.ClaudeAIOAuth
}

// NewOAuthTokenSource creates a token source that auto-refreshes expired tokens.
func NewOAuthTokenSource(creds *OAuthCredentials) *OAuthTokenSource {
	home, _ := os.UserHomeDir()
	credsPath := ""
	if home != "" {
		credsPath = filepath.Join(home, ".claude", ".credentials.json")
	}

	return &OAuthTokenSource{
		creds:      *creds,
		credsPath:  credsPath,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Token returns a valid access token, refreshing if expired.
func (s *OAuthTokenSource) Token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if current token is still valid (with 5 minute buffer).
	if s.creds.ExpiresAt > time.Now().UnixMilli()+5*60*1000 {
		return s.creds.AccessToken, nil
	}

	// Token expired or about to expire — refresh it.
	if s.creds.RefreshToken == "" {
		return "", fmt.Errorf("ai: OAuth token expired and no refresh token available")
	}

	newCreds, err := refreshOAuthToken(s.httpClient, s.creds.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("ai: failed to refresh OAuth token: %w", err)
	}

	s.creds = *newCreds

	// Persist refreshed credentials back to disk.
	if s.credsPath != "" {
		s.persistCredentials()
	}

	return s.creds.AccessToken, nil
}

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

func refreshOAuthToken(httpClient *http.Client, refreshToken string) (*OAuthCredentials, error) {
	reqBody, err := json.Marshal(oauthRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     oauthClientID,
	})
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Post(oauthRefreshURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp oauthRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().UnixMilli() + tokenResp.ExpiresIn*1000,
	}, nil
}

// persistCredentials writes updated credentials back to the credentials file.
func (s *OAuthTokenSource) persistCredentials() {
	data, err := os.ReadFile(s.credsPath)
	if err != nil {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	updated, err := json.Marshal(s.creds)
	if err != nil {
		return
	}
	raw["claudeAiOauth"] = updated

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(s.credsPath, out, 0600)
}
