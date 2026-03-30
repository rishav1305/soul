package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisteredProducts_HasExpectedProducts(t *testing.T) {
	products := registeredProducts()
	if len(products) < 10 {
		t.Errorf("expected at least 10 products, got %d", len(products))
	}

	// Verify core products exist.
	names := make(map[string]bool)
	for _, p := range products {
		names[p.Name] = true
	}
	for _, expected := range []string{"soul", "tasks", "tutor", "projects", "observe", "scout", "sentinel", "bench", "mesh"} {
		if !names[expected] {
			t.Errorf("missing expected product: %s", expected)
		}
	}
}

func TestRegisteredProducts_AllHaveRequiredFields(t *testing.T) {
	for _, p := range registeredProducts() {
		if p.Name == "" {
			t.Error("product has empty name")
		}
		if p.Label == "" {
			t.Errorf("product %q has empty label", p.Name)
		}
		if p.Tools <= 0 {
			t.Errorf("product %q has %d tools (should be > 0)", p.Name, p.Tools)
		}
		if p.Icon == "" {
			t.Errorf("product %q has no icon", p.Name)
		}
	}
}

func TestRegisteredProducts_ScoutHasMostTools(t *testing.T) {
	var scoutTools int
	for _, p := range registeredProducts() {
		if p.Name == "scout" {
			scoutTools = p.Tools
			break
		}
	}
	if scoutTools < 50 {
		t.Errorf("expected scout to have 50+ tools, got %d", scoutTools)
	}
}

func TestHandleProducts(t *testing.T) {
	srv := New(WithPort(0))
	req := httptest.NewRequest("GET", "/api/products", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var products []productInfo
	if err := json.NewDecoder(rec.Body).Decode(&products); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(products) < 10 {
		t.Errorf("expected at least 10 products, got %d", len(products))
	}
}
