package server

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:static
var staticFiles embed.FS

// spaHandler serves embedded static files and falls back to index.html
// for any route that does not match a real file (SPA client-side routing).
// Prefers on-disk web/dist/ when available (so autonomous agent builds take
// effect immediately without a Go recompile). Falls back to embedded webFS,
// then to the placeholder static files.
func (s *Server) spaHandlerFromFS() http.Handler {
	// Prefer on-disk web/dist/ so vite build changes are picked up live.
	if s.projectRoot != "" {
		diskDist := filepath.Join(s.projectRoot, "web", "dist")
		if info, err := os.Stat(filepath.Join(diskDist, "index.html")); err == nil && !info.IsDir() {
			log.Printf("[spa] serving frontend from disk: %s", diskDist)
			return newSPAFileServer(os.DirFS(diskDist))
		}
	}

	var sub fs.FS
	if s.webFS != nil {
		sub = s.webFS
	} else {
		var err error
		sub, err = fs.Sub(staticFiles, "static")
		if err != nil {
			panic("failed to create sub filesystem for static files: " + err.Error())
		}
	}
	return newSPAFileServer(sub)
}

// spaHandler serves from the placeholder static/ embed (used when no webFS is set).
func spaHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to create sub filesystem for static files: " + err.Error())
	}
	return newSPAFileServer(sub)
}

// devProxyHandler returns a reverse proxy to the Vite dev server.
// In dev mode, all non-API requests are proxied to Vite for hot reload.
func devProxyHandler(viteAddr string) http.Handler {
	target, err := url.Parse(viteAddr)
	if err != nil {
		panic("invalid Vite dev server address: " + err.Error())
	}
	return httputil.NewSingleHostReverseProxy(target)
}

func newSPAFileServer(sub fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file. If it exists, serve it directly.
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
