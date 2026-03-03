package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/rishav1305/soul/internal/planner"
)

// VerificationResult holds E2E check results for a task.
type VerificationResult struct {
	Passed      bool                `json:"passed"`
	Checks      []VerificationCheck `json:"checks"`
	Screenshots []string            `json:"screenshots"` // MinIO keys
}

// VerificationCheck is a single pass/fail check.
type VerificationCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// verifyTask runs E2E checks against the dev server.
// Returns nil if verification cannot be run (no browser available).
func (tp *TaskProcessor) verifyTask(ctx context.Context, task planner.Task) *VerificationResult {
	result := &VerificationResult{Passed: true}
	devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)

	// Try to launch Rod browser.
	u, err := launcher.ResolveURL("")
	if err != nil {
		log.Printf("[verify] no browser available: %v — skipping verification", err)
		return nil
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		log.Printf("[verify] failed to connect to browser: %v — skipping verification", err)
		return nil
	}
	defer browser.Close()

	// Create a new page.
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_create", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
		return result
	}

	// Navigate to dev server.
	err = page.Timeout(30 * time.Second).Navigate(devURL)
	if err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "navigate", Passed: false,
			Detail: fmt.Sprintf("Failed to navigate to %s: %v", devURL, err),
		})
		result.Passed = false
		return result
	}

	// Wait for page load.
	err = page.Timeout(15 * time.Second).WaitStable(500 * time.Millisecond)
	if err != nil {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_load", Passed: false, Detail: err.Error(),
		})
		result.Passed = false
	} else {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "page_load", Passed: true, Detail: "Dev server loaded successfully",
		})
	}

	// Take full-page screenshot.
	screenshot, err := page.Screenshot(true, nil)
	if err == nil && tp.server.minioClient != nil {
		key := fmt.Sprintf("tasks/%d/verification-%s.png",
			task.ID, time.Now().Format("20060102-150405"))
		if uploadErr := tp.server.minioClient.Upload(ctx, key, "image/png",
			bytes.NewReader(screenshot), int64(len(screenshot))); uploadErr != nil {
			log.Printf("[verify] failed to upload screenshot: %v", uploadErr)
		} else {
			result.Screenshots = append(result.Screenshots, key)
		}
	} else if err != nil {
		log.Printf("[verify] screenshot failed: %v", err)
	}

	// Determine overall pass/fail.
	for _, check := range result.Checks {
		if !check.Passed {
			result.Passed = false
			break
		}
	}

	return result
}
