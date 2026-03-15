package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/chat/session"
)

// BuiltinExecutor handles built-in tools (memories, custom tools, subagent)
// in-process without HTTP dispatch to product servers.
type BuiltinExecutor struct {
	store *session.Store
}

// NewBuiltinExecutor creates a BuiltinExecutor backed by the given session store.
func NewBuiltinExecutor(store *session.Store) *BuiltinExecutor {
	return &BuiltinExecutor{store: store}
}

// CanHandle returns true if the tool name is a built-in tool that this executor
// can handle: memory_*, tool_*, custom_*, or subagent.
func (be *BuiltinExecutor) CanHandle(toolName string) bool {
	if toolName == "" {
		return false
	}
	return strings.HasPrefix(toolName, "memory_") ||
		strings.HasPrefix(toolName, "tool_") ||
		strings.HasPrefix(toolName, "custom_") ||
		toolName == "subagent"
}

// Execute dispatches a built-in tool call and returns the JSON result.
func (be *BuiltinExecutor) Execute(ctx context.Context, toolName string, inputJSON []byte) (string, error) {
	switch {
	case strings.HasPrefix(toolName, "memory_"):
		return be.executeMemory(ctx, toolName, inputJSON)
	case strings.HasPrefix(toolName, "tool_"):
		return be.executeToolMgmt(ctx, toolName, inputJSON)
	case strings.HasPrefix(toolName, "custom_"):
		return be.executeCustom(ctx, toolName, inputJSON)
	case toolName == "subagent":
		return "", fmt.Errorf("subagent not yet implemented")
	default:
		return "", fmt.Errorf("unknown built-in tool: %s", toolName)
	}
}

// executeMemory handles memory_store, memory_search, memory_list, memory_delete.
func (be *BuiltinExecutor) executeMemory(_ context.Context, toolName string, inputJSON []byte) (string, error) {
	var input map[string]interface{}
	if err := json.Unmarshal(inputJSON, &input); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}

	switch toolName {
	case "memory_store":
		key, _ := input["key"].(string)
		content, _ := input["content"].(string)
		tags, _ := input["tags"].(string)
		if key == "" || content == "" {
			return "", fmt.Errorf("memory_store requires key and content")
		}
		mem, err := be.store.UpsertMemory(key, content, tags)
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(mem)
		return string(result), nil

	case "memory_search":
		query, _ := input["query"].(string)
		if query == "" {
			return "", fmt.Errorf("memory_search requires query")
		}
		memories, err := be.store.SearchMemories(query)
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(memories)
		return string(result), nil

	case "memory_list":
		limit := 50
		if l, ok := input["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		memories, err := be.store.ListMemories(limit)
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(memories)
		return string(result), nil

	case "memory_delete":
		key, _ := input["key"].(string)
		if key == "" {
			return "", fmt.Errorf("memory_delete requires key")
		}
		if err := be.store.DeleteMemory(key); err != nil {
			return "", err
		}
		result, _ := json.Marshal(map[string]string{"status": "deleted", "key": key})
		return string(result), nil

	default:
		return "", fmt.Errorf("unknown memory tool: %s", toolName)
	}
}

// executeToolMgmt handles tool_create, tool_list, tool_delete.
func (be *BuiltinExecutor) executeToolMgmt(_ context.Context, toolName string, inputJSON []byte) (string, error) {
	var input map[string]interface{}
	if err := json.Unmarshal(inputJSON, &input); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}

	switch toolName {
	case "tool_create":
		name, _ := input["name"].(string)
		description, _ := input["description"].(string)
		commandTemplate, _ := input["command_template"].(string)
		if name == "" || description == "" || commandTemplate == "" {
			return "", fmt.Errorf("tool_create requires name, description, and command_template")
		}
		schema, _ := input["schema"].(string)
		if schema == "" {
			schema = "{}"
		}
		tool, err := be.store.CreateCustomTool(name, description, schema, commandTemplate)
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(tool)
		return string(result), nil

	case "tool_list":
		tools, err := be.store.ListCustomTools()
		if err != nil {
			return "", err
		}
		result, _ := json.Marshal(tools)
		return string(result), nil

	case "tool_delete":
		name, _ := input["name"].(string)
		if name == "" {
			return "", fmt.Errorf("tool_delete requires name")
		}
		if err := be.store.DeleteCustomTool(name); err != nil {
			return "", err
		}
		result, _ := json.Marshal(map[string]string{"status": "deleted", "name": name})
		return string(result), nil

	default:
		return "", fmt.Errorf("unknown tool management tool: %s", toolName)
	}
}

// executeCustom handles custom_* tools by loading the tool definition from the
// store and executing the command_template via bash with input params as env vars.
func (be *BuiltinExecutor) executeCustom(ctx context.Context, toolName string, inputJSON []byte) (string, error) {
	// Strip "custom_" prefix to get the tool name.
	name := strings.TrimPrefix(toolName, "custom_")

	tool, err := be.store.GetCustomTool(name)
	if err != nil {
		return "", fmt.Errorf("custom tool not found: %s", name)
	}

	// Parse input params.
	var params map[string]interface{}
	if err := json.Unmarshal(inputJSON, &params); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}

	// Create command with 60s timeout.
	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", tool.CommandTemplate)

	// Pass params as environment variables: PARAM_<key>=<value>.
	for k, v := range params {
		envKey := "PARAM_" + strings.ToUpper(k)
		envVal := fmt.Sprintf("%v", v)
		cmd.Env = append(cmd.Env, envKey+"="+envVal)
	}

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Truncate output to 5000 chars.
	if len(result) > 5000 {
		result = result[:5000] + "\n[output truncated]"
	}

	// Return output even on non-zero exit (don't return Go error).
	if err != nil && len(result) > 0 {
		return result, nil
	}
	if err != nil {
		return fmt.Sprintf("Error executing command: %v", err), nil
	}

	return result, nil
}
