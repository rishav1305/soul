package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

	// Find system Chromium/Chrome binary.
	browserPath, found := launcher.LookPath()
	if !found {
		log.Printf("[verify] no browser binary found — skipping verification")
		return nil
	}
	log.Printf("[verify] using browser: %s", browserPath)

	// Launch a headless browser for verification.
	l, err := launcher.New().Bin(browserPath).
		Headless(true).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Launch()
	if err != nil {
		log.Printf("[verify] failed to launch browser: %v — skipping verification", err)
		return nil
	}

	browser := rod.New().ControlURL(l)
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

	// Check for JavaScript errors in the console.
	jsErrors := tp.checkConsoleErrors(page)
	if jsErrors != "" {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "js_errors", Passed: false, Detail: jsErrors,
		})
	} else {
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "js_errors", Passed: true, Detail: "No JavaScript errors detected",
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

	// Task-specific verification: use AI to check if the task's changes are visible.
	if tp.ai != nil {
		tp.verifyTaskSpecific(ctx, page, task, result)
	}

	// Determine overall pass/fail.
	result.Passed = true
	for _, check := range result.Checks {
		if !check.Passed {
			result.Passed = false
			break
		}
	}

	return result
}

// checkConsoleErrors evaluates JavaScript console errors on the page.
func (tp *TaskProcessor) checkConsoleErrors(page *rod.Page) string {
	// Inject a script to capture console errors.
	obj, err := page.Eval(`() => {
		if (window.__soulErrors && window.__soulErrors.length > 0) {
			return window.__soulErrors.join('\n');
		}
		return '';
	}`)
	if err != nil {
		return ""
	}
	return obj.Value.Str()
}

// verifyTaskSpecific uses the AI to check whether the task's changes are present on the page.
func (tp *TaskProcessor) verifyTaskSpecific(ctx context.Context, page *rod.Page, task planner.Task, result *VerificationResult) {
	// Extract visible text from the page.
	pageText, err := page.Eval(`() => document.body.innerText`)
	if err != nil {
		log.Printf("[verify] failed to get page text: %v", err)
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "task_specific", Passed: false, Detail: "Failed to extract page text for verification",
		})
		return
	}

	bodyText := pageText.Value.Str()
	// Truncate to avoid huge prompts.
	if len(bodyText) > 4000 {
		bodyText = bodyText[:4000] + "\n...(truncated)"
	}

	// Also get the page HTML structure (just tag outlines, not full HTML).
	htmlStructure, err := page.Eval(`() => {
		function summarize(el, depth) {
			if (depth > 3) return '';
			let tag = el.tagName ? el.tagName.toLowerCase() : '';
			if (!tag) return '';
			let classes = el.className && typeof el.className === 'string' ? '.' + el.className.split(' ').filter(Boolean).slice(0, 3).join('.') : '';
			let children = [];
			for (let child of el.children) {
				let s = summarize(child, depth + 1);
				if (s) children.push(s);
			}
			if (children.length > 0) {
				return tag + classes + '{' + children.join(',') + '}';
			}
			return tag + classes;
		}
		return summarize(document.body, 0);
	}`)

	structureStr := ""
	if err == nil {
		structureStr = htmlStructure.Value.Str()
		if len(structureStr) > 2000 {
			structureStr = structureStr[:2000] + "...(truncated)"
		}
	}

	// Build the verification prompt for the AI.
	prompt := tp.buildVerificationPrompt(task, bodyText, structureStr)

	// Call the AI with a lightweight model for quick verification.
	verifyResult, err := tp.ai.CompleteSimple(ctx, "claude-haiku-4-5-20251001", prompt)
	if err != nil {
		log.Printf("[verify] AI verification call failed: %v", err)
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "task_specific", Passed: false,
			Detail: fmt.Sprintf("AI verification unavailable: %v", err),
		})
		return
	}

	// Parse the AI's response — expect JSON with passed and details.
	tp.parseVerificationResponse(verifyResult, result)
}

// buildVerificationPrompt creates a prompt for the AI to verify task-specific changes.
func (tp *TaskProcessor) buildVerificationPrompt(task planner.Task, pageText, htmlStructure string) string {
	var b strings.Builder
	b.WriteString("You are verifying whether a UI task's changes are visible on a web page.\n\n")
	fmt.Fprintf(&b, "## Task\n")
	fmt.Fprintf(&b, "**Title:** %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "**Description:** %s\n", task.Description)
	}
	if task.Acceptance != "" {
		fmt.Fprintf(&b, "**Acceptance Criteria:** %s\n", task.Acceptance)
	}

	b.WriteString("\n## Page Content (visible text)\n```\n")
	b.WriteString(pageText)
	b.WriteString("\n```\n")

	if htmlStructure != "" {
		b.WriteString("\n## Page Structure (DOM outline)\n```\n")
		b.WriteString(htmlStructure)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n## Instructions\n")
	b.WriteString("Based on the task title, description, and acceptance criteria, determine whether the expected UI changes are visible on the page.\n\n")
	b.WriteString("For frontend/UI tasks, check:\n")
	b.WriteString("- Are the expected UI elements present in the page text?\n")
	b.WriteString("- Do CSS class names or component structure suggest the changes were applied?\n")
	b.WriteString("- For backend-only tasks (no UI changes expected), mark as passed.\n\n")
	b.WriteString("Respond with ONLY a JSON object (no markdown fences):\n")
	b.WriteString(`{"passed": true/false, "checks": [{"name": "check_name", "passed": true/false, "detail": "explanation"}]}`)
	b.WriteString("\n\nBe specific about what you looked for and whether you found it. If the task is backend-only with no expected UI changes, return passed=true with a note.\n")

	return b.String()
}

// parseVerificationResponse parses the AI's JSON response into verification checks.
func (tp *TaskProcessor) parseVerificationResponse(response string, result *VerificationResult) {
	// Try to extract JSON from the response (AI might wrap it in markdown).
	response = strings.TrimSpace(response)
	if idx := strings.Index(response, "{"); idx >= 0 {
		response = response[idx:]
	}
	if idx := strings.LastIndex(response, "}"); idx >= 0 {
		response = response[:idx+1]
	}

	var aiResult struct {
		Passed bool `json:"passed"`
		Checks []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}

	if err := json.Unmarshal([]byte(response), &aiResult); err != nil {
		log.Printf("[verify] failed to parse AI verification response: %v", err)
		log.Printf("[verify] response was: %.500s", response)
		result.Checks = append(result.Checks, VerificationCheck{
			Name: "task_specific", Passed: false,
			Detail: "Failed to parse AI verification response",
		})
		return
	}

	for _, check := range aiResult.Checks {
		result.Checks = append(result.Checks, VerificationCheck{
			Name:   "task:" + check.Name,
			Passed: check.Passed,
			Detail: check.Detail,
		})
	}
}
