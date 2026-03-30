package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	s := New()
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestHandleToolExecute_Valid(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/dataeng__analyze/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleToolExecute_Invalid(t *testing.T) {
	s := New()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"input": map[string]string{}})
	resp, err := http.Post(ts.URL+"/api/tools/unknown/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestWithOptions(t *testing.T) {
	s := New(WithHost("0.0.0.0"), WithPort(9999))
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", s.host, "0.0.0.0")
	}
	if s.port != 9999 {
		t.Errorf("port = %d, want 9999", s.port)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "missing")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
