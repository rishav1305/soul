package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	oauthTokenURL    = "https://platform.claude.com/v1/oauth/token"
	oauthCreateKeyURL = "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"
	oauthClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauthBetaHeader  = "oauth-2025-04-20"
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

// OAuthTokenSource manages OAuth tokens and exchanges them for ephemeral API keys.
type OAuthTokenSource struct {
	mu         sync.Mutex
	creds      OAuthCredentials
	credsPath  string
	apiKey     string // ephemeral API key obtained from OAuth token
	apiKeyExp  int64  // when the ephemeral key expires (Unix ms)
	httpClient *http.Client
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

// APIKey returns a valid ephemeral API key, refreshing the OAuth token
// and/or creating a new key as needed.
func (s *OAuthTokenSource) APIKey() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return cached API key if still valid (with 5 minute buffer).
	if s.apiKey != "" && s.apiKeyExp > time.Now().UnixMilli()+5*60*1000 {
		return s.apiKey, nil
	}

	// Ensure we have a valid OAuth access token.
	accessToken, err := s.validAccessToken()
	if err != nil {
		return "", fmt.Errorf("ai: failed to get OAuth access token: %w", err)
	}

	// Exchange the access token for an ephemeral API key.
	apiKey, err := createEphemeralAPIKey(s.httpClient, accessToken)
	if err != nil {
		return "", fmt.Errorf("ai: failed to create ephemeral API key: %w", err)
	}

	s.apiKey = apiKey
	// Ephemeral keys typically last ~1 hour; re-create after 50 minutes.
	s.apiKeyExp = time.Now().UnixMilli() + 50*60*1000

	return s.apiKey, nil
}

// validAccessToken returns a valid OAuth access token, refreshing if expired.
func (s *OAuthTokenSource) validAccessToken() (string, error) {
	// Check if current token is still valid (with 5 minute buffer).
	if s.creds.ExpiresAt > time.Now().UnixMilli()+5*60*1000 {
		return s.creds.AccessToken, nil
	}

	// Token expired or about to expire — refresh it.
	if s.creds.RefreshToken == "" {
		return "", fmt.Errorf("OAuth token expired and no refresh token available")
	}

	log.Println("  Refreshing OAuth token...")

	newCreds, err := refreshOAuthToken(s.httpClient, s.creds.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh OAuth token: %w", err)
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

	resp, err := httpClient.Post(oauthTokenURL, "application/json", bytes.NewReader(reqBody))
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

type createAPIKeyResponse struct {
	APIKey    string `json:"api_key"`
	ExpiresAt string `json:"expires_at"`
}

func createEphemeralAPIKey(httpClient *http.Client, accessToken string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, oauthCreateKeyURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", oauthBetaHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create_api_key returned status %d: %s", resp.StatusCode, string(body))
	}

	var keyResp createAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&keyResp); err != nil {
		return "", fmt.Errorf("failed to decode API key response: %w", err)
	}

	if keyResp.APIKey == "" {
		return "", fmt.Errorf("create_api_key returned empty API key")
	}

	return keyResp.APIKey, nil
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
