# Playwright E2E Merge Gates — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent Soul's autonomous agent from deploying broken frontend builds by adding Playwright smoke tests as merge gates before dev and master.

**Architecture:** Two gates — one before dev merge (autonomous pipeline), one before master merge (user-triggered "Done"). Each gate runs `tsc --noEmit` + `vite build` + 6-check Playwright smoke test via SSH to titan-pc. On failure, merge is reverted and previous working build restored.

**Tech Stack:** Go (gates), Node.js/Playwright (smoke test on titan-pc), SSH transport, React data-testid selectors.

---

### Task 1: Add data-testid attributes to layout components

**Files:**
- Modify: `web/src/components/layout/ProductRail.tsx:64`
- Modify: `web/src/components/layout/HorizontalRail.tsx:297`
- Modify: `web/src/components/chat/ChatPanel.tsx:23`

**Step 1: Add data-testid to ProductRail**

In `ProductRail.tsx` line 64, the outer div:
```tsx
// BEFORE:
<div className="w-14 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-1 shrink-0 z-10">

// AFTER:
<div data-testid="product-rail" className="w-14 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-1 shrink-0 z-10">
```

**Step 2: Add data-testid to HorizontalRail**

In `HorizontalRail.tsx` line 297, the expanded panel div:
```tsx
// BEFORE:
<div
  ref={railContainerRef}
  className="flex flex-col bg-surface shrink-0"

// AFTER:
<div
  ref={railContainerRef}
  data-testid="horizontal-rail"
  className="flex flex-col bg-surface shrink-0"
```

**Step 3: Add data-testid to ChatPanel**

In `ChatPanel.tsx` line 23, the outer div:
```tsx
// BEFORE:
<div className="flex flex-col h-full relative bg-surface">

// AFTER:
<div data-testid="chat-panel" className="flex flex-col h-full relative bg-surface">
```

**Step 4: Build to verify**

Run: `cd /home/rishav/soul/web && npx vite build`
Expected: Build succeeds with no errors.

**Step 5: Commit**

```bash
git add web/src/components/layout/ProductRail.tsx web/src/components/layout/HorizontalRail.tsx web/src/components/chat/ChatPanel.tsx
git commit -m "feat: add data-testid attributes for E2E smoke tests"
```

---

### Task 2: Add `smoke` action to test-runner.js

**Files:**
- Modify: `tools/e2e/test-runner.js` (add new action block after the `dom` action, before the `else` block)

**Step 1: Add the smoke action to test-runner.js**

After the `} else if (action === 'dom') { ... }` block (line 157) and before `} else {` (line 158), add a new `smoke` action. This action runs all 6 checks in one call and returns structured results.

```javascript
} else if (action === 'smoke') {
    var checks = [];

    // Check 1: Page loaded (we already navigated successfully above)
    checks.push({ name: 'page_load', pass: true, detail: 'Page loaded with HTTP 200' });

    // Check 2: No JS errors
    var jsErrors = await page.evaluate(function() {
      return window.__soulErrors || [];
    });
    var hasJsErrors = jsErrors.length > 0;
    checks.push({
      name: 'no_js_errors',
      pass: !hasJsErrors,
      detail: hasJsErrors ? 'JS errors: ' + jsErrors.join('; ').slice(0, 300) : 'No JavaScript errors'
    });

    // Check 3: React rendered
    var rootChildren = await page.evaluate(function() {
      var root = document.querySelector('#root');
      return root ? root.children.length : 0;
    });
    checks.push({
      name: 'react_rendered',
      pass: rootChildren > 0,
      detail: rootChildren > 0 ? '#root has ' + rootChildren + ' children' : '#root is empty — React failed to mount'
    });

    // Check 4: Key UI elements
    var testIds = ['product-rail', 'chat-panel', 'horizontal-rail'];
    for (var ti = 0; ti < testIds.length; ti++) {
      var tid = testIds[ti];
      var el = await page.$('[data-testid="' + tid + '"]');
      checks.push({
        name: 'ui_' + tid.replace(/-/g, '_'),
        pass: !!el,
        detail: el ? tid + ' found' : tid + ' NOT found'
      });
    }

    // Check 5: API health — fetch /api/tasks
    var apiOk = await page.evaluate(function() {
      return fetch('/api/tasks').then(function(r) {
        return { status: r.status, ok: r.ok };
      }).catch(function(e) {
        return { status: 0, ok: false, error: e.message };
      });
    });
    checks.push({
      name: 'api_health',
      pass: apiOk.ok,
      detail: apiOk.ok ? 'API returned ' + apiOk.status : 'API failed: status=' + apiOk.status + (apiOk.error ? ' ' + apiOk.error : '')
    });

    // Check 6: WebSocket connects
    var wsOk = await page.evaluate(function() {
      return new Promise(function(resolve) {
        try {
          var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
          var ws = new WebSocket(proto + '//' + location.host + '/ws');
          var timer = setTimeout(function() {
            ws.close();
            resolve(false);
          }, 5000);
          ws.onopen = function() {
            clearTimeout(timer);
            ws.close();
            resolve(true);
          };
          ws.onerror = function() {
            clearTimeout(timer);
            resolve(false);
          };
        } catch (e) {
          resolve(false);
        }
      });
    });
    checks.push({
      name: 'websocket',
      pass: wsOk,
      detail: wsOk ? 'WebSocket connected successfully' : 'WebSocket failed to connect within 5s'
    });

    var allPass = checks.every(function(c) { return c.pass; });
    console.log(JSON.stringify({ allPass: allPass, checks: checks }));
```

**Step 2: Inject JS error capture into page load**

The smoke test needs to capture JS errors that happen during page load (before our code runs). Add error capture injection right after `page.goto()` succeeds but note that errors during initial script execution are missed by late injection. Instead, we'll use Playwright's `page.on('pageerror')` approach.

Update the existing page load section at the top of `run()` to capture errors when the action is `smoke`:

```javascript
// After line 31 (page creation), before page.goto:
var pageErrors = [];
if (action === 'smoke') {
  page.on('pageerror', function(err) {
    pageErrors.push(err.message);
  });
}
```

And in the smoke action, replace the `__soulErrors` check with:

```javascript
// Check 2: No JS errors (captured via pageerror event)
var hasJsErrors = pageErrors.length > 0;
checks.push({
  name: 'no_js_errors',
  pass: !hasJsErrors,
  detail: hasJsErrors ? 'JS errors: ' + pageErrors.join('; ').slice(0, 300) : 'No JavaScript errors'
});
```

**Step 3: Deploy to titan-pc**

```bash
scp tools/e2e/test-runner.js titan-pc:~/soul-e2e/test-runner.js
```

**Step 4: Test the smoke action**

```bash
ssh titan-pc 'cd ~/soul-e2e && node test-runner.js smoke http://192.168.0.128:3000'
```

Expected: JSON with `allPass: true` and 8 checks (page_load, no_js_errors, react_rendered, ui_product_rail, ui_chat_panel, ui_horizontal_rail, api_health, websocket).

**Step 5: Commit**

```bash
git add tools/e2e/test-runner.js
git commit -m "feat: add smoke action to E2E test runner with 6 health checks"
```

---

### Task 3: Create gates.go — PreMergeGate and SmokeTest

**Files:**
- Create: `internal/server/gates.go`

**Step 1: Create the gates.go file**

```go
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SmokeCheck is one pass/fail entry from the smoke test.
type SmokeCheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// SmokeResult holds the full smoke test output.
type SmokeResult struct {
	AllPass bool         `json:"allPass"`
	Checks  []SmokeCheck `json:"checks"`
}

// PreMergeGate runs tsc and vite build in a worktree to validate the frontend
// compiles without errors. Returns nil on success, error with details on failure.
func PreMergeGate(worktreeWeb string) error {
	log.Printf("[gate] running pre-merge gate in %s", worktreeWeb)

	// Ensure node_modules symlink exists.
	devNodeModules := filepath.Join(worktreeWeb, "node_modules")
	mainNodeModules := filepath.Join(filepath.Dir(filepath.Dir(worktreeWeb)), "web", "node_modules")
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("npx not found in PATH")
	}

	// Create symlink if missing (same pattern as RebuildDevFrontend).
	cmd := exec.Command("ls", devNodeModules)
	if err := cmd.Run(); err != nil {
		symCmd := exec.Command("ln", "-sf", mainNodeModules, devNodeModules)
		if out, err := symCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("symlink node_modules: %s — %w", out, err)
		}
	}

	// Step 1: TypeScript check (catches type mismatches like the notifications bug).
	log.Printf("[gate] running tsc --noEmit")
	tsc := exec.Command("npx", "tsc", "--noEmit")
	tsc.Dir = worktreeWeb
	tscOut, tscErr := tsc.CombinedOutput()
	if tscErr != nil {
		return fmt.Errorf("tsc --noEmit failed:\n%s", string(tscOut))
	}
	log.Printf("[gate] tsc passed")

	// Step 2: Vite build.
	log.Printf("[gate] running vite build")
	vite := exec.Command("npx", "vite", "build")
	vite.Dir = worktreeWeb
	viteOut, viteErr := vite.CombinedOutput()
	if viteErr != nil {
		return fmt.Errorf("vite build failed:\n%s", string(viteOut))
	}
	log.Printf("[gate] vite build passed")

	return nil
}

// RunSmokeTest executes the Playwright smoke test against the given server URL
// via SSH to titan-pc. Returns structured results or error.
func RunSmokeTest(serverURL string) (*SmokeResult, error) {
	log.Printf("[gate] running smoke test against %s", serverURL)

	argsJSON, _ := json.Marshal(map[string]string{
		"action": "smoke",
		"url":    serverURL,
	})

	command := fmt.Sprintf(
		"echo %s | ssh titan-pc 'cat > /tmp/soul-e2e-args.json && cd ~/soul-e2e && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)),
	)

	cmd := exec.Command("bash", "-c", command)
	done := make(chan struct{})
	var out []byte
	var execErr error

	go func() {
		out, execErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return nil, fmt.Errorf("smoke test timed out after 60s")
	}

	if execErr != nil {
		return nil, fmt.Errorf("smoke test failed: %v\n%s", execErr, string(out))
	}

	// Parse JSON result from test-runner output.
	output := strings.TrimSpace(string(out))
	// Find the JSON line (skip any SSH banners).
	lines := strings.Split(output, "\n")
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "{") {
			jsonLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if jsonLine == "" {
		return nil, fmt.Errorf("no JSON output from smoke test: %s", output)
	}

	var result SmokeResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		return nil, fmt.Errorf("failed to parse smoke result: %v\nraw: %s", err, jsonLine)
	}

	log.Printf("[gate] smoke test result: allPass=%v, checks=%d", result.AllPass, len(result.Checks))
	return &result, nil
}

// RevertLastMerge reverts the most recent merge commit in the given directory.
// Used to undo a failed merge and restore the previous working state.
func RevertLastMerge(dir string) error {
	log.Printf("[gate] reverting last merge in %s", dir)
	cmd := exec.Command("git", "revert", "HEAD", "--no-edit")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git revert failed: %s — %w", string(out), err)
	}
	log.Printf("[gate] merge reverted successfully in %s", dir)
	return nil
}

// FormatSmokeFailure formats a SmokeResult into a human-readable gap report
// for feeding back to the agent.
func FormatSmokeFailure(result *SmokeResult) string {
	var b strings.Builder
	b.WriteString("Smoke test FAILED. The following checks did not pass:\n\n")
	for _, check := range result.Checks {
		if !check.Pass {
			fmt.Fprintf(&b, "- **%s**: %s\n", check.Name, check.Detail)
		}
	}
	b.WriteString("\nFix these issues, rebuild the frontend (`cd web && npx vite build`), and try again.")
	return b.String()
}
```

**Step 2: Build to verify**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Compiles successfully.

**Step 3: Commit**

```bash
git add internal/server/gates.go
git commit -m "feat: add merge gate functions — PreMergeGate, SmokeTest, RevertLastMerge"
```

---

### Task 4: Wire dev gate into autonomous.go

**Files:**
- Modify: `internal/server/autonomous.go:211-228` (after commit, around merge+rebuild block)

**Step 1: Add dev smoke test after RebuildDevFrontend**

Replace the block at lines 211-228 (the `if hasChanges { ... MergeToDev ... RebuildDevFrontend ... }` section) with a version that includes the smoke gate. The key change: after `RebuildDevFrontend` succeeds, run `RunSmokeTest` against the dev server. If it fails, revert the merge, rebuild to restore working state, and let the existing retry loop handle it.

In `autonomous.go`, replace lines 211-228:

```go
			if hasChanges {
				// Pre-merge gate: tsc + vite build in worktree.
				tp.sendActivity(taskID, "status", "Running pre-merge type check...")
				worktreeWeb := filepath.Join(taskRoot, "web")
				if gateErr := PreMergeGate(worktreeWeb); gateErr != nil {
					log.Printf("[autonomous] pre-merge gate failed for task %d: %v", taskID, gateErr)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Pre-merge gate failed: %v", gateErr))
					// Feed error to agent as gap report for self-repair.
					prompt = prompt + fmt.Sprintf("\n\n## Pre-Merge Gate Failed\n\n```\n%s\n```\n\nFix these type/build errors before the code can be merged.\n", gateErr.Error())
					hasChanges = false // prevent merge attempt
					continue           // retry loop
				}

				tp.sendActivity(taskID, "status", "Merging to dev branch...")
				if err := tp.worktrees.MergeToDev(taskID, task.Title); err != nil {
					log.Printf("[autonomous] merge to dev failed for task %d: %v", taskID, err)
					tp.sendActivity(taskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
				} else {
					tp.sendActivity(taskID, "status", "Changes merged to dev — rebuilding frontend...")
					if tp.server != nil {
						if err := tp.server.RebuildDevFrontend(); err != nil {
							log.Printf("[autonomous] dev frontend rebuild failed for task %d: %v", taskID, err)
							tp.sendActivity(taskID, "status", fmt.Sprintf("Dev rebuild warning: %v", err))
						} else {
							tp.sendActivity(taskID, "status", "Dev frontend rebuilt — running smoke test...")
							// Dev smoke test gate.
							devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)
							smokeResult, smokeErr := RunSmokeTest(devURL)
							if smokeErr != nil {
								log.Printf("[autonomous] dev smoke test error for task %d: %v", taskID, smokeErr)
								tp.sendActivity(taskID, "status", fmt.Sprintf("Smoke test error: %v", smokeErr))
							} else if !smokeResult.AllPass {
								log.Printf("[autonomous] dev smoke test FAILED for task %d", taskID)
								tp.sendActivity(taskID, "status", "Dev smoke test FAILED — reverting merge...")
								// Revert the merge to restore working dev build.
								devWT := filepath.Join(tp.projectRoot, ".worktrees", "dev-server")
								if revErr := RevertLastMerge(devWT); revErr != nil {
									log.Printf("[autonomous] revert failed: %v", revErr)
								}
								tp.server.RebuildDevFrontend() // restore working build
								// Feed smoke failures to agent for self-repair.
								prompt = prompt + "\n\n## Dev Smoke Test Failed\n\n" + FormatSmokeFailure(smokeResult) + "\n"
								hasChanges = false // prevent E2E verification from running on broken state
								continue           // retry loop
							} else {
								tp.sendActivity(taskID, "status", "Dev smoke test PASSED — all checks green")
							}
						}
					}
				}
			}
```

**Step 2: Add `path/filepath` import if not present**

Check the imports at the top of `autonomous.go`. If `path/filepath` is not imported, add it.

**Step 3: Build to verify**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Compiles successfully.

**Step 4: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "feat: wire dev smoke gate into autonomous pipeline with revert-on-failure"
```

---

### Task 5: Replace Rod verification with Playwright smoke test

**Files:**
- Modify: `internal/server/autonomous.go:230-304` (the existing E2E verification block)

**Step 1: Simplify E2E verification block**

The existing block (lines 230-304) uses Rod-based `verifyTask()` which doesn't work on ARM64. Since we now have the dev smoke gate (Task 4) catching rendering issues _before_ this point, we can simplify this section. Replace the Rod-based verification with the Playwright-based one:

Replace lines 230-304 with:

```go
		// Run E2E verification (only if there are changes to verify).
		verificationPassed = true
		verificationSkipped = false

		if tp.server != nil && hasChanges {
			// Smoke test already passed in the dev gate above.
			// Now run task-specific verification if AI is available.
			tp.sendActivity(taskID, "status", "Running task-specific verification...")
			devURL := fmt.Sprintf("http://localhost:%d", tp.server.cfg.Port+1)
			smokeResult, _ := RunSmokeTest(devURL)
			if smokeResult != nil && smokeResult.AllPass {
				verificationPassed = true
				tp.postVerificationComment(taskID, "**E2E Verification: PASSED**\n\nAll smoke checks passed.")
			} else if smokeResult != nil {
				verificationPassed = false
				tp.postVerificationComment(taskID, "**E2E Verification: FAILED**\n\n"+FormatSmokeFailure(smokeResult))
			} else {
				verificationSkipped = true
				tp.postVerificationComment(taskID, "**E2E Verification: SKIPPED**\n\nSmoke test unavailable (SSH to titan-pc failed).")
			}
			break // Already handled retries in the dev gate above
		} else if !hasChanges {
			tp.sendActivity(taskID, "status", "Skipping E2E verification — no changes to verify")
			break
		} else {
			break
		}
```

**Step 2: Build to verify**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Compiles. Note: this may require removing unused Rod imports. Check if `verification.go` is still needed — if the only callers were the old autonomous.go code, the Rod imports in verification.go will still compile fine (they're in a separate file that's still referenced by types).

**Step 3: Commit**

```bash
git add internal/server/autonomous.go
git commit -m "refactor: replace Rod E2E verification with Playwright smoke test"
```

---

### Task 6: Wire prod gate into tasks.go

**Files:**
- Modify: `internal/server/tasks.go:259-274` (the "Done" merge block)

**Step 1: Add prod smoke test after MergeToMaster + RebuildFrontend**

Replace lines 259-274 in tasks.go:

```go
	// Gate: merge to master when task moves to Done.
	if body.Stage == planner.StageDone && s.worktrees != nil {
		log.Printf("[tasks] task %d moved to done — merging to master", id)

		if err := s.worktrees.MergeToMaster(id, task.Title); err != nil {
			log.Printf("[tasks] merge to master failed for task %d: %v", id, err)
		} else {
			// Rebuild prod frontend.
			if err := s.worktrees.RebuildFrontend(s.projectRoot); err != nil {
				log.Printf("[tasks] prod frontend rebuild failed: %v", err)
			} else {
				// Prod smoke test gate.
				prodURL := fmt.Sprintf("http://localhost:%d", s.cfg.Port)
				smokeResult, smokeErr := RunSmokeTest(prodURL)
				if smokeErr != nil {
					log.Printf("[tasks] prod smoke test error for task %d: %v", id, smokeErr)
				} else if !smokeResult.AllPass {
					log.Printf("[tasks] prod smoke test FAILED for task %d — reverting", id)
					// Revert the merge to restore working prod.
					if revErr := RevertLastMerge(s.projectRoot); revErr != nil {
						log.Printf("[tasks] revert master failed: %v", revErr)
					}
					s.worktrees.RebuildFrontend(s.projectRoot) // restore working build
					// Move task back to validation.
					validation := planner.StageValidation
					s.planner.Update(id, planner.TaskUpdate{Stage: &validation})
					log.Printf("[tasks] task %d reverted to validation — prod smoke test failed", id)
					writeJSON(w, http.StatusConflict, map[string]string{
						"error": "Prod smoke test failed — merge reverted, task moved back to validation. Details: " + FormatSmokeFailure(smokeResult),
					})
					return
				} else {
					log.Printf("[tasks] prod smoke test PASSED for task %d", id)
				}
			}
		}

		// Cleanup the worktree.
		s.worktrees.Cleanup(id, task.Title)
	}
```

**Step 2: Build to verify**

Run: `cd /home/rishav/soul && go build ./...`
Expected: Compiles successfully.

**Step 3: Commit**

```bash
git add internal/server/tasks.go
git commit -m "feat: wire prod smoke gate into Done transition with revert-on-failure"
```

---

### Task 7: Build, deploy, and end-to-end verify

**Files:** None new — integration testing.

**Step 1: Full build**

```bash
cd /home/rishav/soul && go build -o soul ./cmd/soul
```

Expected: Binary compiles.

**Step 2: Rebuild frontend with data-testids**

```bash
cd /home/rishav/soul/web && npx vite build
```

**Step 3: Copy to dev server**

```bash
cp -r /home/rishav/soul/web/dist/* /home/rishav/soul/.worktrees/dev-server/web/dist/
```

**Step 4: Restart Soul**

```bash
pkill -f "./soul serve" && sleep 2
SOUL_HOST=0.0.0.0 ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY ./soul serve &
```

**Step 5: Test smoke action manually**

```bash
ssh titan-pc 'cd ~/soul-e2e && node test-runner.js smoke http://192.168.0.128:3000'
```

Expected: `{"allPass":true,"checks":[...8 checks all pass...]}`

**Step 6: Test dev server smoke**

```bash
ssh titan-pc 'cd ~/soul-e2e && node test-runner.js smoke http://192.168.0.128:3001'
```

Expected: Same — all pass.

**Step 7: Push to Gitea**

```bash
git push origin master
```

**Step 8: Final commit**

```bash
git commit --allow-empty -m "chore: verified E2E merge gates working on both servers"
```
