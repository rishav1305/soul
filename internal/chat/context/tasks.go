package context

import (
	"encoding/json"

	"github.com/rishav1305/soul/internal/chat/stream"
)

func tasksContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Tasks assistant. Tasks is an autonomous task execution engine — users create tasks, and an AI agent executes them in isolated git worktrees with verification gates.

Available tools let you list, create, update, start, and stop tasks. Use list_tasks to check current state before creating duplicates. Use start_task only when the user explicitly requests execution.

Key concepts: Tasks have stages (backlog → active → validation → done/blocked). Each task belongs to a product. The executor runs autonomously with step-verify-fix loops.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "list_tasks",
				Description: "List tasks, optionally filtered by stage (backlog, active, validation, done, blocked) and/or product.",
				InputSchema: mustJSON(`{"type":"object","properties":{"stage":{"type":"string","enum":["backlog","active","validation","done","blocked"],"description":"Filter by stage"},"product":{"type":"string","description":"Filter by product name"}}}`),
			},
			{
				Name:        "create_task",
				Description: "Create a new task in the backlog.",
				InputSchema: mustJSON(`{"type":"object","properties":{"title":{"type":"string","description":"Task title"},"description":{"type":"string","description":"Detailed task description"}},"required":["title","description"]}`),
			},
			{
				Name:        "get_task",
				Description: "Get full details of a specific task by ID.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
			{
				Name:        "update_task",
				Description: "Update a task's fields (title, description, stage, product).",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"},"title":{"type":"string"},"description":{"type":"string"},"stage":{"type":"string","enum":["backlog","active","validation","done","blocked"]},"product":{"type":"string"}},"required":["task_id"]}`),
			},
			{
				Name:        "start_task",
				Description: "Start autonomous execution of a task. Only use when user explicitly requests it.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
			{
				Name:        "stop_task",
				Description: "Stop a currently running task.",
				InputSchema: mustJSON(`{"type":"object","properties":{"task_id":{"type":"integer","description":"Task ID"}},"required":["task_id"]}`),
			},
		},
	}
}

// mustJSON validates and returns a JSON string as json.RawMessage.
// Panics if the input is not valid JSON — used only for compile-time
// schema definitions that should never be invalid.
func mustJSON(s string) json.RawMessage {
	var js json.RawMessage = []byte(s)
	if !json.Valid(js) {
		panic("invalid JSON in tool schema: " + s)
	}
	return js
}
