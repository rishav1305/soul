package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:static
var staticFiles embed.FS

// spaHandler serves embedded static files and falls back to index.html
// for any route that does not match a real file (SPA client-side routing).
func spaHandler() http.Handler {
	// Strip the "static" prefix so files are served from root.
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to create sub filesystem for static files: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean path: remove leading slash for fs lookup.
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
