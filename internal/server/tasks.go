package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/rishav1305/soul/internal/planner"
)

// handleTaskCreate handles POST /api/tasks — creates a new planner task.
func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Product     string `json:"product"`
		Acceptance  string `json:"acceptance"`
		Source      string `json:"source"`
		ParentID    *int64 `json:"parent_id,omitempty"`
		Metadata    string `json:"metadata"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	task := planner.NewTask(body.Title, body.Description)
	task.Priority = body.Priority
	task.Product = body.Product
	task.Acceptance = body.Acceptance
	task.Metadata = body.Metadata
	task.ParentID = body.ParentID
	if body.Source != "" {
		task.Source = body.Source
	}

	id, err := s.planner.Create(task)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create task: %v", err)})
		return
	}

	// Fetch the created task to return the full object.
	created, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to fetch created task: %v", err)})
		return
	}

	s.broadcastTaskEvent("task.created", created)
	writeJSON(w, http.StatusCreated, created)
}

// handleTaskList handles GET /api/tasks — lists tasks with optional filters.
func (s *Server) handleTaskList(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	filter := planner.TaskFilter{
		Stage:   planner.Stage(r.URL.Query().Get("stage")),
		Product: r.URL.Query().Get("product"),
	}

	tasks, err := s.planner.List(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list tasks: %v", err)})
		return
	}

	// Return empty array instead of null.
	if tasks == nil {
		tasks = []planner.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// handleTaskGet handles GET /api/tasks/{id} — returns a single task.
func (s *Server) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	task, err := s.planner.Get(id)
	if errors.Is(err, planner.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get task: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleTaskUpdate handles PATCH /api/tasks/{id} — partial update of a task.
func (s *Server) handleTaskUpdate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var update planner.TaskUpdate
	if err := readJSON(r, &update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.planner.Update(id, update); err != nil {
		if errors.Is(err, planner.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to update task: %v", err)})
		return
	}

	// Fetch updated task to return and broadcast.
	updated, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to fetch updated task: %v", err)})
		return
	}

	s.broadcastTaskEvent("task.updated", updated)

	// Check if autonomous was just toggled on — start processing.
	if update.Metadata != nil && s.processor != nil {
		var meta map[string]any
		if err := json.Unmarshal([]byte(*update.Metadata), &meta); err == nil {
			if auto, ok := meta["autonomous"].(bool); ok && auto {
				if !s.processor.IsRunning(id) {
					log.Printf("[tasks] autonomous toggled on for task %d — starting processor", id)
					s.processor.StartTask(id)
				}
			} else {
				// Autonomous toggled off — stop if running.
				if s.processor.IsRunning(id) {
					log.Printf("[tasks] autonomous toggled off for task %d — stopping processor", id)
					s.processor.StopTask(id)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, updated)
}

// handleTaskDelete handles DELETE /api/tasks/{id} — deletes a task.
func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.planner.Delete(id); err != nil {
		if errors.Is(err, planner.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to delete task: %v", err)})
		return
	}

	s.broadcastTaskEvent("task.deleted", map[string]int64{"id": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleTaskMove handles POST /api/tasks/{id}/move — moves a task to a new stage.
func (s *Server) handleTaskMove(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	id, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		Stage   planner.Stage `json:"stage"`
		Comment string        `json:"comment"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if !body.Stage.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid stage: %q", body.Stage)})
		return
	}

	// Fetch current task to validate the transition.
	task, err := s.planner.Get(id)
	if errors.Is(err, planner.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get task: %v", err)})
		return
	}

	if !planner.ValidTransition(task.Stage, body.Stage) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid transition from %q to %q", task.Stage, body.Stage),
		})
		return
	}

	// Apply the stage update.
	update := planner.TaskUpdate{Stage: &body.Stage}
	if err := s.planner.Update(id, update); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to move task: %v", err)})
		return
	}

	// Gate: merge to master when task moves to Done.
	if body.Stage == planner.StageDone && s.worktrees != nil {
		log.Printf("[tasks] task %d moved to done — merging to master", id)

		if err := s.worktrees.MergeToMaster(id, task.Title); err != nil {
			log.Printf("[tasks] merge to master failed for task %d: %v", id, err)
			// Don't fail the move — log the error. User can retry.
		} else {
			// Rebuild prod frontend.
			if err := s.worktrees.RebuildFrontend(s.projectRoot); err != nil {
				log.Printf("[tasks] prod frontend rebuild failed: %v", err)
			}
		}

		// Cleanup the worktree.
		s.worktrees.Cleanup(id, task.Title)
	}

	// Fetch the updated task.
	moved, err := s.planner.Get(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to fetch moved task: %v", err)})
		return
	}

	s.broadcastTaskEvent("task.updated", moved)
	writeJSON(w, http.StatusOK, moved)
}

// parseTaskID extracts the {id} path parameter and parses it as int64.
func parseTaskID(r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	if idStr == "" {
		return 0, fmt.Errorf("missing task id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid task id: %q", idStr)
	}
	return id, nil
}

// readJSON reads and decodes JSON from the request body.
func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("empty request body")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// handleCommentCreate handles POST /api/tasks/{id}/comments — adds a comment to a task.
func (s *Server) handleCommentCreate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Verify task exists.
	if _, err := s.planner.Get(taskID); errors.Is(err, planner.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get task: %v", err)})
		return
	}

	var body struct {
		Author string `json:"author"`
		Type   string `json:"type"`
		Body   string `json:"body"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if body.Author == "" {
		body.Author = "user"
	}
	if body.Type == "" {
		body.Type = "feedback"
	}
	if body.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body is required"})
		return
	}

	comment := planner.Comment{
		TaskID:      taskID,
		Author:      body.Author,
		Type:        body.Type,
		Body:        body.Body,
		Attachments: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	id, err := s.planner.CreateComment(comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create comment: %v", err)})
		return
	}
	comment.ID = id

	s.broadcastTaskEvent("task.comment.added", comment)
	writeJSON(w, http.StatusCreated, comment)
}

// handleCommentList handles GET /api/tasks/{id}/comments — lists comments for a task.
func (s *Server) handleCommentList(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "planner not configured"})
		return
	}

	taskID, err := parseTaskID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	comments, err := s.planner.ListComments(taskID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list comments: %v", err)})
		return
	}

	if comments == nil {
		comments = []planner.Comment{}
	}
	writeJSON(w, http.StatusOK, comments)
}
