package ws

import (
	"net/http/httptest"
	"testing"
)

func TestIsPrivateOrTrustedIP(t *testing.T) {
	tests := []struct {
		addr    string
		private bool
	}{
		{"127.0.0.1:1234", true},
		{"192.168.0.100:1234", true},
		{"10.0.0.5:1234", true},
		{"172.16.0.1:1234", true},
		{"100.116.180.93:1234", true},  // Tailscale
		{"100.64.0.1:1234", true},      // Tailscale start
		{"100.127.255.255:1234", true},  // Tailscale end
		{"203.0.113.50:12345", false},   // Public IP
		{"8.8.8.8:53", false},           // Google DNS
		{"100.128.0.1:1234", false},     // Just outside Tailscale
	}
	for _, tt := range tests {
		got := isPrivateOrTrustedIP(tt.addr)
		if got != tt.private {
			t.Errorf("isPrivateOrTrustedIP(%q) = %v, want %v", tt.addr, got, tt.private)
		}
	}
}

func TestIsOriginAllowed_EmptyOrigin_PublicIP(t *testing.T) {
	hub := NewHub()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "203.0.113.50:12345"
	if hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from public IP to be rejected")
	}
}

func TestIsOriginAllowed_EmptyOrigin_PrivateIP(t *testing.T) {
	hub := NewHub()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "192.168.0.100:12345"
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from private IP to be allowed")
	}
}

func TestIsOriginAllowed_EmptyOrigin_CloudflareJWT_PublicIP(t *testing.T) {
	hub := NewHub()
	// CF JWT header from public IP should NOT be trusted (spoofable on direct access)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "203.0.113.50:12345"
	req.Header.Set("Cf-Access-Jwt-Assertion", "eyJhbGciOi...")
	if hub.isOriginAllowed(req) {
		t.Error("expected CF JWT from public IP to be rejected (header spoofable)")
	}
}

func TestIsOriginAllowed_EmptyOrigin_CloudflaredLocalhost(t *testing.T) {
	hub := NewHub()
	// cloudflared connects from localhost — allowed via private IP check
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from localhost (cloudflared) to be allowed")
	}
}

func TestIsOriginAllowed_EmptyOrigin_TailscaleIP(t *testing.T) {
	hub := NewHub()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "100.116.180.93:12345"
	if !hub.isOriginAllowed(req) {
		t.Error("expected empty-origin from Tailscale IP to be allowed")
	}
}
