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
	Method  string        // GET, POST, PATCH, DELETE
	Path    string        // e.g., "/api/tasks/{task_id}"
	Product string        // tasks, tutor, projects, observe
	Timeout time.Duration // per-route timeout override; 0 = use default (10s)
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
			"infra":    envOr("SOUL_INFRA_URL", "http://127.0.0.1:3012"),
			"quality":  envOr("SOUL_QUALITY_URL", "http://127.0.0.1:3014"),
			"data":     envOr("SOUL_DATA_URL", "http://127.0.0.1:3016"),
			"docs":     envOr("SOUL_DOCS_URL", "http://127.0.0.1:3018"),
			"scout":    envOr("SOUL_SCOUT_URL", "http://127.0.0.1:3020"),
			"sentinel": envOr("SOUL_SENTINEL_URL", "http://127.0.0.1:3022"),
			"mesh":     envOr("SOUL_MESH_URL", "http://127.0.0.1:3024"),
			"bench":    envOr("SOUL_BENCH_URL", "http://127.0.0.1:3026"),
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
			// Infra — devops, dba, migrate
			"devops__analyze":  {Method: "POST", Path: "/api/tools/devops__analyze/execute", Product: "infra"},
			"devops__report":   {Method: "POST", Path: "/api/tools/devops__report/execute", Product: "infra"},
			"dba__analyze":     {Method: "POST", Path: "/api/tools/dba__analyze/execute", Product: "infra"},
			"dba__report":      {Method: "POST", Path: "/api/tools/dba__report/execute", Product: "infra"},
			"migrate__analyze": {Method: "POST", Path: "/api/tools/migrate__analyze/execute", Product: "infra"},
			"migrate__report":  {Method: "POST", Path: "/api/tools/migrate__report/execute", Product: "infra"},
			// Quality — compliance, qa, analytics
			"compliance__scan":   {Method: "POST", Path: "/api/tools/compliance__scan/execute", Product: "quality"},
			"compliance__fix":    {Method: "POST", Path: "/api/tools/compliance__fix/execute", Product: "quality"},
			"compliance__badge":  {Method: "POST", Path: "/api/tools/compliance__badge/execute", Product: "quality"},
			"compliance__report": {Method: "POST", Path: "/api/tools/compliance__report/execute", Product: "quality"},
			"qa__analyze":        {Method: "POST", Path: "/api/tools/qa__analyze/execute", Product: "quality"},
			"qa__report":         {Method: "POST", Path: "/api/tools/qa__report/execute", Product: "quality"},
			"analytics__analyze": {Method: "POST", Path: "/api/tools/analytics__analyze/execute", Product: "quality"},
			"analytics__report":  {Method: "POST", Path: "/api/tools/analytics__report/execute", Product: "quality"},
			// Data — dataeng, costops, viz
			"dataeng__analyze": {Method: "POST", Path: "/api/tools/dataeng__analyze/execute", Product: "data"},
			"dataeng__report":  {Method: "POST", Path: "/api/tools/dataeng__report/execute", Product: "data"},
			"costops__analyze": {Method: "POST", Path: "/api/tools/costops__analyze/execute", Product: "data"},
			"costops__report":  {Method: "POST", Path: "/api/tools/costops__report/execute", Product: "data"},
			"viz__analyze":     {Method: "POST", Path: "/api/tools/viz__analyze/execute", Product: "data"},
			"viz__report":      {Method: "POST", Path: "/api/tools/viz__report/execute", Product: "data"},
			// Docs — docs, api
			"docs__analyze": {Method: "POST", Path: "/api/tools/docs__analyze/execute", Product: "docs"},
			"docs__report":  {Method: "POST", Path: "/api/tools/docs__report/execute", Product: "docs"},
			"api__analyze":  {Method: "POST", Path: "/api/tools/api__analyze/execute", Product: "docs"},
			"api__report":   {Method: "POST", Path: "/api/tools/api__report/execute", Product: "docs"},
			// Sentinel
			"challenge_list":   {Method: "POST", Path: "/api/tools/challenge_list/execute", Product: "sentinel"},
			"challenge_start":  {Method: "POST", Path: "/api/tools/challenge_start/execute", Product: "sentinel"},
			"challenge_submit": {Method: "POST", Path: "/api/tools/challenge_submit/execute", Product: "sentinel"},
			"attack":           {Method: "POST", Path: "/api/tools/attack/execute", Product: "sentinel"},
			"sandbox_config":   {Method: "POST", Path: "/api/tools/sandbox_config/execute", Product: "sentinel"},
			"defend":           {Method: "POST", Path: "/api/tools/defend/execute", Product: "sentinel"},
			"scan":             {Method: "POST", Path: "/api/tools/scan/execute", Product: "sentinel"},
			// Bench
			"run_benchmark":  {Method: "POST", Path: "/api/tools/run_benchmark/execute", Product: "bench"},
			"run_smoke":      {Method: "POST", Path: "/api/tools/run_smoke/execute", Product: "bench"},
			"list_results":   {Method: "POST", Path: "/api/tools/list_results/execute", Product: "bench"},
			"compare_results": {Method: "POST", Path: "/api/tools/compare_results/execute", Product: "bench"},
			// Mesh
			"cluster_status": {Method: "POST", Path: "/api/tools/cluster_status/execute", Product: "mesh"},
			"list_nodes":     {Method: "POST", Path: "/api/tools/list_nodes/execute", Product: "mesh"},
			"node_info":      {Method: "POST", Path: "/api/tools/node_info/execute", Product: "mesh"},
			"link_node":      {Method: "POST", Path: "/api/tools/link_node/execute", Product: "mesh"},
			// Scout
			"lead_add":          {Method: "POST", Path: "/api/tools/lead_add/execute", Product: "scout"},
			"lead_update":       {Method: "POST", Path: "/api/tools/lead_update/execute", Product: "scout"},
			"lead_list":         {Method: "POST", Path: "/api/tools/lead_list/execute", Product: "scout"},
			"lead_get":          {Method: "POST", Path: "/api/tools/lead_get/execute", Product: "scout"},
			"analytics":         {Method: "POST", Path: "/api/tools/analytics/execute", Product: "scout"},
			"sync":              {Method: "POST", Path: "/api/tools/sync/execute", Product: "scout"},
			"sweep":             {Method: "POST", Path: "/api/tools/sweep/execute", Product: "scout"},
			"sweep_now":         {Method: "POST", Path: "/api/tools/sweep_now/execute", Product: "scout"},
			"sweep_status":      {Method: "POST", Path: "/api/tools/sweep_status/execute", Product: "scout"},
			"sweep_digest":      {Method: "POST", Path: "/api/tools/sweep_digest/execute", Product: "scout"},
			"profile":           {Method: "POST", Path: "/api/tools/profile/execute", Product: "scout"},
			"profile_pull":      {Method: "POST", Path: "/api/tools/profile_pull/execute", Product: "scout"},
			"profile_push":      {Method: "POST", Path: "/api/tools/profile_push/execute", Product: "scout"},
			"optimization_add":  {Method: "POST", Path: "/api/tools/optimization_add/execute", Product: "scout"},
			"optimization_list": {Method: "POST", Path: "/api/tools/optimization_list/execute", Product: "scout"},
			"optimize_profile":  {Method: "POST", Path: "/api/tools/optimize_profile/execute", Product: "scout"},
			"optimize_apply":    {Method: "POST", Path: "/api/tools/optimize_apply/execute", Product: "scout"},
			"lead_action":       {Method: "POST", Path: "/api/tools/lead_action/execute", Product: "scout"},
			"agent_status":      {Method: "POST", Path: "/api/tools/agent_status/execute", Product: "scout"},
			"agent_history":     {Method: "POST", Path: "/api/tools/agent_history/execute", Product: "scout"},
			"scored_leads":      {Method: "POST", Path: "/api/tools/scored_leads/execute", Product: "scout"},
			// Scout AI tools
			// Scout AI tools — 30s timeout (Claude API calls are slower than DB ops)
			"resume_match":    {Method: "POST", Path: "/api/ai/match", Product: "scout", Timeout: 30 * time.Second},
			"proposal_gen":    {Method: "POST", Path: "/api/ai/proposal", Product: "scout", Timeout: 30 * time.Second},
			"cover_letter":    {Method: "POST", Path: "/api/ai/cover-letter", Product: "scout", Timeout: 30 * time.Second},
			"cold_outreach":   {Method: "POST", Path: "/api/ai/outreach", Product: "scout", Timeout: 30 * time.Second},
			"salary_lookup":   {Method: "POST", Path: "/api/ai/salary", Product: "scout", Timeout: 30 * time.Second},
			// Async tools — 10s is fine, they return immediately with run_id
			"referral_finder": {Method: "POST", Path: "/api/ai/referral", Product: "scout"},
			"company_pitch":   {Method: "POST", Path: "/api/ai/pitch", Product: "scout"},
			// Batch 1 tools
			"resume_tailor":          {Method: "POST", Path: "/api/ai/resume-tailor", Product: "scout", Timeout: 30 * time.Second},
			"freelance_score":        {Method: "POST", Path: "/api/ai/freelance-score", Product: "scout", Timeout: 30 * time.Second},
			"networking_draft":       {Method: "POST", Path: "/api/ai/networking-draft", Product: "scout", Timeout: 30 * time.Second},
			"weekly_networking_brief": {Method: "POST", Path: "/api/ai/networking-brief", Product: "scout", Timeout: 30 * time.Second},
			"content_series_gen":     {Method: "POST", Path: "/api/ai/content-series", Product: "scout", Timeout: 30 * time.Second},
			"hook_writer":            {Method: "POST", Path: "/api/ai/hook-writer", Product: "scout", Timeout: 30 * time.Second},
			"content_topic_gen":      {Method: "POST", Path: "/api/ai/content-topic", Product: "scout", Timeout: 30 * time.Second},
			"expert_application":     {Method: "POST", Path: "/api/ai/expert-application", Product: "scout", Timeout: 30 * time.Second},
			"call_prep_brief":        {Method: "POST", Path: "/api/ai/call-prep", Product: "scout", Timeout: 30 * time.Second},
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

	// Create request with per-request timeout (route can override default 10s).
	timeout := 10 * time.Second
	if route.Timeout > 0 {
		timeout = route.Timeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
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
