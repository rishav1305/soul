package ws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul/internal/chat/stream"
)

// Sender matches the stream.Client's Send method.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// SubagentConfig holds the configuration for a subagent invocation.
type SubagentConfig struct {
	Task          string `json:"task"`
	MaxIterations int    `json:"max_iterations"`
}

// applyDefaults sets MaxIterations to 5 if zero and caps at 10.
func (sc *SubagentConfig) applyDefaults() {
	if sc.MaxIterations <= 0 {
		sc.MaxIterations = 5
	}
	if sc.MaxIterations > 10 {
		sc.MaxIterations = 10
	}
}

// readOnlyTools returns tool definitions for read-only code exploration.
func readOnlyTools() []stream.Tool {
	return []stream.Tool{
		{
			Name:        "file_read",
			Description: "Read the contents of a file at the given path.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Absolute or relative file path to read"}},"required":["path"]}`),
		},
		{
			Name:        "file_search",
			Description: "Search for files matching a query string in file names.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query to match against file names"},"directory":{"type":"string","description":"Directory to search in (default: project root)"}},"required":["query"]}`),
		},
		{
			Name:        "file_grep",
			Description: "Search for a pattern in file contents using grep-style matching.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regular expression pattern to search for"},"directory":{"type":"string","description":"Directory to search in (default: project root)"},"include":{"type":"string","description":"Glob pattern to filter files (e.g. *.go, *.ts)"}},"required":["pattern"]}`),
		},
		{
			Name:        "file_glob",
			Description: "Find files matching a glob pattern.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern to match (e.g. **/*.go, src/**/*.ts)"},"directory":{"type":"string","description":"Directory to search in (default: project root)"}},"required":["pattern"]}`),
		},
	}
}

const subagentSystemPrompt = `You are a code exploration subagent. Your task is to investigate code and answer questions using read-only tools. You cannot modify any files. Be thorough but concise in your findings.`

// executeSubagent runs a read-only code exploration subagent loop.
func executeSubagent(ctx context.Context, sender Sender, inputJSON []byte, projectRoot string) (string, error) {
	var config SubagentConfig
	if err := json.Unmarshal(inputJSON, &config); err != nil {
		return "", fmt.Errorf("invalid subagent input: %w", err)
	}
	config.applyDefaults()

	if config.Task == "" {
		return "", fmt.Errorf("subagent requires a task description")
	}

	tools := readOnlyTools()

	messages := []stream.Message{
		{
			Role: "user",
			Content: []stream.ContentBlock{
				{Type: "text", Text: config.Task},
			},
		},
	}

	var resultText string

	for i := 0; i < config.MaxIterations; i++ {
		req := &stream.Request{
			System:         subagentSystemPrompt,
			Messages:       messages,
			Tools:          tools,
			MaxTokens:      4096,
			SkipValidation: i > 0,
		}

		resp, err := sender.Send(ctx, req)
		if err != nil {
			return "", fmt.Errorf("subagent send failed: %w", err)
		}

		// Extract text from response.
		for _, block := range resp.Content {
			if block.Type == "text" {
				resultText += block.Text
			}
		}

		// If not a tool_use stop, we're done.
		if resp.StopReason != "tool_use" {
			break
		}

		// Extract tool_use blocks and execute them.
		var toolResults []stream.ContentBlock
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				toolOutput := executeReadOnlyTool(projectRoot, block.Name, string(block.Input))
				toolResults = append(toolResults, stream.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   toolOutput,
				})
			}
		}

		// Append assistant response and tool results to conversation.
		messages = append(messages, stream.Message{
			Role:    "assistant",
			Content: resp.Content,
		})
		messages = append(messages, stream.Message{
			Role:    "user",
			Content: toolResults,
		})

		// Clear resultText for next iteration (we want the final text).
		resultText = ""
	}

	// Truncate result to 3000 chars.
	if len(resultText) > 3000 {
		resultText = resultText[:3000] + "\n[output truncated]"
	}

	return resultText, nil
}

