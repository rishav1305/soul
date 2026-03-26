package context

import "github.com/rishav1305/soul/internal/chat/stream"

const memorySystemPrompt = `You have persistent memory that survives across conversations.
Use memory_store to save important information.
Use memory_search to find relevant memories.
Use memory_list to see recent memories.
Use memory_delete to remove outdated memories.
You can create custom tools using tool_create. Custom tools appear with a 'custom_' prefix.`

// builtinTools returns tool definitions available in all contexts:
// memory (4), custom tool management (3), and subagent (1).
func builtinTools() []stream.Tool {
	return []stream.Tool{
		// Memory tools
		{
			Name:        "memory_store",
			Description: "Store a key-value pair in persistent memory with optional tags for categorization.",
			InputSchema: mustJSON(`{"type":"object","properties":{"key":{"type":"string","description":"Unique key for the memory"},"content":{"type":"string","description":"Content to store"},"tags":{"type":"string","description":"Comma-separated tags for categorization"}},"required":["key","content"]}`),
		},
		{
			Name:        "memory_search",
			Description: "Search persistent memory by query string. Returns matching memories ranked by relevance.",
			InputSchema: mustJSON(`{"type":"object","properties":{"query":{"type":"string","description":"Search query"}},"required":["query"]}`),
		},
		{
			Name:        "memory_list",
			Description: "List recent memories, ordered by last updated.",
			InputSchema: mustJSON(`{"type":"object","properties":{"limit":{"type":"integer","description":"Max results to return","default":20}}}`),
		},
		{
			Name:        "memory_delete",
			Description: "Delete a memory by key.",
			InputSchema: mustJSON(`{"type":"object","properties":{"key":{"type":"string","description":"Key of the memory to delete"}},"required":["key"]}`),
		},
		// Custom tool management
		{
			Name:        "tool_create",
			Description: "Create a custom tool that will be available with a 'custom_' prefix.",
			InputSchema: mustJSON(`{"type":"object","properties":{"name":{"type":"string","description":"Tool name (will be prefixed with custom_)"},"description":{"type":"string","description":"What the tool does"},"input_schema":{"type":"string","description":"JSON schema for tool input parameters"},"command_template":{"type":"string","description":"Command template to execute"}},"required":["name","description","command_template"]}`),
		},
		{
			Name:        "tool_list",
			Description: "List all custom tools.",
			InputSchema: mustJSON(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "tool_delete",
			Description: "Delete a custom tool by name.",
			InputSchema: mustJSON(`{"type":"object","properties":{"name":{"type":"string","description":"Name of the custom tool to delete"}},"required":["name"]}`),
		},
		// Subagent
		{
			Name:        "subagent",
			Description: "Spawn a focused sub-agent to investigate a specific question or task autonomously.",
			InputSchema: mustJSON(`{"type":"object","properties":{"task":{"type":"string","description":"Focused investigation prompt"},"max_iterations":{"type":"integer","description":"Maximum iterations (1-10)","default":5}},"required":["task"]}`),
		},
	}
}
