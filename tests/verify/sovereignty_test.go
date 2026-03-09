package verify_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSovereigntyAudit scans frontend source for external network requests.
// All fetch/WebSocket calls must use relative URLs. No external dependencies.
func TestSovereigntyAudit(t *testing.T) {
	webSrc := findWebSrc(t)

	var files []string
	err := filepath.Walk(webSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".ts" || ext == ".tsx" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking web/src: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no .ts/.tsx files found — check path")
	}

	// Patterns that indicate external network requests
	externalFetch := regexp.MustCompile(`fetch\s*\(\s*['"\x60]https?://`)
	externalWS := regexp.MustCompile(`new\s+WebSocket\s*\(\s*['"\x60](wss?://|https?://)`)
	xmlHttpReq := regexp.MustCompile(`XMLHttpRequest`)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("reading %s: %v", f, err)
			continue
		}

		rel, _ := filepath.Rel(webSrc, f)
		lines := strings.Split(string(data), "\n")

		for i, line := range lines {
			lineNum := i + 1
			trimmed := strings.TrimSpace(line)

			// Skip comments
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}

			if externalFetch.MatchString(line) {
				t.Errorf("%s:%d: external fetch() call: %s", rel, lineNum, trimmed)
			}
			if externalWS.MatchString(line) {
				t.Errorf("%s:%d: external WebSocket connection: %s", rel, lineNum, trimmed)
			}
			if xmlHttpReq.MatchString(line) {
				t.Errorf("%s:%d: XMLHttpRequest usage (use fetch instead): %s", rel, lineNum, trimmed)
			}
		}
	}
}

// TestServiceWorkerExists verifies the service worker file is present.
func TestServiceWorkerExists(t *testing.T) {
	repoRoot := findRepoRoot(t)

	swPath := filepath.Join(repoRoot, "web", "public", "sw.js")
	if _, err := os.Stat(swPath); os.IsNotExist(err) {
		t.Fatal("service worker missing: web/public/sw.js")
	}
}

// TestServiceWorkerRegistered verifies main.tsx registers the service worker.
func TestServiceWorkerRegistered(t *testing.T) {
	webSrc := findWebSrc(t)

	mainPath := filepath.Join(webSrc, "main.tsx")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("reading main.tsx: %v", err)
	}

	if !strings.Contains(string(data), "serviceWorker.register") {
		t.Fatal("main.tsx does not register a service worker")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file location to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod)")
		}
		dir = parent
	}
}

func findWebSrc(t *testing.T) string {
	t.Helper()
	root := findRepoRoot(t)
	src := filepath.Join(root, "web", "src")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Fatalf("web/src not found at %s", src)
	}
	return src
}
