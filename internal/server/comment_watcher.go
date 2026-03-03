package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/planner"
)

// CommentWatcher polls for new user comments and dispatches mini-agents.
type CommentWatcher struct {
	server      *Server
	planner     *planner.Store
	broadcast   func(WSMessage)
	projectRoot string
	worktrees   *WorktreeManager

	lastID int64
}

// NewCommentWatcher creates a comment watcher.
func NewCommentWatcher(srv *Server) *CommentWatcher {
	return &CommentWatcher{
		server:      srv,
		planner:     srv.planner,
		broadcast:   srv.broadcast,
		projectRoot: srv.projectRoot,
		worktrees:   srv.worktrees,
	}
}

// Start begins polling in a background goroutine.
func (cw *CommentWatcher) Start(ctx context.Context) {
	// Seed lastID to current max so we don't reprocess old comments.
	maxID, err := cw.planner.MaxCommentID()
	if err != nil {
		log.Printf("[comment-watcher] failed to get max comment ID: %v", err)
	}
	cw.lastID = maxID

	go cw.poll(ctx)
	log.Printf("[comment-watcher] started (last_id=%d)", cw.lastID)
}

func (cw *CommentWatcher) poll(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[comment-watcher] stopped")
			return
		case <-ticker.C:
			cw.checkNewComments(ctx)
		}
	}
}

func (cw *CommentWatcher) checkNewComments(ctx context.Context) {
	comments, err := cw.planner.CommentsAfter(cw.lastID)
	if err != nil {
		log.Printf("[comment-watcher] poll error: %v", err)
		return
	}

	for _, comment := range comments {
		cw.lastID = comment.ID
		log.Printf("[comment-watcher] new user comment #%d on task %d: %.80s",
			comment.ID, comment.TaskID, comment.Body)

		// Check task is in actionable state.
		task, err := cw.planner.Get(comment.TaskID)
		if err != nil {
			log.Printf("[comment-watcher] failed to get task %d: %v", comment.TaskID, err)
			continue
		}

		if task.Stage != planner.StageActive &&
			task.Stage != planner.StageValidation &&
			task.Stage != planner.StageBlocked {
			cw.postReply(comment.TaskID, "status",
				fmt.Sprintf("Task is in %s stage — comment noted but no action taken.", task.Stage))
			continue
		}

		go cw.handleComment(ctx, task, comment)
	}
}

func (cw *CommentWatcher) handleComment(ctx context.Context, task planner.Task, comment planner.Comment) {
	cw.postReply(comment.TaskID, "status", "Investigating your feedback...")

	// Determine worktree path.
	taskRoot := cw.projectRoot
	if cw.worktrees != nil {
		wt := cw.worktrees.worktreePath(comment.TaskID)
		if info, err := os.Stat(wt); err == nil && info.IsDir() {
			taskRoot = wt
		}
	}

	// Gather all comments for context.
	allComments, _ := cw.planner.ListComments(comment.TaskID)
	var commentLog strings.Builder
	for _, c := range allComments {
		fmt.Fprintf(&commentLog, "[%s] %s (%s): %s\n", c.CreatedAt, c.Author, c.Type, c.Body)
	}

	// Build prompt for the mini-agent.
	prompt := cw.buildFeedbackPrompt(task, comment, commentLog.String(), taskRoot)

	sessionID := fmt.Sprintf("comment-%d-%d", comment.TaskID, comment.ID)
	var outputBuf strings.Builder

	sendEvent := func(msg WSMessage) {
		if msg.Type == "chat.token" {
			outputBuf.WriteString(msg.Content)
		}
	}

	agent := NewAgentLoop(cw.server.ai, cw.server.products, cw.server.sessions, cw.planner, cw.broadcast, cw.server.cfg.Model, taskRoot)
	agent.Run(ctx, sessionID, prompt, "code", nil, false, sendEvent)

	// Post the agent's response as a soul comment.
	response := outputBuf.String()
	if response == "" {
		response = "Investigation complete — no specific output generated."
	}
	cw.postReply(comment.TaskID, "status", response)

	// Commit changes in the worktree and merge to dev (same as autonomous pipeline).
	if cw.worktrees != nil && taskRoot != cw.projectRoot {
		cw.postReply(comment.TaskID, "status", "Committing changes...")
		if err := cw.worktrees.CommitInWorktree(comment.TaskID, task.Title); err != nil {
			log.Printf("[comment-watcher] commit failed for task %d: %v", comment.TaskID, err)
			cw.postReply(comment.TaskID, "status", fmt.Sprintf("Commit warning: %v", err))
		} else {
			cw.postReply(comment.TaskID, "status", "Merging to dev branch...")
			if err := cw.worktrees.MergeToDev(comment.TaskID, task.Title); err != nil {
				log.Printf("[comment-watcher] merge to dev failed for task %d: %v", comment.TaskID, err)
				cw.postReply(comment.TaskID, "status", fmt.Sprintf("Merge to dev warning: %v", err))
			} else {
				log.Printf("[comment-watcher] merged feedback changes for task %d to dev", comment.TaskID)
			}
		}
	}

	// Rebuild dev frontend after merge.
	if cw.server != nil {
		if err := cw.server.RebuildDevFrontend(); err != nil {
			log.Printf("[comment-watcher] dev rebuild failed: %v", err)
		} else {
			cw.postReply(comment.TaskID, "status", "Changes committed, merged to dev, and frontend rebuilt — check the dev server.")
		}
	}
}

func (cw *CommentWatcher) buildFeedbackPrompt(task planner.Task, comment planner.Comment, commentLog, taskRoot string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are responding to user feedback on task #%d.\n\n", task.ID)
	fmt.Fprintf(&b, "**Task Title:** %s\n", task.Title)
	fmt.Fprintf(&b, "**Task Description:** %s\n", task.Description)
	fmt.Fprintf(&b, "**Current Stage:** %s\n\n", task.Stage)
	fmt.Fprintf(&b, "## Comment History\n```\n%s```\n\n", commentLog)
	fmt.Fprintf(&b, "## User's Current Feedback\n%s\n\n", comment.Body)
	b.WriteString("## Instructions\n")
	b.WriteString("- Diagnose the issue described in the user's feedback\n")
	b.WriteString("- If it's a code issue within this task's scope, fix it\n")
	b.WriteString("- If the dev server needs a restart or rebuild, the system will handle that\n")
	b.WriteString("- Write a clear summary of what you found and what you did\n")
	b.WriteString("- Do NOT run git commands — the system handles commits and merges\n")
	fmt.Fprintf(&b, "\nProject root: `%s`\n", taskRoot)
	return b.String()
}

func (cw *CommentWatcher) postReply(taskID int64, commentType, body string) {
	comment := planner.Comment{
		TaskID:      taskID,
		Author:      "soul",
		Type:        commentType,
		Body:        body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	id, err := cw.planner.CreateComment(comment)
	if err != nil {
		log.Printf("[comment-watcher] failed to post reply on task %d: %v", taskID, err)
		return
	}
	comment.ID = id
	raw, _ := json.Marshal(comment)
	cw.broadcast(WSMessage{Type: "task.comment.added", Data: raw})
}
