package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rishav1305/soul-v2/internal/tasks/store"
)

// CommentWatcher polls for new user comments and responds.
type CommentWatcher struct {
	store  *store.Store
	lastID int64
}

// New creates a new CommentWatcher.
func New(s *store.Store) *CommentWatcher {
	return &CommentWatcher{store: s}
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

		reply := "Received feedback. Agent processing not yet implemented."
		if _, err := cw.store.InsertComment(c.TaskID, "soul", "auto", reply); err != nil {
			log.Printf("watcher: insert reply: %v", err)
		}
	}
}
