package context

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ToolRoute maps a tool name to an HTTP endpoint on a product server.
type ToolRoute struct {
	Method  string // GET, POST, PATCH, DELETE
	Path    string // e.g., "/api/tasks/{task_id}"
	Product string // tasks, tutor, projects, observe
}

// Dispatcher routes tool calls to product server REST APIs.
type Dispatcher struct {
	client *http.Client
	routes map[string]ToolRoute
	urls   map[string]string // product → base URL
}

// NewDispatcher creates a dispatcher with a shared HTTP client.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		urls: map[string]string{
			"tasks":    envOr("SOUL_TASKS_URL", "http://127.0.0.1:3004"),
			"tutor":    envOr("SOUL_TUTOR_URL", "http://127.0.0.1:3006"),
			"projects": envOr("SOUL_PROJECTS_URL", "http://127.0.0.1:3008"),
			"observe":  envOr("SOUL_OBSERVE_URL", "http://127.0.0.1:3010"),
		},
		routes: map[string]ToolRoute{
			// Tasks
			"list_tasks":  {Method: "GET", Path: "/api/tasks", Product: "tasks"},
			"create_task": {Method: "POST", Path: "/api/tasks", Product: "tasks"},
			"get_task":    {Method: "GET", Path: "/api/tasks/{task_id}", Product: "tasks"},
			"update_task": {Method: "PATCH", Path: "/api/tasks/{task_id}", Product: "tasks"},
			"start_task":  {Method: "POST", Path: "/api/tasks/{task_id}/start", Product: "tasks"},
			"stop_task":   {Method: "POST", Path: "/api/tasks/{task_id}/stop", Product: "tasks"},
			// Tutor
			"tutor_dashboard": {Method: "GET", Path: "/api/tutor/dashboard", Product: "tutor"},
			"list_topics":     {Method: "GET", Path: "/api/tutor/topics", Product: "tutor"},
			"start_drill":     {Method: "POST", Path: "/api/tutor/drill/start", Product: "tutor"},
			"answer_drill":    {Method: "POST", Path: "/api/tutor/drill/answer", Product: "tutor"},
			"due_reviews":     {Method: "GET", Path: "/api/tutor/drill/due", Product: "tutor"},
			"create_mock":     {Method: "POST", Path: "/api/tutor/mocks", Product: "tutor"},
			"list_mocks":      {Method: "GET", Path: "/api/tutor/mocks", Product: "tutor"},
			// Projects — note: server uses {id} for project and {mid} for milestone
			"projects_dashboard": {Method: "GET", Path: "/api/projects/dashboard", Product: "projects"},
			"get_project":        {Method: "GET", Path: "/api/projects/{project_id}", Product: "projects"},
			"update_project":     {Method: "PATCH", Path: "/api/projects/{project_id}", Product: "projects"},
			"update_milestone":   {Method: "PATCH", Path: "/api/projects/{project_id}/milestones/{milestone_id}", Product: "projects"},
			"record_metric":      {Method: "POST", Path: "/api/projects/{project_id}/metrics", Product: "projects"},
			"get_guide":          {Method: "GET", Path: "/api/projects/{project_id}/guide", Product: "projects"},
			// Observe — dispatcher calls observe server directly on :3010, routes use /api/* directly
			"observe_overview": {Method: "GET", Path: "/api/overview", Product: "observe"},
			"observe_pillars":  {Method: "GET", Path: "/api/pillars", Product: "observe"},
			"observe_tail":     {Method: "GET", Path: "/api/tail", Product: "observe"},
			"observe_alerts":   {Method: "GET", Path: "/api/alerts", Product: "observe"},
		},
	}
	return d
}

const maxToolResultBytes = 50 * 1024 // 50KB

// Execute dispatches a tool call to the appropriate product server.
// Returns the response body as a string for use as a tool_result.
// HTTP and network errors are returned as error strings (not Go errors)
// so they can be fed back to Claude as tool results.
func (d *Dispatcher) Execute(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
	route, ok := d.routes[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	baseURL, ok := d.urls[route.Product]
	if !ok {
		return "", fmt.Errorf("no URL configured for product: %s", route.Product)
	}

	// Parse input parameters.
	var params map[string]interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
	}
	if params == nil {
		params = make(map[string]interface{})
	}

	// Substitute path parameters and collect remaining params for query/body.
	path := route.Path
	remaining := make(map[string]interface{})
	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", v))
		} else {
			remaining[k] = v
		}
	}

	fullURL := baseURL + path

	var body io.Reader
	if route.Method == "GET" {
		// Remaining params become query string.
		if len(remaining) > 0 {
			q := url.Values{}
			for k, v := range remaining {
				q.Set(k, fmt.Sprintf("%v", v))
			}
			fullURL += "?" + q.Encode()
		}
	} else {
		// POST/PATCH — remaining params become JSON body.
		if len(remaining) > 0 {
			b, err := json.Marshal(remaining)
			if err != nil {
				return "", fmt.Errorf("marshal body: %w", err)
			}
			body = bytes.NewReader(b)
		}
	}

	// Create request with per-request timeout.
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, route.Method, fullURL, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		// Network errors returned as strings for tool results, not Go errors.
		return fmt.Sprintf("Error calling %s: %v", toolName, err), nil
	}
	defer resp.Body.Close()

	// Read response, capped at maxToolResultBytes+1 to detect truncation.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxToolResultBytes+1))
	if err != nil {
		return fmt.Sprintf("Error reading response from %s: %v", toolName, err), nil
	}

	result := string(data)
	if len(data) > maxToolResultBytes {
		result = result[:maxToolResultBytes] + "\n...(truncated)"
	}

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("Error from %s (HTTP %d): %s", toolName, resp.StatusCode, result), nil
	}

	return result, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
