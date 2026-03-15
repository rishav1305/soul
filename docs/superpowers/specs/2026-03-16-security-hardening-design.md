# Security Hardening — Design Spec

**Date:** 2026-03-16
**Status:** Approved
**Threat model:** Full defense-in-depth (internet → LAN → compromised device)
**Approach:** Outside-in — fix outermost exposure first, work inward

## Context

A security audit on 2026-03-16 identified critical gaps in Soul v2's infrastructure:

- `soul.rishavchatterjee.com` is publicly accessible with zero authentication
- All `/api/*` endpoints and `/ws` are unauthenticated
- WebSocket origin validation allows clients with no Origin header
- Database files have world-readable permissions (0666)
- Cloudflared config has unnecessary `noTLSVerify: true`
- Missing HSTS, Referrer-Policy, and Permissions-Policy headers
- Portainer (Docker management UI) exposed on :9080

The system is a single-user home lab, but the Cloudflare tunnel creates internet-facing exposure that demands immediate hardening.

## Step 1: Cloudflare Access (Internet Gate)

**Goal:** Block all unauthenticated internet access to `soul.rishavchatterjee.com`.

**Implementation:**
- Create a Cloudflare Access application in the Zero Trust dashboard
- Hostname: `soul.rishavchatterjee.com`
- Policy: Allow only `rishav.chatt@gmail.com` via email one-time PIN
- Cloudflare sends a 6-digit code, user enters it, receives `CF_Authorization` cookie (24h TTL)
- All requests (HTTP + WebSocket upgrade) are blocked at Cloudflare's edge before reaching titan-pi

**WebSocket compatibility:** The initial HTTP upgrade carries the `CF_Authorization` cookie. Cloudflare proxies the upgraded connection after validating the cookie.

**No code changes required.**

**Verification:**
```bash
curl -s -o /dev/null -w "%{http_code}" https://soul.rishavchatterjee.com/
# Expected: 302 redirect to Cloudflare login page (not 200)
```

## Step 2: Infrastructure Hardening

### 2a: Fix File Permissions

**Problem:** `~/.soul-v2/` is `0777`, `chat.db` is `0666` — any system user can read/write session data.

**Fix:**
```bash
chmod 700 ~/.soul-v2/
chmod 600 ~/.soul-v2/*.db
chmod 600 ~/.soul-v2/*.jsonl
```

**Verification:**
```bash
ls -la ~/.soul-v2/
# Directory: drwx------
# DB files: -rw-------
```

### 2b: Clean Cloudflared Config

**Problem:** `noTLSVerify: true` in `/etc/cloudflared/config.yml` disables TLS verification. Since soul-v2 runs plain HTTP on localhost, this setting is unnecessary and misleading.

**Fix:** Remove the `originRequest` block from config:

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

Then `sudo systemctl restart cloudflared`.

### 2c: Restrict Portainer Exposure

**Problem:** Port 9080 proxies to Portainer (Docker management UI) on titan-pc via nginx. Portainer has its own auth, but reducing exposure is defense-in-depth.

**Fix:**
```bash
sudo ufw delete allow 9080/tcp  # Remove broad access if it exists
sudo ufw allow from 192.168.0.145 to any port 9080 comment "Portainer from titan-pi only"
```

## Step 3: WebSocket Origin Hardening

**Problem:** `isOriginAllowed()` in `hub.go` returns `true` when the Origin header is missing, allowing curl/scripts to bypass origin validation entirely.

**Current code (hub.go:303-305):**
```go
if origin == "" {
    // No origin header — non-browser client, allow by default.
    return true
}
```

**New logic:**
```go
if origin == "" {
    // Allow Cloudflare tunnel (has JWT header)
    if r.Header.Get("Cf-Access-Jwt-Assertion") != "" {
        return true
    }
    // Allow private network IPs (LAN/Tailscale)
    remoteIP := extractIP(r.RemoteAddr)
    if isPrivateIP(remoteIP) {
        return true
    }
    // Reject all other empty-origin requests
    return false
}
```

**Helper: `isPrivateIP(ip string) bool`** — checks if IP matches RFC 1918 ranges (192.168.x.x, 10.x.x.x, 172.16-31.x.x) or Tailscale CGNAT (100.64.0.0/10).

**File:** `internal/chat/ws/hub.go`

**Verification:**
```bash
# From internet (no Origin, no CF header, no private IP) — should fail
curl -s -o /dev/null -w "%{http_code}" \
  -H 'Upgrade: websocket' -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -H 'Sec-WebSocket-Version: 13' \
  http://localhost:3002/ws
# Expected: 403

# From localhost (private IP) — should work
curl -s -o /dev/null -w "%{http_code}" \
  -H 'Origin: http://localhost:3002' \
  -H 'Upgrade: websocket' -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -H 'Sec-WebSocket-Version: 13' \
  http://localhost:3002/ws
# Expected: 101
```

## Step 4: Application-Level Auth Middleware

**Goal:** Require bearer token for all API and WebSocket access, regardless of network path.

### Server Side

**Environment variable:** `SOUL_V2_AUTH_TOKEN`
- Loaded at startup in `cmd/chat/main.go`
- Stored in Vaultwarden, set in systemd service file
- If empty, middleware is skipped (backwards-compatible for local dev without auth)

**Middleware: `authMiddleware(token string)`**
- Inserted into chain between `rateLimitMiddleware` and `requestLoggerMiddleware`
- Checks `Authorization: Bearer <token>` header
- Exempt routes: `/` (SPA), `/assets/*`, `/manifest.json`, `/favicon.ico`
- Returns `401 Unauthorized` with `{"error": "unauthorized"}` on mismatch

**WebSocket auth:**
- Browsers cannot set custom headers on `new WebSocket(url)`
- Support token via query param: `ws://host/ws?token=<token>`
- `HandleUpgrade` in `hub.go` checks query param `token` before origin validation
- Rejects with 401 if token is invalid

**Files:**
- `internal/chat/server/server.go` — new `authMiddleware` function, insert into chain
- `internal/chat/ws/hub.go` — check `r.URL.Query().Get("token")` in `HandleUpgrade`
- `cmd/chat/main.go` — read `SOUL_V2_AUTH_TOKEN` env, pass to server
- `/etc/systemd/system/soul-v2.service` — add `Environment=SOUL_V2_AUTH_TOKEN=<token>`

### Frontend Side

**Token prompt:**
- On app mount, check `localStorage.getItem('soul-v2-token')`
- If missing, render a simple full-screen prompt: "Enter access token" with an input and submit button
- On submit, store in `localStorage` and proceed to app

**API calls (`lib/api.ts` or equivalent):**
- Add `Authorization: Bearer <token>` header to all `fetch()` calls
- On `401` response, clear `localStorage` token and re-prompt

**WebSocket (`hooks/useWebSocket.ts`):**
- Modify `getWebSocketURL()` or the `connect()` function to append `?token=<token>` to the WS URL
- On close with code 4001 (or similar auth error), clear token and re-prompt

**Files:**
- `web/src/lib/ws.ts` or `web/src/hooks/useWebSocket.ts` — append token to WS URL
- `web/src/lib/api.ts` — add auth header to fetch wrapper
- `web/src/main.tsx` or new `web/src/components/AuthGate.tsx` — token prompt component

**Verification:**
```bash
# No token — should 401
curl -s -o /dev/null -w "%{http_code}" http://localhost:3002/api/sessions
# Expected: 401

# Valid token — should 200
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer <token>" \
  http://localhost:3002/api/sessions
# Expected: 200

# WS with token — should 101
curl -s -o /dev/null -w "%{http_code}" \
  -H 'Origin: http://localhost:3002' \
  -H 'Upgrade: websocket' -H 'Connection: Upgrade' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -H 'Sec-WebSocket-Version: 13' \
  'http://localhost:3002/ws?token=<token>'
# Expected: 101
```

## Step 5: Security Headers

**Goal:** Add missing HTTP security headers to match OWASP recommendations.

**Implementation:** Rename `cspMiddleware` to `securityHeadersMiddleware` in `server.go`. Add:

| Header | Value | Purpose |
|--------|-------|---------|
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | Force HTTPS (only set when `X-Forwarded-Proto: https` or `r.TLS != nil`) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Prevent URL leakage in referrer |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Disable unused browser APIs |

**Existing headers (unchanged):**
- `Content-Security-Policy` — already strong
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`

**Not adding `X-XSS-Protection`** — deprecated, modern browsers use CSP. Can introduce vulnerabilities in older IE.

**File:** `internal/chat/server/server.go`

**Verification:**
```bash
curl -sI http://localhost:3002/ | grep -iE "strict-transport|referrer-policy|permissions-policy"
# Expected: all three headers present
```

## Files Modified (Complete List)

| File | Changes |
|------|---------|
| `internal/chat/server/server.go` | Add `authMiddleware`, rename `cspMiddleware` → `securityHeadersMiddleware`, add HSTS/Referrer/Permissions headers |
| `internal/chat/ws/hub.go` | Fix empty-Origin bypass, add WS token auth, add `isPrivateIP` helper |
| `cmd/chat/main.go` | Read `SOUL_V2_AUTH_TOKEN` env var, pass to server |
| `web/src/hooks/useWebSocket.ts` | Append `?token=` to WS URL |
| `web/src/lib/api.ts` (or equivalent) | Add `Authorization: Bearer` header to fetch calls |
| `web/src/main.tsx` or new `AuthGate.tsx` | Token prompt UI component |
| `/etc/cloudflared/config.yml` | Remove `noTLSVerify` |
| `/etc/systemd/system/soul-v2.service` | Add `SOUL_V2_AUTH_TOKEN` env var |
| `internal/chat/server/server_test.go` | Tests for auth middleware, security headers |
| `internal/chat/ws/hub_test.go` | Tests for WS origin + token validation |

## Execution Order

1. **Cloudflare Access** — browser config, 5 minutes, blocks internet attackers immediately
2. **File permissions** — CLI commands, 1 minute
3. **Cloudflared config** — edit + restart, 2 minutes
4. **Portainer UFW** — CLI command, 1 minute
5. **WS origin fix** — Go code change in hub.go
6. **Auth middleware** — Go code change in server.go + cmd/chat/main.go
7. **Frontend token gate** — React component + fetch/WS changes
8. **Security headers** — Go code change in server.go
9. **Build, deploy, verify** — `make build`, systemd restart, E2E test
10. **Final E2E** — MacBook Safari via Cloudflare tunnel: email OTP → token prompt → chat works
