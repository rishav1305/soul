package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
)

// PMService provides project management intelligence — automatic task hygiene,
// duplicate detection, AI-powered enrichment, and periodic board sweeps.
type PMService struct {
	planner   *planner.Store
	broadcast func(WSMessage)
	ai        *ai.Client
	model     string
	ticker    *time.Ticker
	stopCh    chan struct{}
}

// NewPMService creates a new PMService.
func NewPMService(store *planner.Store, broadcast func(WSMessage), aiClient *ai.Client, model string) *PMService {
	return &PMService{
		planner:   store,
		broadcast: broadcast,
		ai:        aiClient,
		model:     model,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the periodic sweep ticker (every 30 minutes).
func (pm *PMService) Start() {
	pm.ticker = time.NewTicker(30 * time.Minute)
	go func() {
		for {
			select {
			case <-pm.ticker.C:
				pm.sweep()
			case <-pm.stopCh:
				return
			}
		}
	}()
	log.Printf("[pm] started — sweep every 30 minutes")
}

// Stop halts the periodic sweep.
func (pm *PMService) Stop() {
	if pm.ticker != nil {
		pm.ticker.Stop()
	}
	close(pm.stopCh)
	log.Printf("[pm] stopped")
}

// ---------------------------------------------------------------------------
// Levenshtein helpers
// ---------------------------------------------------------------------------

// levenshtein computes the edit distance between two strings using standard DP.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			min := ins
			if del < min {
				min = del
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// titleSimilarity returns a 0.0–1.0 similarity score between two strings.
func titleSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return 1.0
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshtein(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// postComment creates a PM comment on a task.
func (pm *PMService) postComment(taskID int64, body string) {
	c := planner.Comment{
		TaskID:      taskID,
		Author:      "pm",
		Type:        "feedback",
		Body:        body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := pm.planner.CreateComment(c); err != nil {
		log.Printf("[pm] failed to post comment on task %d: %v", taskID, err)
	}
}

// broadcastPM sends a pm.notification WSMessage to all connected clients.
func (pm *PMService) broadcastPM(severity, content string, taskIDs []int64, check string) {
	data, _ := json.Marshal(map[string]any{
		"severity": severity,
		"task_ids": taskIDs,
		"check":    check,
	})
	pm.broadcast(WSMessage{
		Type:    "pm.notification",
		Content: content,
		Data:    data,
	})
}

// ---------------------------------------------------------------------------
// AfterCreate — called when a new task is created
// ---------------------------------------------------------------------------

// AfterCreate is the public entry point called after a task is inserted.
func (pm *PMService) AfterCreate(task planner.Task) {
	go pm.afterCreateAsync(task)
}

func (pm *PMService) afterCreateAsync(task planner.Task) {
	// 1. Missing description
	if len(task.Description) < 20 {
		if task.Source == "ai" && pm.ai != nil {
			pm.enrichDescription(task)
		} else {
			pm.postComment(task.ID, "This task has a very short or missing description. Consider adding more detail so the agent (or a teammate) knows what to build.")
		}
	} else {
		// 2. Has description but no acceptance criteria
		descLower := strings.ToLower(task.Description)
		if !strings.Contains(task.Description, "- [ ]") && !strings.Contains(descLower, "acceptance") {
			pm.postComment(task.ID, "No acceptance criteria detected. Consider adding a checklist (`- [ ]` items) so completion can be verified.")
		}
	}

	// 3. Priority sanity: critical with no description
	if task.Priority >= 3 && len(task.Description) < 20 {
		pm.postComment(task.ID, "This task is marked critical (priority >= 3) but has no meaningful description. Please add details before it enters the active pipeline.")
		pm.broadcastPM("error",
			fmt.Sprintf("Critical task #%d (%s) created with no description", task.ID, task.Title),
			[]int64{task.ID}, "critical_no_desc")
	}

	// 4. Duplicate detection
	pm.checkDuplicate(task)

	// 5. Large task decomposition
	if pm.shouldDecompose(task) && pm.ai != nil {
		pm.decompose(task)
	}
}

// ---------------------------------------------------------------------------
// Duplicate detection
// ---------------------------------------------------------------------------

func (pm *PMService) checkDuplicate(task planner.Task) {
	openStages := []planner.Stage{planner.StageBacklog, planner.StageBrainstorm, planner.StageActive}
	for _, stage := range openStages {
		tasks, err := pm.planner.List(planner.TaskFilter{Stage: stage})
		if err != nil {
			log.Printf("[pm] failed to list %s tasks for duplicate check: %v", stage, err)
			continue
		}
		for _, existing := range tasks {
			if existing.ID == task.ID {
				continue
			}
			if titleSimilarity(task.Title, existing.Title) > 0.7 {
				pm.postComment(task.ID,
					fmt.Sprintf("Possible duplicate of task #%d (%s) in stage %s. Please review before starting work.",
						existing.ID, existing.Title, existing.Stage))
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Decomposition heuristics
// ---------------------------------------------------------------------------

func (pm *PMService) shouldDecompose(task planner.Task) bool {
	if task.ParentID != nil {
		return false
	}
	if len(task.Description) > 500 {
		return true
	}
	lower := strings.ToLower(task.Title)
	triggers := []string{"refactor", "redesign", "implement full", "add feature with", "new feature", "migration", "overhaul"}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// AI-powered enrichment
// ---------------------------------------------------------------------------

func (pm *PMService) enrichDescription(task planner.Task) {
	prompt := fmt.Sprintf(
		"Given this task title: %q\nProduct: %s\n\nGenerate a 2-3 line description with acceptance criteria as a markdown checklist (- [ ] items).\nReturn ONLY the description text, no preamble.",
		task.Title, task.Product)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := pm.ai.CompleteSimple(ctx, "claude-haiku-4-5-20251001", prompt)
	if err != nil {
		log.Printf("[pm] enrichDescription failed for task %d: %v", task.ID, err)
		pm.postComment(task.ID, "Could not auto-generate a description. Please add one manually so the agent knows what to build.")
		return
	}

	result = strings.TrimSpace(result)
	if result == "" {
		pm.postComment(task.ID, "Could not auto-generate a description. Please add one manually so the agent knows what to build.")
		return
	}

	if err := pm.planner.Update(task.ID, planner.TaskUpdate{Description: &result}); err != nil {
		log.Printf("[pm] failed to update description for task %d: %v", task.ID, err)
		return
	}

	pm.postComment(task.ID, "Auto-generated description from task title. Please review and refine as needed.")
}

// ---------------------------------------------------------------------------
// AI-powered decomposition
// ---------------------------------------------------------------------------

func (pm *PMService) decompose(task planner.Task) {
	prompt := fmt.Sprintf(
		`Break this task into 2-5 focused subtasks using INCREMENTAL DECOMPOSITION:

Rules:
- Each subtask adds exactly ONE new capability that is independently testable.
- Subtasks MUST be ordered sequentially — each builds on the previous one's confirmed working state.
- First subtask should wire settings/toggles to existing behavior (change nothing visible yet).
- Middle subtasks add behavior one piece at a time (e.g., one position, one panel, one interaction).
- Last subtask should be the final polish and integration.
- Each subtask must be completable in one autonomous agent session (1-3 files, under 30 minutes).
- For UI tasks: include "verify with e2e" in each subtask description.

Task: %s
Description: %s

Return a JSON array only, no markdown fences, no preamble:
[{"title": "...", "description": "...", "priority": 1}]`,
		task.Title, task.Description)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := pm.ai.CompleteSimple(ctx, "claude-haiku-4-5-20251001", prompt)
	if err != nil {
		log.Printf("[pm] decompose failed for task %d: %v", task.ID, err)
		return
	}

	result = strings.TrimSpace(result)

	// Strip markdown fences if present.
	if strings.HasPrefix(result, "```") {
		lines := strings.Split(result, "\n")
		// Remove first line (```json or ```) and last line (```)
		if len(lines) >= 2 {
			end := len(lines) - 1
			if strings.TrimSpace(lines[end]) == "```" {
				lines = lines[1:end]
			} else {
				lines = lines[1:]
			}
			result = strings.Join(lines, "\n")
		}
	}

	var subtasks []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
	}
	if err := json.Unmarshal([]byte(result), &subtasks); err != nil {
		log.Printf("[pm] decompose: failed to parse JSON for task %d: %v", task.ID, err)
		return
	}

	if len(subtasks) < 2 {
		return
	}

	var createdIDs []int64
	for _, st := range subtasks {
		sub := planner.NewTask(st.Title, st.Description)
		sub.Source = "ai"
		sub.Product = task.Product
		sub.ParentID = &task.ID
		sub.Priority = st.Priority
		id, err := pm.planner.Create(sub)
		if err != nil {
			log.Printf("[pm] decompose: failed to create subtask for task %d: %v", task.ID, err)
			continue
		}
		createdIDs = append(createdIDs, id)
	}

	if len(createdIDs) > 0 {
		idStrs := make([]string, len(createdIDs))
		for i, id := range createdIDs {
			idStrs[i] = fmt.Sprintf("#%d", id)
		}
		pm.postComment(task.ID,
			fmt.Sprintf("Decomposed into %d subtasks: %s", len(createdIDs), strings.Join(idStrs, ", ")))
	}
}

// ---------------------------------------------------------------------------
// Periodic sweep
// ---------------------------------------------------------------------------

func (pm *PMService) sweep() {
	log.Printf("[pm] running periodic sweep")

	now := time.Now().UTC()
	var findings []string

	// Helper to parse RFC3339 timestamps.
	parseTime := func(s string) (time.Time, bool) {
		if s == "" {
			return time.Time{}, false
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}

	// 1. Stale backlog (> 7 days)
	if tasks, err := pm.planner.List(planner.TaskFilter{Stage: planner.StageBacklog}); err == nil {
		for _, t := range tasks {
			if created, ok := parseTime(t.CreatedAt); ok {
				days := int(now.Sub(created).Hours() / 24)
				if days > 7 {
					pm.postComment(t.ID, fmt.Sprintf("Stale — %d+ days in backlog. Still relevant?", days))
					findings = append(findings, fmt.Sprintf("Task #%d stale in backlog (%d days)", t.ID, days))
				}
			}
		}
	}

	// 2. Active > 48 hours
	if tasks, err := pm.planner.List(planner.TaskFilter{Stage: planner.StageActive}); err == nil {
		for _, t := range tasks {
			ref := t.CreatedAt
			if t.StartedAt != "" {
				ref = t.StartedAt
			}
			if started, ok := parseTime(ref); ok {
				hours := now.Sub(started).Hours()
				if hours > 48 {
					pm.postComment(t.ID, fmt.Sprintf("Active for %.0f hours — check if it's stuck or needs to be broken down.", hours))
					pm.broadcastPM("warning",
						fmt.Sprintf("Task #%d (%s) active for %.0f hours", t.ID, t.Title, hours),
						[]int64{t.ID}, "active_too_long")
					findings = append(findings, fmt.Sprintf("Task #%d active for %.0f hours", t.ID, hours))
				}
			}
		}
	}

	// 3. Blocked > 5 days
	if tasks, err := pm.planner.List(planner.TaskFilter{Stage: planner.StageBlocked}); err == nil {
		for _, t := range tasks {
			if created, ok := parseTime(t.CreatedAt); ok {
				days := int(now.Sub(created).Hours() / 24)
				if days > 5 {
					pm.broadcastPM("warning",
						fmt.Sprintf("Task #%d (%s) blocked for %d days", t.ID, t.Title, days),
						[]int64{t.ID}, "blocked_too_long")
					findings = append(findings, fmt.Sprintf("Task #%d blocked for %d days", t.ID, days))
				}
			}
		}
	}

	// 4. Validation > 3 days
	if tasks, err := pm.planner.List(planner.TaskFilter{Stage: planner.StageValidation}); err == nil {
		for _, t := range tasks {
			if created, ok := parseTime(t.CreatedAt); ok {
				days := int(now.Sub(created).Hours() / 24)
				if days > 3 {
					pm.broadcastPM("info",
						fmt.Sprintf("Task #%d (%s) in validation for %d days", t.ID, t.Title, days),
						[]int64{t.ID}, "validation_stale")
					findings = append(findings, fmt.Sprintf("Task #%d in validation for %d days", t.ID, days))
				}
			}
		}
	}

	// 5. Priority decay: backlog tasks with priority=1, created > 14 days ago.
	if tasks, err := pm.planner.List(planner.TaskFilter{Stage: planner.StageBacklog}); err == nil {
		for _, t := range tasks {
			if t.Priority == 1 {
				if created, ok := parseTime(t.CreatedAt); ok {
					days := int(now.Sub(created).Hours() / 24)
					if days > 14 {
						zero := 0
						if err := pm.planner.Update(t.ID, planner.TaskUpdate{Priority: &zero}); err != nil {
							log.Printf("[pm] failed to decay priority for task %d: %v", t.ID, err)
							continue
						}
						pm.postComment(t.ID, "Priority decayed from 1 to 0 — task has been in backlog for 14+ days without action.")
						findings = append(findings, fmt.Sprintf("Task #%d priority decayed (1 → 0)", t.ID))
					}
				}
			}
		}
	}

	// Broadcast batched findings.
	if len(findings) > 0 {
		summary := "**[PM Board Review]**\n"
		for _, f := range findings {
			summary += "- " + f + "\n"
		}
		pm.broadcastPM("info", summary, nil, "sweep_summary")
	} else {
		log.Printf("[pm] sweep: board is clean")
	}
}
