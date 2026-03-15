# Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden Soul v2 against internet, LAN, and compromised-device attacks using an outside-in layered approach.

**Architecture:** Cloudflare Access as the internet gate (zero-trust email OTP), bearer token middleware for application-level auth (all network paths), WebSocket origin hardening, file permission lockdown, and OWASP-compliant security headers.

**Tech Stack:** Go (net/http middleware), React/TypeScript (localStorage token), Cloudflare Zero Trust (browser dashboard), UFW, cloudflared

**Spec:** `docs/superpowers/specs/2026-03-16-security-hardening-design.md`

---

## File Structure

| File | Responsibility | Action |
|------|---------------|--------|
| `internal/chat/server/server.go` | Auth middleware, security headers middleware | Modify |
| `internal/chat/server/server_test.go` | Tests for auth + headers middleware | Modify |
| `internal/chat/ws/hub.go` | WS origin hardening, WS token auth | Modify |
| `internal/chat/ws/hub_test.go` | Tests for origin + token validation | Modify |
| `cmd/chat/main.go` | Read SOUL_V2_AUTH_TOKEN env, pass to server | Modify |
| `web/src/lib/ws.ts` | Append token to WS URL | Modify |
| `web/src/lib/api.ts` | Add Authorization header, handle 401 | Modify |
| `web/src/components/AuthGate.tsx` | Token prompt UI component | Create |
| `web/src/main.tsx` | Wrap app in AuthGate | Modify |
| `/etc/cloudflared/config.yml` | Remove noTLSVerify | Modify (infra) |
| `/etc/systemd/system/soul-v2.service` | Add SOUL_V2_AUTH_TOKEN env | Modify (infra) |

---

## Task 1: Cloudflare Access — Internet Gate

**Files:** None (browser-based config)

- [ ] **Step 1: Navigate to Cloudflare Zero Trust dashboard**

Open `https://one.dash.cloudflare.com/` → Access → Applications → Add an application → Self-hosted

- [ ] **Step 2: Configure the application**

```
Application name: Soul v2
Session Duration: 24 hours
Application domain: soul.rishavchatterjee.com
```

- [ ] **Step 3: Add access policy**

```
Policy name: Owner only
Action: Allow
Include: Emails — rishav.chatt@gmail.com
```

- [ ] **Step 4: Verify internet access is blocked**

```bash
curl -s -o /dev/null -w "%{http_code}" https://soul.rishavchatterjee.com/
# Expected: 302 (redirect to Cloudflare login, not 200)
```

- [ ] **Step 5: Commit note**

No code to commit — document in the PR description.

---

## Task 2: Infrastructure Hardening

**Files:**
- Modify: `/etc/cloudflared/config.yml`
- Modify: UFW rules

- [ ] **Step 1: Fix file permissions**

```bash
chmod 700 ~/.soul-v2/
chmod 600 ~/.soul-v2/*.db
chmod 600 ~/.soul-v2/*.jsonl
```

- [ ] **Step 2: Verify permissions**

```bash
stat -c "%a %n" ~/.soul-v2/ ~/.soul-v2/*.db
# Expected: 700 ~/.soul-v2/   600 ~/.soul-v2/chat.db (etc.)
```

- [ ] **Step 3: Fix cloudflared config**

Edit `/etc/cloudflared/config.yml` — remove the `originRequest` block:

Before:
```yaml
ingress:
  - hostname: soul.rishavchatterjee.com
    service: http://localhost:3002
    originRequest:
      noTLSVerify: true
  - service: http_status:404
```

After:
```yaml
ingress:
  - hostname: soul.rishavchatterjee.com
    service: http://localhost:3002
  - service: http_status:404
```

- [ ] **Step 4: Restart cloudflared**

```bash
sudo systemctl restart cloudflared
sudo systemctl status cloudflared --no-pager | head -5
# Expected: active (running)
```

- [ ] **Step 5: Restrict Portainer UFW**

```bash
sudo ufw delete allow 9080/tcp 2>/dev/null
sudo ufw allow from 192.168.0.145 to any port 9080 comment "Portainer from titan-pi only"
sudo ufw status | grep 9080
```

---

## Task 3: WebSocket Origin Hardening

**Files:**
- Modify: `internal/chat/ws/hub.go`
- Modify: `internal/chat/ws/hub_test.go`

- [ ] **Step 1: Write failing test for empty-Origin rejection**

Add to `internal/chat/ws/hub_test.go`:

```go
func TestIsOriginAllowed_EmptyOrigin_PublicIP(t *testing.T) {
	hub := NewHub()
	// Simulate request from public IP with no Origin header
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "203.0.113.50:12345" // Public IP (TEST-NET-3)
	if hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from public IP to be rejected")
	}
}

func TestIsOriginAllowed_EmptyOrigin_PrivateIP(t *testing.T) {
	hub := NewHub()
	// Simulate request from private IP with no Origin header
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "192.168.0.100:12345"
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from private IP to be allowed")
	}
}

func TestIsOriginAllowed_EmptyOrigin_CloudflareJWT(t *testing.T) {
	hub := NewHub()
	// Simulate Cloudflare tunnel request
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "203.0.113.50:12345"
	req.Header.Set("Cf-Access-Jwt-Assertion", "eyJhbGciOi...")
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin with CF JWT to be allowed")
	}
}

func TestIsOriginAllowed_EmptyOrigin_TailscaleIP(t *testing.T) {
	hub := NewHub()
	// Simulate Tailscale request (CGNAT range)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "100.116.180.93:12345"
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from Tailscale IP to be allowed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/rishav/soul-v2 && go test ./internal/chat/ws/ -run "TestIsOriginAllowed_EmptyOrigin" -v
# Expected: TestIsOriginAllowed_EmptyOrigin_PublicIP FAIL (currently allows all empty origins)
```

- [ ] **Step 3: Add `isPrivateOrTrustedIP` helper and fix `isOriginAllowed`**

In `internal/chat/ws/hub.go`, add helper function:

```go
// isPrivateOrTrustedIP checks if an IP is RFC 1918, loopback, or Tailscale CGNAT.
func isPrivateOrTrustedIP(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// Loopback
	if ip.IsLoopback() {
		return true
	}
	// RFC 1918 + Tailscale CGNAT (100.64.0.0/10)
	privateRanges := []struct{ start, end net.IP }{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")},
		{net.ParseIP("100.64.0.0"), net.ParseIP("100.127.255.255")},
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	for _, r := range privateRanges {
		if bytes.Compare(ip4, r.start.To4()) >= 0 && bytes.Compare(ip4, r.end.To4()) <= 0 {
			return true
		}
	}
	return false
}
```

Then update the empty-origin block in `isOriginAllowed`:

```go
if origin == "" {
	// Allow Cloudflare tunnel (has JWT header).
	if r.Header.Get("Cf-Access-Jwt-Assertion") != "" {
		return true
	}
	// Allow private/trusted network IPs (LAN, Tailscale, loopback).
	if isPrivateOrTrustedIP(r.RemoteAddr) {
		return true
	}
	// Reject all other empty-origin requests.
	return false
}
```

Add `"bytes"` and `"net"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/chat/ws/ -run "TestIsOriginAllowed" -v
# Expected: all 4 new tests PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/chat/ws/hub.go internal/chat/ws/hub_test.go
git commit -m "fix: reject empty-Origin WebSocket from public IPs

Allow empty-Origin only from private IPs (LAN/Tailscale) or
Cloudflare tunnel (Cf-Access-Jwt-Assertion header). Blocks
script-based WebSocket bypass from the internet."
```

---

## Task 4: Auth Middleware — Server Side

**Files:**
- Modify: `internal/chat/server/server.go`
- Modify: `internal/chat/server/server_test.go`
- Modify: `internal/chat/ws/hub.go`
- Modify: `cmd/chat/main.go`

- [ ] **Step 1: Write failing test for auth middleware**

Add to `internal/chat/server/server_test.go`:

```go
func TestAuthMiddleware_RejectsNoToken(t *testing.T) {
	srv := New(WithAuthToken("secret123"))
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_AcceptsValidToken(t *testing.T) {
	srv := New(WithAuthToken("secret123"), WithSessionStore(newTestStore(t)))
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == 401 {
		t.Error("expected non-401 with valid token")
	}
}

func TestAuthMiddleware_AllowsStaticWithoutToken(t *testing.T) {
	srv := New(WithAuthToken("secret123"), WithStaticDir("../../web/dist"))
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == 401 {
		t.Error("expected SPA route to be accessible without token")
	}
}

func TestAuthMiddleware_Disabled_WhenEmpty(t *testing.T) {
	srv := New(WithAuthToken(""))
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == 401 {
		t.Error("auth middleware should be disabled when token is empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/chat/server/ -run "TestAuthMiddleware" -v
# Expected: FAIL — WithAuthToken not defined
```

- [ ] **Step 3: Add `authToken` field and `WithAuthToken` option**

In `internal/chat/server/server.go`, add field to Server struct:

```go
type Server struct {
	// ... existing fields ...
	authToken string // bearer token for API auth (empty = disabled)
}
```

Add option:

```go
// WithAuthToken sets the bearer token for API authentication.
// If empty, auth middleware is disabled (local dev mode).
func WithAuthToken(token string) Option {
	return func(s *Server) { s.authToken = token }
}
```

- [ ] **Step 4: Implement `authMiddleware`**

In `internal/chat/server/server.go`:

```go
// authMiddleware rejects requests to /api/* and /ws without a valid bearer token.
// Static assets (/, /assets/*, /manifest.json, /favicon.ico, /sw.js) are exempt.
func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path

			// Exempt: SPA, static assets, service worker
			if path == "/" || path == "/favicon.ico" || path == "/manifest.json" || path == "/sw.js" ||
				strings.HasPrefix(path, "/assets/") {
				next.ServeHTTP(w, r)
				return
			}

			// Check Authorization header
			auth := r.Header.Get("Authorization")
			if auth == "Bearer "+token {
				next.ServeHTTP(w, r)
				return
			}

			// Check query param (for WebSocket)
			if r.URL.Query().Get("token") == token {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
		})
	}
}
```

- [ ] **Step 5: Insert into middleware chain**

In the `New()` function, insert auth middleware between rate limit and request logger:

```go
// Build middleware chain: outermost runs first.
// Recovery → RequestID → CSP → BodyLimit → RateLimit → Auth → RequestLogger → mux
handler := http.Handler(s.mux)
if s.metrics != nil {
	handler = requestLoggerMiddleware(s.metrics)(handler)
}
if s.authToken != "" {
	handler = authMiddleware(s.authToken)(handler)
}
handler = rateLimitMiddleware(60)(handler)
handler = bodyLimitMiddleware(64 << 10)(handler)
handler = cspMiddleware(handler)
handler = requestIDMiddleware(handler)
handler = recoveryMiddleware(handler)
```

- [ ] **Step 6: Pass auth token from main.go**

In `cmd/chat/main.go`, add env var reading and pass to server:

```go
authToken := os.Getenv("SOUL_V2_AUTH_TOKEN")
// ... in serverOpts ...
if authToken != "" {
	serverOpts = append(serverOpts, server.WithAuthToken(authToken))
}
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./internal/chat/server/ -run "TestAuthMiddleware" -v
# Expected: all 4 tests PASS
```

- [ ] **Step 8: Commit**

```bash
git add internal/chat/server/server.go internal/chat/server/server_test.go cmd/chat/main.go
git commit -m "feat: add bearer token auth middleware

Protects /api/* and /ws with SOUL_V2_AUTH_TOKEN env var.
Static assets exempt. Disabled when token is empty (local dev).
Supports Authorization header and ?token= query param (for WS)."
```

---

## Task 5: Auth — Frontend Token Gate

**Files:**
- Create: `web/src/components/AuthGate.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/ws.ts`
- Modify: `web/src/hooks/useWebSocket.ts`
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Create AuthGate component**

Create `web/src/components/AuthGate.tsx`:

```tsx
import { useState, useEffect, type ReactNode } from 'react';

const TOKEN_KEY = 'soul-v2-token';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  window.location.reload();
}

export function AuthGate({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(getToken());
  const [input, setInput] = useState('');

  useEffect(() => {
    const handler = () => setToken(null);
    window.addEventListener('soul-v2-auth-failed', handler);
    return () => window.removeEventListener('soul-v2-auth-failed', handler);
  }, []);

  if (token) return <>{children}</>;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim()) {
      localStorage.setItem(TOKEN_KEY, input.trim());
      setToken(input.trim());
    }
  };

  return (
    <div data-testid="auth-gate" className="h-screen flex items-center justify-center bg-base">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4 p-8 bg-elevated rounded-xl border border-border-default max-w-sm w-full">
        <h2 className="text-lg font-semibold text-fg text-center">Soul v2</h2>
        <p className="text-sm text-fg-muted text-center">Enter access token to continue</p>
        <input
          data-testid="auth-token-input"
          type="password"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder="Access token"
          className="px-3 py-2 bg-surface border border-border-default rounded-lg text-fg text-sm focus:outline-none focus:border-soul"
          autoFocus
        />
        <button
          data-testid="auth-submit"
          type="submit"
          className="px-4 py-2 bg-soul text-white rounded-lg text-sm font-medium hover:opacity-90"
        >
          Continue
        </button>
      </form>
    </div>
  );
}
```

- [ ] **Step 2: Update api.ts to include auth header and handle 401**

Replace `web/src/lib/api.ts`:

```typescript
import { reportError } from './telemetry';
import { getToken, clearToken } from '../components/AuthGate';

const BASE = '';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  try {
    const token = getToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const res = await fetch(`${BASE}${path}`, {
      headers,
      ...init,
    });
    if (res.status === 401) {
      clearToken();
      throw new Error('Unauthorized');
    }
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      const err = new Error(body.error || `HTTP ${res.status}`);
      reportError(`api.${init?.method || 'GET'}`, err);
      throw err;
    }
    if (res.status === 204) return undefined as T;
    return res.json();
  } catch (err) {
    if (err instanceof TypeError) {
      reportError('api.network', err);
    }
    throw err;
  }
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: (path: string) => request<void>(path, { method: 'DELETE' }),
};
```

- [ ] **Step 3: Update ws.ts to include token in WS URL**

Replace `web/src/lib/ws.ts`:

```typescript
import { getToken } from '../components/AuthGate';

/**
 * WebSocket URL helper — computes the correct ws:// or wss:// URL
 * based on the current page protocol and host, with auth token.
 */
export function getWebSocketURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const token = getToken();
  const params = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${proto}//${window.location.host}/ws${params}`;
}
```

- [ ] **Step 4: Wrap app in AuthGate in main.tsx**

Update `web/src/main.tsx`:

```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router';
import { ErrorBoundary } from './components/ErrorBoundary';
import { AuthGate } from './components/AuthGate';
import { ChatProvider } from './contexts/ChatContext';
import { router } from './router';
import './app.css';

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <ErrorBoundary>
        <AuthGate>
          <ChatProvider>
            <RouterProvider router={router} />
          </ChatProvider>
        </AuthGate>
      </ErrorBoundary>
    </StrictMode>,
  );
}

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {
    // Service worker registration failed — app works without it
  });
}
```

- [ ] **Step 5: Verify TypeScript compiles**

```bash
cd /home/rishav/soul-v2/web && npx tsc --noEmit
# Expected: no errors
```

- [ ] **Step 6: Build frontend**

```bash
cd /home/rishav/soul-v2/web && npx vite build
# Expected: build succeeds
```

- [ ] **Step 7: Commit**

```bash
git add web/src/components/AuthGate.tsx web/src/lib/api.ts web/src/lib/ws.ts web/src/main.tsx
git commit -m "feat: add frontend auth gate with token prompt

Shows token input on first visit, stores in localStorage.
All API calls include Authorization header. WS URL includes
?token= param. On 401, clears token and re-prompts."
```

---

## Task 6: Security Headers

**Files:**
- Modify: `internal/chat/server/server.go`
- Modify: `internal/chat/server/server_test.go`

- [ ] **Step 1: Write failing test for new headers**

Add to `internal/chat/server/server_test.go`:

```go
func TestSecurityHeaders_HSTS(t *testing.T) {
	srv := New()
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	hsts := resp.Header.Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header when X-Forwarded-Proto is https")
	}
}

func TestSecurityHeaders_ReferrerPolicy(t *testing.T) {
	srv := New()
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Referrer-Policy") == "" {
		t.Error("expected Referrer-Policy header")
	}
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	srv := New()
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Permissions-Policy") == "" {
		t.Error("expected Permissions-Policy header")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/chat/server/ -run "TestSecurityHeaders" -v
# Expected: FAIL
```

- [ ] **Step 3: Rename and extend the middleware**

In `internal/chat/server/server.go`, rename `cspMiddleware` to `securityHeadersMiddleware` and add headers:

```go
// securityHeadersMiddleware sets all security headers on every response.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// HSTS only when behind TLS (Cloudflare tunnel or direct TLS)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
```

Update the middleware chain reference from `cspMiddleware` to `securityHeadersMiddleware`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/chat/server/ -run "TestSecurityHeaders" -v
# Expected: all 3 PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/chat/server/server.go internal/chat/server/server_test.go
git commit -m "feat: add HSTS, Referrer-Policy, Permissions-Policy headers

Rename cspMiddleware → securityHeadersMiddleware. Add HSTS
(conditional on HTTPS), Referrer-Policy, Permissions-Policy.
Matches OWASP recommendations."
```

---

## Task 7: Deploy + Systemd + Verify

**Files:**
- Modify: `/etc/systemd/system/soul-v2.service`

- [ ] **Step 1: Store auth token in Vaultwarden**

```bash
export NODE_TLS_REJECT_UNAUTHORIZED=0
# Generate a random 32-char token
TOKEN=$(openssl rand -hex 16)
echo "Token: $TOKEN"
# Store in Vaultwarden via bw CLI (or manually via web UI)
```

- [ ] **Step 2: Add token to systemd service**

```bash
sudo systemctl edit soul-v2
# Add under [Service]:
# Environment=SOUL_V2_AUTH_TOKEN=<token-from-step-1>
```

Or edit `/etc/systemd/system/soul-v2.service` directly to add the Environment line.

- [ ] **Step 3: Build everything**

```bash
cd /home/rishav/soul-v2
go build -o soul-chat ./cmd/chat
cd web && npx vite build
```

- [ ] **Step 4: Deploy**

```bash
sudo kill $(pgrep soul-chat) 2>/dev/null
sleep 1
sudo systemctl daemon-reload
sudo systemctl start soul-v2
sleep 2
ss -tlnp | grep 3002
# Expected: *:3002
```

- [ ] **Step 5: Verify auth middleware**

```bash
# Without token — 401
curl -s -o /dev/null -w "%{http_code}" http://localhost:3002/api/sessions
# Expected: 401

# With token — 200
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:3002/api/sessions
# Expected: 200

# SPA without token — 200 (exempt)
curl -s -o /dev/null -w "%{http_code}" http://localhost:3002/
# Expected: 200
```

- [ ] **Step 6: Verify WS auth**

```bash
# WS without token — 401
curl -s -o /dev/null -w "%{http_code}" \
  -H 'Origin: http://localhost:3002' \
  -H 'Upgrade: websocket' -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -H 'Sec-WebSocket-Version: 13' \
  http://localhost:3002/ws
# Expected: 401

# WS with token — 101
curl -s -o /dev/null -w "%{http_code}" \
  -H 'Origin: http://localhost:3002' \
  -H 'Upgrade: websocket' -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -H 'Sec-WebSocket-Version: 13' \
  "http://localhost:3002/ws?token=$TOKEN"
# Expected: 101
```

- [ ] **Step 7: Verify security headers**

```bash
curl -sI http://localhost:3002/ | grep -iE "referrer-policy|permissions-policy|x-frame|x-content-type|content-security"
# Expected: all 5 headers present
```

- [ ] **Step 8: E2E — MacBook Safari via Cloudflare tunnel**

1. Open `https://soul.rishavchatterjee.com` in Safari
2. Cloudflare Access shows login — enter email, receive OTP, submit
3. Soul v2 loads — AuthGate shows token prompt
4. Enter the token from Step 1
5. Chat page loads, sessions appear, send a test message
6. Verify WebSocket connects (green connection indicator)

- [ ] **Step 9: Commit deploy notes**

```bash
git add -A
git commit -m "chore: security hardening deployment notes"
```

---

## Security Checklist (Post-Deploy)

- [ ] `curl https://soul.rishavchatterjee.com/` returns Cloudflare login (302), not Soul UI
- [ ] `curl http://localhost:3002/api/sessions` returns 401
- [ ] `ls -la ~/.soul-v2/` shows 700/600 permissions
- [ ] `grep noTLSVerify /etc/cloudflared/config.yml` returns nothing
- [ ] Empty-Origin WS from public IP returns 403
- [ ] Response headers include HSTS, Referrer-Policy, Permissions-Policy
- [ ] MacBook Safari: full flow works (CF login → token → chat)
- [ ] Tailscale: `http://100.116.180.112:3002` requires token, works after entering it
