package auth

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testSecret = "test-signing-secret-32bytes!!"
const testAdmin = "admin-password-123"

// --- JWT roundtrip -------------------------------------------------------

func TestCreateAndVerifyToken(t *testing.T) {
	token, err := CreateAccessToken("user42", testSecret)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}

	sub, err := VerifyToken(token, testSecret)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if sub != "user42" {
		t.Fatalf("got sub=%q, want %q", sub, "user42")
	}

	// Refresh token roundtrip.
	rt, err := CreateRefreshToken("refresh-sub", testSecret)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	sub, err = VerifyToken(rt, testSecret)
	if err != nil {
		t.Fatalf("VerifyToken (refresh): %v", err)
	}
	if sub != "refresh-sub" {
		t.Fatalf("got sub=%q, want %q", sub, "refresh-sub")
	}
}

// --- Invalid tokens ------------------------------------------------------

func TestVerifyToken_Invalid(t *testing.T) {
	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt"},
		{"two parts", "abc.def"},
		{"bad base64 sig", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.!!!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyToken(tc.token, testSecret)
			if err == nil {
				t.Fatal("expected error for invalid token")
			}
		})
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, err := CreateAccessToken("user1", testSecret)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = VerifyToken(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got: %v", err)
	}
}

// --- AuthMiddleware ------------------------------------------------------

func TestAuthMiddleware_ValidToken(t *testing.T) {
	token, _ := CreateAccessToken("mw-user", testSecret)

	handler := AuthMiddleware(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	handler := AuthMiddleware(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rr.Code)
	}
}

func TestAuthMiddleware_SkipPath(t *testing.T) {
	handler := AuthMiddleware(testSecret, []string{"/health", "/.well-known/"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// /health should bypass auth.
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/health: got %d, want 200", rr.Code)
	}

	// /.well-known/oauth-authorization-server should bypass auth.
	req = httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/.well-known/: got %d, want 200", rr.Code)
	}
}

// --- OriginMiddleware ----------------------------------------------------

func TestOriginValidation(t *testing.T) {
	handler := OriginMiddleware([]string{"https://app.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("allowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
	})

	t.Run("no origin (server-to-server)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rr.Code)
		}
	})

	t.Run("bad origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://evil.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rr.Code)
		}
	})
}

// --- Discovery endpoints -------------------------------------------------

func TestProtectedResourceMetadata(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)
	h.SetBaseURL("https://mcp.example.com")

	req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
	rr := httptest.NewRecorder()
	h.HandleProtectedResource(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Resource points to the MCP endpoint (base + "/mcp/"), not the bare base.
	// Authorization server is the bare base URL.
	if body["resource"] != "https://mcp.example.com/mcp/" {
		t.Fatalf("resource=%v", body["resource"])
	}

	servers, ok := body["authorization_servers"].([]interface{})
	if !ok || len(servers) != 1 || servers[0] != "https://mcp.example.com" {
		t.Fatalf("authorization_servers=%v", body["authorization_servers"])
	}
}

func TestAuthorizationServerMetadata(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)
	h.SetBaseURL("https://mcp.example.com")

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	rr := httptest.NewRecorder()
	h.HandleAuthorizationServer(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	checks := map[string]string{
		"issuer":                 "https://mcp.example.com",
		"authorization_endpoint": "https://mcp.example.com/authorize",
		"token_endpoint":         "https://mcp.example.com/token",
		"registration_endpoint":  "https://mcp.example.com/register",
	}
	for key, want := range checks {
		got, _ := body[key].(string)
		if got != want {
			t.Errorf("%s=%q, want %q", key, got, want)
		}
	}

	methods, _ := body["code_challenge_methods_supported"].([]interface{})
	if len(methods) != 1 || methods[0] != "S256" {
		t.Errorf("code_challenge_methods_supported=%v", body["code_challenge_methods_supported"])
	}
}

// --- Client Registration -------------------------------------------------

func TestClientRegistration(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)

	reqBody := `{"redirect_uris":["http://localhost:8080/callback"],"client_name":"test-client"}`
	req := httptest.NewRequest("POST", "/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.HandleRegister(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	clientID, _ := body["client_id"].(string)
	if !strings.HasPrefix(clientID, "client_") {
		t.Fatalf("client_id=%q, want prefix 'client_'", clientID)
	}

	clientSecret, _ := body["client_secret"].(string)
	if len(clientSecret) < 32 {
		t.Fatalf("client_secret too short: %d chars", len(clientSecret))
	}
}

// --- PKCE ----------------------------------------------------------------

func TestPKCE_S256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64URLEncode(h[:])

	if !verifyCodeChallenge(verifier, challenge) {
		t.Fatal("PKCE S256 roundtrip failed")
	}

	if verifyCodeChallenge("wrong-verifier", challenge) {
		t.Fatal("PKCE should reject wrong verifier")
	}
}

// --- Full OAuth flow (integration) ---------------------------------------

func TestFullOAuthFlow(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)
	h.SetBaseURL("https://mcp.example.com")

	// 1. Register client.
	regBody := `{"redirect_uris":["http://localhost:9999/cb"],"client_name":"flow-test"}`
	regReq := httptest.NewRequest("POST", "/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRR := httptest.NewRecorder()
	h.HandleRegister(regRR, regReq)
	if regRR.Code != http.StatusCreated {
		t.Fatalf("register: %d", regRR.Code)
	}
	var regResp map[string]interface{}
	json.NewDecoder(regRR.Body).Decode(&regResp)
	clientID := regResp["client_id"].(string)

	// 2. PKCE: generate code_verifier and code_challenge.
	codeVerifier := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	codeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64URLEncode(codeHash[:])

	// 3. Authorize (POST — simulate form submission).
	form := url.Values{}
	form.Set("password", testAdmin)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", "http://localhost:9999/cb")
	form.Set("state", "xyzzy")
	form.Set("code_challenge", codeChallenge)
	form.Set("code_challenge_method", "S256")

	authReq := httptest.NewRequest("POST", "/authorize", strings.NewReader(form.Encode()))
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authRR := httptest.NewRecorder()
	h.HandleAuthorize(authRR, authReq)

	if authRR.Code != http.StatusFound {
		t.Fatalf("authorize: got %d, want 302; body=%s", authRR.Code, authRR.Body.String())
	}

	loc := authRR.Header().Get("Location")
	locURL, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	code := locURL.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect: %s", loc)
	}
	if locURL.Query().Get("state") != "xyzzy" {
		t.Fatalf("state mismatch in redirect: %s", loc)
	}

	// 4. Exchange code for tokens.
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("code", code)
	tokenForm.Set("code_verifier", codeVerifier)
	tokenForm.Set("client_id", clientID)

	tokenReq := httptest.NewRequest("POST", "/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRR := httptest.NewRecorder()
	h.HandleToken(tokenRR, tokenReq)

	if tokenRR.Code != http.StatusOK {
		body, _ := io.ReadAll(tokenRR.Body)
		t.Fatalf("token exchange: got %d; body=%s", tokenRR.Code, string(body))
	}

	var tokenResp map[string]interface{}
	json.NewDecoder(tokenRR.Body).Decode(&tokenResp)

	accessToken, _ := tokenResp["access_token"].(string)
	refreshToken, _ := tokenResp["refresh_token"].(string)

	if accessToken == "" || refreshToken == "" {
		t.Fatalf("empty tokens: %v", tokenResp)
	}

	// Verify the access token.
	sub, err := VerifyToken(accessToken, testSecret)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if sub != clientID {
		t.Fatalf("sub=%q, want %q", sub, clientID)
	}

	// 5. Refresh token grant.
	refreshForm := url.Values{}
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("refresh_token", refreshToken)

	refreshReq := httptest.NewRequest("POST", "/token", strings.NewReader(refreshForm.Encode()))
	refreshReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshRR := httptest.NewRecorder()
	h.HandleToken(refreshRR, refreshReq)

	if refreshRR.Code != http.StatusOK {
		body, _ := io.ReadAll(refreshRR.Body)
		t.Fatalf("refresh grant: got %d; body=%s", refreshRR.Code, string(body))
	}

	var refreshResp map[string]interface{}
	json.NewDecoder(refreshRR.Body).Decode(&refreshResp)

	newAccess, _ := refreshResp["access_token"].(string)
	if newAccess == "" {
		t.Fatal("refresh did not return new access token")
	}

	// 6. Code reuse should fail.
	tokenReq2 := httptest.NewRequest("POST", "/token", strings.NewReader(tokenForm.Encode()))
	tokenReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRR2 := httptest.NewRecorder()
	h.HandleToken(tokenRR2, tokenReq2)
	if tokenRR2.Code == http.StatusOK {
		t.Fatal("code reuse should fail")
	}
}

func TestAuthorize_WrongPassword(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)

	form := url.Values{}
	form.Set("password", "wrong-password")
	form.Set("client_id", "c1")
	form.Set("redirect_uri", "http://localhost/cb")
	form.Set("code_challenge", "abc")
	form.Set("code_challenge_method", "S256")

	req := httptest.NewRequest("POST", "/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.HandleAuthorize(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", rr.Code)
	}
}

func TestToken_WrongPKCEVerifier(t *testing.T) {
	h := NewOAuthHandler(testSecret, testAdmin)

	// Generate proper PKCE.
	verifier := "correct-verifier-value"
	codeHash := sha256.Sum256([]byte(verifier))
	challenge := base64URLEncode(codeHash[:])

	// Authorize with correct challenge.
	form := url.Values{}
	form.Set("password", testAdmin)
	form.Set("client_id", "c1")
	form.Set("redirect_uri", "http://localhost/cb")
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "S256")

	authReq := httptest.NewRequest("POST", "/authorize", strings.NewReader(form.Encode()))
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authRR := httptest.NewRecorder()
	h.HandleAuthorize(authRR, authReq)

	loc := authRR.Header().Get("Location")
	locURL, _ := url.Parse(loc)
	code := locURL.Query().Get("code")

	// Exchange with wrong verifier.
	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("code", code)
	tokenForm.Set("code_verifier", "wrong-verifier-value")

	tokenReq := httptest.NewRequest("POST", "/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRR := httptest.NewRecorder()
	h.HandleToken(tokenRR, tokenReq)

	if tokenRR.Code == http.StatusOK {
		t.Fatal("wrong PKCE verifier should fail")
	}
}
