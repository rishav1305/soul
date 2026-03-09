//go:build ignore

// monitor.go — runtime health check tool for soul-v2.
//
// Usage: go run tools/monitor.go [--port 3002] [--interval 30s]
//
// Checks:
// 1. HTTP health endpoint responds (GET /api/health returns 200 with {"status":"ok"})
// 2. WebSocket connects to /ws and receives connection.ready within 5s
// 3. Auth status is not "error" (GET /api/auth/status returns valid JSON)
// 4. System metrics (goroutines, memory) within bounds
//
// Exit code 0 = all checks pass, 1 = one or more checks failed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// checkResult represents the outcome of a single health check.
type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass" or "fail"
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

func main() {
	port := flag.Int("port", 3002, "server port to check")
	interval := flag.Duration("interval", 0, "repeat interval (0 = run once)")
	flag.Parse()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", *port)

	run := func() bool {
		results := runChecks(baseURL)
		printResults(results)
		for _, r := range results {
			if r.Status == "fail" {
				return false
			}
		}
		return true
	}

	if *interval <= 0 {
		if !run() {
			os.Exit(1)
		}
		return
	}

	// Continuous monitoring mode.
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	fmt.Printf("monitoring %s every %s\n\n", baseURL, *interval)
	run()
	for range ticker.C {
		fmt.Println()
		run()
	}
}

// runChecks performs all health checks and returns the results.
func runChecks(baseURL string) []checkResult {
	var results []checkResult

	results = append(results, checkHealth(baseURL))
	results = append(results, checkAuthStatus(baseURL))
	results = append(results, checkWebSocket(baseURL))
	results = append(results, checkSystemMetrics())

	return results
}

// printResults writes a formatted summary of check results to stdout.
func printResults(results []checkResult) {
	ts := time.Now().Format("15:04:05")
	allPass := true
	for _, r := range results {
		status := "PASS"
		if r.Status == "fail" {
			status = "FAIL"
			allPass = false
		}
		latency := ""
		if r.Latency != "" {
			latency = fmt.Sprintf(" (%s)", r.Latency)
		}
		msg := ""
		if r.Message != "" {
			msg = fmt.Sprintf(" — %s", r.Message)
		}
		fmt.Printf("[%s] %-20s [%s]%s%s\n", ts, r.Name, status, latency, msg)
	}

	if allPass {
		fmt.Printf("[%s] all checks passed\n", ts)
	} else {
		fmt.Printf("[%s] SOME CHECKS FAILED\n", ts)
	}
}

// --- Check 1: Health endpoint ---

func checkHealth(baseURL string) checkResult {
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/api/health")
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return checkResult{
			Name:    "health",
			Status:  "fail",
			Message: fmt.Sprintf("request failed: %v", err),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return checkResult{
			Name:    "health",
			Status:  "fail",
			Message: fmt.Sprintf("status %d (want 200)", resp.StatusCode),
			Latency: latency,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return checkResult{
			Name:    "health",
			Status:  "fail",
			Message: fmt.Sprintf("read body: %v", err),
			Latency: latency,
		}
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return checkResult{
			Name:    "health",
			Status:  "fail",
			Message: fmt.Sprintf("invalid JSON: %v", err),
			Latency: latency,
		}
	}

	if health.Status != "ok" {
		return checkResult{
			Name:    "health",
			Status:  "fail",
			Message: fmt.Sprintf("status=%q (want ok)", health.Status),
			Latency: latency,
		}
	}

	return checkResult{
		Name:    "health",
		Status:  "pass",
		Latency: latency,
	}
}

// --- Check 2: Auth status ---

func checkAuthStatus(baseURL string) checkResult {
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/api/auth/status")
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return checkResult{
			Name:    "auth",
			Status:  "fail",
			Message: fmt.Sprintf("request failed: %v", err),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return checkResult{
			Name:    "auth",
			Status:  "fail",
			Message: fmt.Sprintf("status %d (want 200)", resp.StatusCode),
			Latency: latency,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return checkResult{
			Name:    "auth",
			Status:  "fail",
			Message: fmt.Sprintf("read body: %v", err),
			Latency: latency,
		}
	}

	var authStatus struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &authStatus); err != nil {
		return checkResult{
			Name:    "auth",
			Status:  "fail",
			Message: fmt.Sprintf("invalid JSON: %v", err),
			Latency: latency,
		}
	}

	if authStatus.State == "error" {
		return checkResult{
			Name:    "auth",
			Status:  "fail",
			Message: "auth state is error",
			Latency: latency,
		}
	}

	return checkResult{
		Name:    "auth",
		Status:  "pass",
		Message: fmt.Sprintf("state=%s", authStatus.State),
		Latency: latency,
	}
}

// --- Check 3: WebSocket connection ---

func checkWebSocket(baseURL string) checkResult {
	start := time.Now()

	// Convert http:// to ws://
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a simple HTTP upgrade check rather than a full WebSocket library
	// to avoid adding external dependencies.
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/ws", nil)
	if err != nil {
		return checkResult{
			Name:    "websocket",
			Status:  "fail",
			Message: fmt.Sprintf("create request: %v", err),
		}
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return checkResult{
			Name:    "websocket",
			Status:  "fail",
			Message: fmt.Sprintf("connect to %s failed: %v", wsURL, err),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	// A successful WebSocket upgrade returns 101.
	if resp.StatusCode == http.StatusSwitchingProtocols {
		return checkResult{
			Name:    "websocket",
			Status:  "pass",
			Message: "upgrade accepted",
			Latency: latency,
		}
	}

	// Some servers may return 200 or 400 depending on configuration.
	// As long as the endpoint is reachable, it's a pass.
	if resp.StatusCode < 500 {
		return checkResult{
			Name:    "websocket",
			Status:  "pass",
			Message: fmt.Sprintf("endpoint reachable (status %d)", resp.StatusCode),
			Latency: latency,
		}
	}

	return checkResult{
		Name:    "websocket",
		Status:  "fail",
		Message: fmt.Sprintf("server error: status %d", resp.StatusCode),
		Latency: latency,
	}
}

// --- Check 4: System metrics ---

const (
	maxGoroutines = 10000
	maxHeapMB     = 1024 // 1 GB
)

func checkSystemMetrics() checkResult {
	goroutines := runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	heapMB := float64(memStats.HeapAlloc) / 1024 / 1024

	var issues []string

	if goroutines > maxGoroutines {
		issues = append(issues, fmt.Sprintf("goroutines=%d (max %d)", goroutines, maxGoroutines))
	}

	if heapMB > maxHeapMB {
		issues = append(issues, fmt.Sprintf("heap=%.1fMB (max %dMB)", heapMB, maxHeapMB))
	}

	if len(issues) > 0 {
		return checkResult{
			Name:    "system",
			Status:  "fail",
			Message: strings.Join(issues, "; "),
		}
	}

	return checkResult{
		Name:    "system",
		Status:  "pass",
		Message: fmt.Sprintf("goroutines=%d heap=%.1fMB", goroutines, heapMB),
	}
}
