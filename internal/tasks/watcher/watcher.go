package watcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

// Sender sends a request to the Claude API and returns the response.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// CommentWatcher polls for new user comments and responds.
type CommentWatcher struct {
	store       *store.Store
	sender      Sender
	projectRoot string
	lastID      int64
}

// New creates a new CommentWatcher.
func New(s *store.Store, sender Sender, projectRoot string) *CommentWatcher {
	return &CommentWatcher{store: s, sender: sender, projectRoot: projectRoot}
}

// Start begins polling for new comments every 5 seconds.
func (cw *CommentWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cw.poll(ctx)
		}
	}
}

// actionableStages are stages where agent action is possible.
var actionableStages = map[string]bool{
	"active":     true,
	"validation": true,
	"blocked":    true,
}

func (cw *CommentWatcher) poll(ctx context.Context) {
	comments, err := cw.store.CommentsAfter(cw.lastID)
	if err != nil {
		log.Printf("watcher: poll error: %v", err)
		return
	}

	for _, c := range comments {
		cw.lastID = c.ID

		task, err := cw.store.Get(c.TaskID)
		if err != nil {
			log.Printf("watcher: get task %d: %v", c.TaskID, err)
			continue
		}

		if !actionableStages[task.Stage] {
			reply := fmt.Sprintf("Task is in %s — comment noted but no action taken.", task.Stage)
			if _, err := cw.store.InsertComment(c.TaskID, "soul", "auto", reply); err != nil {
				log.Printf("watcher: insert reply: %v", err)
			}
			continue
		}

		cw.handleComment(ctx, c, *task)
	}
}

func (cw *CommentWatcher) handleComment(ctx context.Context, comment store.Comment, task store.Task) {
	if cw.sender == nil {
		reply := "Received feedback. Agent not configured."
		if _, err := cw.store.InsertComment(comment.TaskID, "soul", "auto", reply); err != nil {
			log.Printf("watcher: insert reply: %v", err)
		}
		return
	}

	// Load all comments for this task to build context.
	allComments, err := cw.store.GetComments(comment.TaskID)
	if err != nil {
		log.Printf("watcher: get comments for task %d: %v", comment.TaskID, err)
		reply := fmt.Sprintf("Error loading comment history: %v", err)
		if _, err := cw.store.InsertComment(comment.TaskID, "soul", "auto", reply); err != nil {
			log.Printf("watcher: insert error reply: %v", err)
		}
		return
	}

	// Build comment history string.
	var history strings.Builder
	for _, c := range allComments {
		fmt.Fprintf(&history, "[%s] %s: %s\n", c.CreatedAt, c.Author, c.Body)
	}

	// Build the feedback prompt.
	prompt := fmt.Sprintf(
		"Task: %s\nDescription: %s\nStage: %s\n\nComment history:\n%s\nLatest comment from %s:\n%s\n\nPlease respond to this feedback.",
		task.Title, task.Description, task.Stage, history.String(), comment.Author, comment.Body,
	)

	req := &stream.Request{
		System:    "You are a development agent responding to task feedback.",
		MaxTokens: 1024,
		Messages: []stream.Message{
			{
				Role: "user",
				Content: []stream.ContentBlock{
					{Type: "text", Text: prompt},
				},
			},
		},
	}

	agentCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, err := cw.sender.Send(agentCtx, req)
	if err != nil {
		log.Printf("watcher: agent call for task %d: %v", comment.TaskID, err)
		reply := fmt.Sprintf("Agent error: %v", err)
		if _, err := cw.store.InsertComment(comment.TaskID, "soul", "auto", reply); err != nil {
			log.Printf("watcher: insert error reply: %v", err)
		}
		return
	}

	// Extract text from response content blocks.
	var texts []string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}

	reply := strings.Join(texts, "\n")
	if reply == "" {
		reply = "Agent returned empty response."
	}

	if _, err := cw.store.InsertComment(comment.TaskID, "soul", "auto", reply); err != nil {
		log.Printf("watcher: insert agent reply: %v", err)
	}
}
