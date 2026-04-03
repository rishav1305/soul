// Package team implements the Soul Control backend — a real-time dashboard
// for monitoring and managing the soul-team multi-agent system.
package team

import "time"

// AgentState represents the aggregated state of a single agent.
type AgentState struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`       // working, idle, blocked, offline
	CurrentTask string    `json:"current_task"`  // from heartbeat
	NextTask    string    `json:"next_task"`     // from heartbeat
	BlockedOn   string    `json:"blocked_on"`    // from heartbeat
	Since       time.Time `json:"since"`         // when status last changed
	Updated     time.Time `json:"updated"`       // last heartbeat time
	Machine     string    `json:"machine"`       // titan-pi or titan-pc
	PaneID      string    `json:"pane_id"`       // tmux pane identifier
	InboxCount  int       `json:"inbox_count"`   // unread messages
	TaskCount   int       `json:"task_count"`    // open tasks assigned
	Restarts    int       `json:"restarts"`      // from guardian DB
	SpendUSD    float64   `json:"spend_usd"`     // from guardian DB
}

// PaneDelta represents a diff of tmux pane output.
type PaneDelta struct {
	Agent     string   `json:"agent"`
	Lines     []string `json:"lines"`      // full content if first push
	DiffLines []string `json:"diff_lines"` // only changed lines (delta)
	Offset    int      `json:"offset"`     // first changed line index
	Total     int      `json:"total"`      // total line count
	Timestamp int64    `json:"timestamp"`  // unix millis
}

// ChatMessage represents a message in the inbox system.
type ChatMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Subject   string    `json:"subject,omitempty"`
	Body      string    `json:"body"`
	Priority  string    `json:"priority"` // P1, P2, P3
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // pending, delivered, read
}

// SendMessageRequest is the POST body for sending a chat message.
type SendMessageRequest struct {
	To       string `json:"to"`
	Body     string `json:"body"`
	Priority string `json:"priority,omitempty"` // defaults to P2
	From     string `json:"from,omitempty"`     // defaults to team-lead
}

// TaskItem represents a task from the backlog.
type TaskItem struct {
	Title       string `json:"title"`
	Project     string `json:"project"`
	Status      string `json:"status"`     // open, in_progress, blocked, done
	Priority    string `json:"priority"`   // 1-10
	DueDate     string `json:"due_date"`   // YYYY-MM-DD or empty
	Created     string `json:"created"`    // YYYY-MM-DD
	Description string `json:"description"`
	File        string `json:"file"` // backing file path
}

// TaskDashboard is the response from the backlog CLI dashboard command.
type TaskDashboard struct {
	Projects []ProjectTasks `json:"projects"`
	Overdue  []TaskItem     `json:"overdue"`
	DueSoon  []TaskItem     `json:"due_soon"`
	Summary  TaskSummary    `json:"summary"`
}

// ProjectTasks groups tasks by project.
type ProjectTasks struct {
	Project    string     `json:"project"`
	Open       int        `json:"open"`
	InProgress int        `json:"in_progress"`
	Blocked    int        `json:"blocked"`
	Done       int        `json:"done"`
	TopTasks   []TaskItem `json:"top_tasks"`
}

// TaskSummary gives aggregate task counts.
type TaskSummary struct {
	Total      int `json:"total"`
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
}

// CreateTaskRequest is the POST body for creating a task.
type CreateTaskRequest struct {
	Title       string `json:"title"`
	Project     string `json:"project"`
	Priority    string `json:"priority,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	Description string `json:"description,omitempty"`
	Assign      string `json:"assign,omitempty"` // agent name
}

// UpdateTaskRequest is the PATCH body for updating a task.
type UpdateTaskRequest struct {
	Status   string `json:"status,omitempty"`
	Priority string `json:"priority,omitempty"`
	Assign   string `json:"assign,omitempty"`
}

// HealthStatus represents overall system health.
type HealthStatus struct {
	Status     string            `json:"status"` // ok, degraded, error
	Agents     int               `json:"agents"`
	AgentsUp   int               `json:"agents_up"`
	Uptime     string            `json:"uptime"`
	StartedAt  time.Time         `json:"started_at"`
	Services   map[string]string `json:"services"` // service -> status
	SystemLoad float64           `json:"system_load"`
	MemoryMB   int               `json:"memory_mb"`
	Resources  SystemResources   `json:"resources"`
	TokenUsage []AgentTokenUsage `json:"token_usage"`
}

// SystemResources holds host-level metrics read from /proc and /sys.
type SystemResources struct {
	Hostname      string  `json:"hostname"`
	CPUTempC      float64 `json:"cpu_temp_c"`
	MemoryUsedMB  int     `json:"memory_used_mb"`
	MemoryTotalMB int     `json:"memory_total_mb"`
	LoadAvg1      float64 `json:"load_avg_1"`
	LoadAvg5      float64 `json:"load_avg_5"`
	LoadAvg15     float64 `json:"load_avg_15"`
}

// AgentTokenUsage holds per-agent token spend from guardian.db.
type AgentTokenUsage struct {
	Agent      string  `json:"agent"`
	TokensUsed int64   `json:"tokens_used"`
	CostUSD    float64 `json:"cost_usd"`
}

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type    string      `json:"type"`    // pane.{agent}, state.update, chat.message, task.update, health.update
	Payload interface{} `json:"payload"` // type-specific data
}
