// Package auth implements OAuth 2.1 authentication for the Soul v2 MCP server.
// JWT tokens use HS256 (HMAC-SHA256) without external dependencies.
// PKCE with S256 code challenge method is required for authorization code grants.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Token lifetimes.
const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour // 30 days
	authCodeTTL     = 5 * time.Minute
)

// --- JWT (HS256) --------------------------------------------------------

// jwtHeader is the fixed header for HS256 JWTs.
var jwtHeader = base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

// jwtClaims holds the payload fields we emit and verify.
type jwtClaims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// CreateAccessToken returns an HS256 JWT with a 1-hour expiry.
func CreateAccessToken(sub, secret string) (string, error) {
	return createToken(sub, secret, accessTokenTTL)
}

// CreateRefreshToken returns an HS256 JWT with a 30-day expiry.
func CreateRefreshToken(sub, secret string) (string, error) {
	return createToken(sub, secret, refreshTokenTTL)
}

func createToken(sub, secret string, ttl time.Duration) (string, error) {
	now := time.Now().Unix()
	claims := jwtClaims{Sub: sub, Iat: now, Exp: now + int64(ttl.Seconds())}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	payloadB64 := base64URLEncode(payload)
	signingInput := jwtHeader + "." + payloadB64
	sig := hmacSHA256([]byte(signingInput), []byte(secret))
	return signingInput + "." + base64URLEncode(sig), nil
}

// VerifyToken validates an HS256 JWT signature and expiry, returning the sub claim.
func VerifyToken(tokenStr, secret string) (string, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := hmacSHA256([]byte(signingInput), []byte(secret))
	actualSig, err := base64URLDecode(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return "", fmt.Errorf("invalid token signature")
	}

	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode claims: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", fmt.Errorf("parse claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return "", fmt.Errorf("token expired")
	}

	return claims.Sub, nil
}

// --- Helpers -------------------------------------------------------------

func hmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// --- PKCE (S256) ---------------------------------------------------------

// verifyCodeChallenge checks that SHA256(verifier) base64url-encoded equals challenge.
func verifyCodeChallenge(verifier, challenge string) bool {
	h := sha256.Sum256([]byte(verifier))
	computed := base64URLEncode(h[:])
	return computed == challenge
}

// --- Middleware -----------------------------------------------------------

// AuthMiddleware returns HTTP middleware that requires a valid Bearer token.
// Requests whose path starts with any entry in skipPaths bypass the check.
func AuthMiddleware(secret string, skipPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range skipPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				// Return WWW-Authenticate header per RFC 6750 so clients can discover the OAuth server
				w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="/.well-known/oauth-protected-resource"`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"missing or invalid authorization header"}`))
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			_, err := VerifyToken(token, secret)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"invalid token"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OriginMiddleware returns HTTP middleware that validates the Origin header
// against an allowlist. Requests with no Origin header are allowed (server-to-server).
// OAuth endpoints (/authorize, /token, /register, /.well-known/) skip validation
// since they handle their own authentication.
func OriginMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	oauthPaths := []string{"/authorize", "/token", "/register", "/.well-known/", "/health"}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip origin check for OAuth and discovery endpoints
			for _, p := range oauthPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !allowed[origin] {
				http.Error(w, `{"error":"origin not allowed"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- OAuth 2.1 Handler ---------------------------------------------------

type authCodeEntry struct {
	clientID      string
	codeChallenge string
	createdAt     time.Time
}

type clientEntry struct {
	secret       string
	redirectURIs []string
}

// OAuthHandler implements OAuth 2.1 authorization server endpoints with PKCE.
type OAuthHandler struct {
	secret    string
	adminPass string
	baseURL   string
	authCodes map[string]authCodeEntry
	clients   map[string]clientEntry
	mu        sync.Mutex
}

// NewOAuthHandler creates an OAuthHandler with the given JWT signing secret
// and admin password (used to approve authorization requests).
func NewOAuthHandler(secret, adminPass string) *OAuthHandler {
	return &OAuthHandler{
		secret:    secret,
		adminPass: adminPass,
		authCodes: make(map[string]authCodeEntry),
		clients:   make(map[string]clientEntry),
	}
}

// SetBaseURL sets the server's external base URL (e.g. "https://mcp.example.com").
func (h *OAuthHandler) SetBaseURL(url string) {
	h.mu.Lock()
	h.baseURL = url
	h.mu.Unlock()
}

func (h *OAuthHandler) getBaseURL() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.baseURL
}

// --- Discovery endpoints -------------------------------------------------

// HandleProtectedResource serves GET /.well-known/oauth-protected-resource.
func (h *OAuthHandler) HandleProtectedResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	base := h.getBaseURL()
	resp := map[string]interface{}{
		"resource":              base,
		"authorization_servers": []string{base},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleAuthorizationServer serves GET /.well-known/oauth-authorization-server.
func (h *OAuthHandler) HandleAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	base := h.getBaseURL()
	resp := map[string]interface{}{
		"issuer":                             base,
		"authorization_endpoint":             base + "/authorize",
		"token_endpoint":                     base + "/token",
		"registration_endpoint":              base + "/register",
		"response_types_supported":           []string{"code"},
		"grant_types_supported":              []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":    []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Authorization -------------------------------------------------------

// HandleAuthorize handles the authorization endpoint.
// GET: renders a simple HTML password form.
// POST: validates the admin password, generates an auth code, and redirects.
func (h *OAuthHandler) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.serveAuthForm(w, r)
	case http.MethodPost:
		h.processAuth(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *OAuthHandler) serveAuthForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Soul MCP — Authorize</title></head>
<body style="font-family:sans-serif;max-width:400px;margin:60px auto">
<h2>Authorize MCP Client</h2>
<form method="POST" action="">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<input type="hidden" name="code_challenge_method" value="%s">
<label>Password:<br><input type="password" name="password" required style="width:100%%;padding:8px;margin:8px 0"></label><br>
<button type="submit" style="padding:8px 24px;margin-top:8px">Approve</button>
</form>
</body></html>`,
		clientID, redirectURI, state, codeChallenge, codeChallengeMethod)
}

func (h *OAuthHandler) processAuth(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"bad form data"}`, http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")

	if password != h.adminPass {
		http.Error(w, `{"error":"invalid password"}`, http.StatusForbidden)
		return
	}

	if codeChallengeMethod != "S256" {
		http.Error(w, `{"error":"code_challenge_method must be S256"}`, http.StatusBadRequest)
		return
	}

	if codeChallenge == "" {
		http.Error(w, `{"error":"code_challenge required"}`, http.StatusBadRequest)
		return
	}

	code := randomHex(32)

	h.mu.Lock()
	h.authCodes[code] = authCodeEntry{
		clientID:      clientID,
		codeChallenge: codeChallenge,
		createdAt:     time.Now(),
	}
	h.mu.Unlock()

	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	location := redirectURI + sep + "code=" + code
	if state != "" {
		location += "&state=" + state
	}
	http.Redirect(w, r, location, http.StatusFound)
}

// --- Token ---------------------------------------------------------------

// HandleToken handles the token endpoint for authorization_code and refresh_token grants.
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"bad form data"}`, http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		h.handleAuthCodeExchange(w, r)
	case "refresh_token":
		h.handleRefreshGrant(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type"})
	}
}

func (h *OAuthHandler) handleAuthCodeExchange(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")

	h.mu.Lock()
	entry, ok := h.authCodes[code]
	if ok {
		delete(h.authCodes, code) // single-use
	}
	h.mu.Unlock()

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "invalid or expired authorization code"})
		return
	}

	// Check expiry.
	if time.Since(entry.createdAt) > authCodeTTL {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "authorization code expired"})
		return
	}

	// PKCE verification.
	if codeVerifier == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "code_verifier required"})
		return
	}

	if !verifyCodeChallenge(codeVerifier, entry.codeChallenge) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "PKCE verification failed"})
		return
	}

	h.issueTokens(w, entry.clientID)
}

func (h *OAuthHandler) handleRefreshGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	if refreshToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "error_description": "refresh_token required"})
		return
	}

	sub, err := VerifyToken(refreshToken, h.secret)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "invalid refresh token"})
		return
	}

	h.issueTokens(w, sub)
}

func (h *OAuthHandler) issueTokens(w http.ResponseWriter, sub string) {
	accessToken, err := CreateAccessToken(sub, h.secret)
	if err != nil {
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)
		return
	}

	refreshToken, err := CreateRefreshToken(sub, h.secret)
	if err != nil {
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
		"refresh_token": refreshToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(resp)
}

// --- Dynamic Client Registration (RFC 7591) ------------------------------

// HandleRegister handles POST /register for dynamic client registration.
func (h *OAuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client_metadata"})
		return
	}

	clientID := "client_" + randomHex(16)
	clientSecret := randomHex(32)

	h.mu.Lock()
	h.clients[clientID] = clientEntry{
		secret:       clientSecret,
		redirectURIs: req.RedirectURIs,
	}
	h.mu.Unlock()

	resp := map[string]interface{}{
		"client_id":                clientID,
		"client_secret":            clientSecret,
		"client_name":              req.ClientName,
		"redirect_uris":            req.RedirectURIs,
		"grant_types":              []string{"authorization_code", "refresh_token"},
		"response_types":           []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
