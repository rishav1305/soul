# Soul Autonomous Pipeline v2 — "Step-Verify-Fix"

## Problem Statement

Soul's autonomous task execution fails frequently despite having TDD, unit tests, and E2E quality gates. Root causes identified:

1. **Wrong model**: Tasks use Sonnet without thinking — insufficient reasoning for complex multi-file changes
2. **No step-level verification**: Agent plans well but executes all steps blind, only checks at the end
3. **Weak final gate**: Only runs `tsc + vite build` — misses runtime errors (e.g., `ReferenceError: selectedTask is not defined`)
4. **E2E tools stripped from autonomous**: Agent told "pipeline handles verification" but pipeline barely verifies
5. **Chat and autonomous pipelines diverge**: Different models, different skills, different verification — brainstorm→task handoff loses quality
6. **Chat creates subtasks unprompted**: Agent auto-creates tasks during brainstorm without user approval
7. **Inconsistent task tracking in chat**: Agent starts managing tasks but abandons them mid-conversation

## Design Decisions (from brainstorming)

- **Model strategy**: Phase-based routing (not one model for everything)
- **Verification**: Layered approach — build + runtime + feature test + visual regression
- **Failure handling**: Opus as "senior engineer" fixes verification failures; Sonnet never retries its own mistakes
- **Step verification**: Build + runtime + Opus diff review after every step
- **Architecture**: One execution engine, two triggers (chat = human in loop, autonomous = pipeline acts as reviewer)
- **Chat behavior**: No auto-subtask creation, consistent task tracking, inline E2E screenshots

---

## Architecture: One Engine, Two Triggers

```
┌─────────────────────────────────────────────────────────────┐
│                    EXECUTION ENGINE                          │
│                                                              │
│  Same pipeline for both chat and autonomous:                 │
│  Plan → Execute → Verify → Fix (if needed) → Final Gate     │
│                                                              │
│  ┌──────────────┐         ┌──────────────────┐              │
│  │ Chat Trigger  │         │ Autonomous Trigger│              │
│  │ Human reviews │         │ Pipeline reviews  │              │
│  │ between steps │         │ between steps     │              │
│  └──────────────┘         └──────────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

Difference is only WHO acts at review checkpoints:
- Chat: Human can intervene, provide feedback, redirect
- Autonomous: Pipeline runs verification gates automatically

---

## Phase-Based Model Router

| Phase | Model | Rationale |
|-------|-------|-----------|
| **Planning** | Opus + thinking | Deep reasoning about architecture, dependencies, side effects. Also writes verification spec. |
| **Implementation** | Sonnet + thinking | Fast code generation, thinking catches edge cases |
| **Step Review** | Opus + thinking | Reviews diff for logic/reference errors (catches bugs like `selectedTask` removal) |
| **Fixing** (on failure) | Opus + thinking | Diagnoses and fixes verification failures. Senior engineer role. |
| **Micro tasks** | Haiku | Simple substitutions (rename, color change, typo fix) — no step decomposition |

**Cost control**: Opus only used for planning (once), brief diff review (per step), and fixing (only on failure). Sonnet does bulk code generation. Haiku handles trivial tasks entirely.

---

## Task Lifecycle: Step-Verify-Fix

```
TASK START
  │
  ▼
┌─────────────────────────────────────────────┐
│ PHASE 1: PLAN (Opus + thinking)             │
│                                              │
│ • Read codebase, identify affected files     │
│ • Design approach with numbered steps        │
│ • Write verification spec (what to test)     │
│ • Output: step list + verification spec      │
│ • Max: 1 planning iteration                  │
└─────────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────────┐
│ PHASE 2: EXECUTE + VERIFY (per step)        │
│                                              │
│ For each step in plan:                       │
│                                              │
│   2a. IMPLEMENT (Sonnet + thinking)          │
│       • Execute one logical step             │
│       • Agent declares "Step N complete"     │
│                                              │
│   2b. VERIFY (automated + Opus)              │
│       • Layer A: tsc --noEmit + vite build   │
│       • Runtime: Load page, check JS errors  │
│       • Opus diff review: check for logic    │
│         errors, missing references, etc.     │
│                                              │
│   2c. FIX (if verification fails)            │
│       • Opus + thinking diagnoses & fixes    │
│       • Fix goes through same verification   │
│       • 2 Opus failures on same step →       │
│         task moves to BLOCKED                │
│                                              │
│   ✓ Step passes → proceed to next step       │
└─────────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────────┐
│ PHASE 3: FINAL GATE (3 layers)              │
│                                              │
│ Layer A: Build verification (ALWAYS)         │
│   • tsc --noEmit                             │
│   • vite build                               │
│   • Load page in headless Chrome             │
│   • Check for JS console errors (zero tol.)  │
│                                              │
│ Layer B: Feature verification (quick/full)   │
│   • Agent-written feature test script        │
│   • Based on verification spec from Phase 1  │
│   • Runs assertions against actual page      │
│   • e.g., "click tab → assert active class"  │
│                                              │
│ Layer C: Visual regression (UI tasks only)   │
│   • Screenshot BEFORE merge (baseline)       │
│   • Screenshot AFTER merge                   │
│   • Compare key pages: chat, planner, panel  │
│   • Flag layout/visual breakage              │
│                                              │
│ ALL layers must pass before merge to dev.    │
│ Same layers run again after merge to master. │
└─────────────────────────────────────────────┘
  │
  ▼
┌─────────────────────────────────────────────┐
│ PHASE 4: MERGE + DEPLOY                     │
│                                              │
│ • git merge --no-ff to dev                   │
│ • Rebuild dev frontend                       │
│ • Run Final Gate on dev server (:3001)       │
│ • Pass → stage=validation (human review)     │
│ • Fail → revert merge, retry (up to 3x)     │
│                                              │
│ Human moves to Done:                         │
│ • Merge to master                            │
│ • Rebuild prod frontend                      │
│ • Run Final Gate on prod (:3000)             │
│ • Pass → cleanup worktree + branch           │
│ • Fail → revert, back to validation          │
└─────────────────────────────────────────────┘
```

---

## Chat Behavior Rules

### 1. No Auto-Subtask Creation
- **Remove** system prompt line: "For large tasks (3+ files), create a parent task + subtasks"
- **Remove** system prompt line: "Act directly on safe actions (create subtasks)"
- **Replace with**: "NEVER create tasks or subtasks unless the user explicitly asks. Propose decomposition, wait for approval."
- **Brainstorm mode**: Existing "NEVER create tasks" rule stays, strengthen to absolute prohibition

### 2. Consistent Task Tracking
- If the agent starts tracking tasks in a conversation, it MUST continue until conversation ends
- Before any context compression, agent must summarize current task status
- Agent should update task stage/status in real-time as work progresses
- If agent uses tools to work on a task, it must report completion/failure

### 3. Inline E2E Screenshots
- When `e2e_screenshot` tool is used in chat, the saved image path should be returned as a rendered image in the chat message
- Frontend `Message.tsx` should detect image paths in tool output and render them inline
- This gives visual proof of work directly in the conversation

### 4. Unified Execution Engine
- Chat and autonomous use the same step-verify-fix pipeline
- In chat: human acts as reviewer at checkpoints (can approve, redirect, or intervene)
- In autonomous: pipeline runs verification automatically at checkpoints
- Same model routing rules apply to both

---

## Verification Spec (Written During Planning Phase)

During Phase 1, Opus writes a verification spec alongside the plan. Example:

```yaml
task: "Replace chat type dropdown with segmented slider"
steps:
  - name: "Add SegmentedControl component"
    verify:
      - build: true
      - runtime_errors: 0
      - dom_check: "[data-testid='chat-type-control']"
  - name: "Wire to chat state"
    verify:
      - build: true
      - runtime_errors: 0
      - assert: "click Brainstorm tab → chatType state === 'brainstorm'"
  - name: "Remove old dropdown"
    verify:
      - build: true
      - runtime_errors: 0
      - assert: "select.chat-type-dropdown does NOT exist"
      - visual: "chat input area matches expected layout"
final_gate:
  - all_build_checks: true
  - runtime_errors: 0
  - feature_tests:
    - "All 4 chat types selectable and functional"
    - "Active tab visually highlighted"
    - "Switching types changes agent behavior"
  - visual_regression:
    - pages: ["/", "/planner"]
    - threshold: 0.95
```

---

## Failure Modes Addressed

| Previous Failure | How v2 Fixes It |
|-----------------|-----------------|
| `selectedTask is not defined` — agent removed declaration, left references | Opus diff review after each step catches "removed X but Y still references it" |
| Runtime JS errors not caught | Layer A loads page in headless Chrome, checks console for errors |
| Agent uses all context, then retries with nothing left | Step-level verification catches errors early, before context is exhausted |
| Context emergency at 85% clears everything | With step-level verification, agent finishes faster and cleaner — less likely to hit 85% |
| Smoke test only checks "page loads" | Layer B runs feature-specific assertions; Layer C does visual regression |
| Skills stripped from autonomous | One engine means autonomous gets same capabilities as chat |
| Chat creates subtasks during brainstorm | System prompt explicitly prohibits auto-creation |
| Agent forgets to track tasks in chat | System prompt mandates consistent tracking once started |
| No visual proof in chat | E2E screenshots render inline |

---

## Infrastructure Requirements

- **Puppeteer on titan-pc**: Already exists (`~/soul-e2e/test-runner.js`), needs extensions for:
  - Console error capture (return JS errors as structured data)
  - Visual regression (screenshot comparison with threshold)
  - Feature test runner (execute assertion scripts)
- **Model switching**: `ai.Client` needs method to create requests with different models mid-conversation
- **Step boundary detection**: Agent output parser to detect "Step N complete" markers
- **Verification spec format**: YAML or JSON schema for structured verification specs

---

## Implementation Plan (Sequential with Quality Gates)

### Step 1: Model Router Infrastructure
**What**: Add model selection to `ai.Client` — ability to switch models per-request without creating new clients.
**Files**: `internal/ai/client.go`, `internal/ai/oauth.go`
**Gate**: Unit test — create requests with different models, verify correct model in API call

### Step 2: Phase-Based Agent Loop
**What**: Refactor `AgentLoop` to support phases (plan/execute/review/fix) with different models per phase. Add step boundary detection.
**Files**: `internal/server/agent.go` (major refactor)
**Gate**: Agent can switch from Opus (planning) to Sonnet (execution) mid-task. Log shows model switches.

### Step 3: Step-Level Verification Gate
**What**: After each step boundary, run: `tsc --noEmit` + `vite build` + headless Chrome page load + JS console error check.
**Files**: `internal/server/gates.go` (new file), `internal/server/autonomous.go`
**Gate**: Create a task that introduces a `ReferenceError`. Verify the step-level gate catches it before final gate.

### Step 4: Opus Diff Review
**What**: After each step passes build/runtime, send the git diff to Opus with prompt: "Review this diff for logic errors, missing references, removed declarations with remaining usage, etc."
**Files**: `internal/server/agent.go` (add review phase), `internal/server/autonomous.go`
**Gate**: Create a task that removes a variable declaration but leaves references. Verify Opus review catches it.

### Step 5: Opus Fix on Failure
**What**: When step verification fails, hand to Opus+thinking with the error. Opus fixes. Fix goes through same verification. 2 Opus failures on same step → blocked.
**Files**: `internal/server/agent.go`, `internal/server/autonomous.go`
**Gate**: Create a task with a deliberate bug. Verify Sonnet produces it, verification catches it, Opus fixes it, verification passes.

### Step 6: Verification Spec in Planning Phase
**What**: During Opus planning phase, agent outputs a verification spec (YAML) alongside the step plan. This spec drives Layer B feature tests.
**Files**: `internal/server/agent.go` (planning phase), `internal/server/autonomous.go` (spec parser)
**Gate**: Run a task, verify verification spec is generated and parseable.

### Step 7: Layered Final Gate
**What**: Extend final gate from just `tsc+vite` to full 3-layer verification:
- Layer A: build + runtime (always)
- Layer B: feature test from verification spec (quick/full)
- Layer C: visual regression screenshots (UI tasks)
**Files**: `internal/server/gates.go`, Puppeteer `test-runner.js` on titan-pc
**Gate**: Run a task end-to-end. All 3 layers execute. Intentionally break one — verify it's caught.

### Step 8: Chat Behavior Rules
**What**: Update system prompt — remove auto-subtask creation, add consistent task tracking mandate, prohibit task creation in brainstorm mode.
**Files**: `internal/server/agent.go` (system prompt constants)
**Gate**: Start a brainstorm chat, verify agent does NOT create tasks. Start a task-tracking chat, verify agent maintains status updates throughout.

### Step 9: Inline E2E Screenshots in Chat
**What**: When `e2e_screenshot` returns an image path, frontend renders it inline in the chat message.
**Files**: `web/src/components/chat/ToolCall.tsx` or `Message.tsx`
**Gate**: Run e2e_screenshot in chat, verify image renders inline.

### Step 10: Unified Engine (Chat + Autonomous)
**What**: Make chat use the same step-verify-fix pipeline as autonomous. In chat, human is the reviewer. In autonomous, pipeline reviews automatically.
**Files**: `internal/server/ws.go`, `internal/server/agent.go`, `internal/server/autonomous.go`
**Gate**: Run same task in chat and autonomous mode. Both produce same quality output with same verification gates.

### Step 11: Integration Test — Full End-to-End
**What**: Create a real task ("Add a new button to the chat header"). Run it through the full v2 pipeline. Verify:
- Opus plans with verification spec
- Sonnet implements step by step
- Each step passes verification
- Final gate passes all 3 layers
- Merges to dev cleanly
- Dev server shows the button
- No JS errors, feature works, visual regression passes
**Gate**: Task completes successfully with zero manual intervention.
