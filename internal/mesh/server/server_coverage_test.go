package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/mesh/hub"
	"github.com/rishav1305/soul/internal/mesh/store"
)

func TestListenAndServe_Shutdown(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir + "/mesh_srv.db")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer st.Close()

	h := hub.New(st)
	info := testNodeInfo()
	srv := New(info, st, h, "test-secret")

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe("127.0.0.1", port)
	}()
	time.Sleep(100 * time.Millisecond)

	// Health check.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}

	// Shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	srvErr := <-errCh
	if srvErr != http.ErrServerClosed {
		t.Errorf("ListenAndServe error = %v, want http.ErrServerClosed", srvErr)
	}
}

func testMeshServerWithStore(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir + "/mesh.db")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	h := hub.New(st)
	info := testNodeInfo()
	srv := New(info, st, h, "secret")
	return srv, st
}

func TestHandleLink_ClosedStoreCov(t *testing.T) {
	srv, st := testMeshServerWithStore(t)
	st.Close()

	req := httptest.NewRequest("POST", "/api/mesh/link", nil)
	w := httptest.NewRecorder()
	srv.handleLink(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed store, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleStatus_ClosedStoreCov(t *testing.T) {
	srv, st := testMeshServerWithStore(t)
	st.Close()

	req := httptest.NewRequest("GET", "/api/mesh/status", nil)
	w := httptest.NewRecorder()
	srv.handleStatus(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for closed store, got %d; body: %s", w.Code, w.Body.String())
	}
}
