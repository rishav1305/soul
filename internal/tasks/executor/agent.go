package executor

import (
	"context"
	"fmt"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

const (
	agentMaxTokens  = 16384
	truncateMaxLen  = 200
)

// Sender is satisfied by *stream.Client and any mock sender in tests.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// ToolCallRecord records a single tool invocation during the agent loop.
type ToolCallRecord struct {
	Name   string
	Input  string
	Output string
}

// AgentResult is returned by AgentLoop.Run and summarises the completed run.
type AgentResult struct {
	Text              string
	Iterations        int
	TotalInputTokens  int
	TotalOutputTokens int
	HitLimit          bool
	ToolCalls         []ToolCallRecord
}

// AgentLoop drives the tool-calling loop against the Claude API.
type AgentLoop struct {
	sender     Sender
	tools      *ToolSet
	taskID     int64
	maxIter    int
	onActivity func(eventType string, data map[string]interface{})
}

// NewAgentLoop creates an AgentLoop with the given sender, toolset, and limit.
func NewAgentLoop(sender Sender, tools *ToolSet, taskID int64, maxIter int, onActivity func(string, map[string]interface{})) *AgentLoop {
	return &AgentLoop{
		sender:     sender,
		tools:      tools,
		taskID:     taskID,
		maxIter:    maxIter,
		onActivity: onActivity,
	}
}

// notify fires the optional activity callback when set.
func (a *AgentLoop) notify(eventType string, data map[string]interface{}) {
	if a.onActivity != nil {
		a.onActivity(eventType, data)
	}
}

// Run executes the agent loop starting from userMessage.
// It iterates up to a.maxIter times, dispatching tool calls automatically,
// and returns an AgentResult when the model issues end_turn or the limit is hit.
func (a *AgentLoop) Run(ctx context.Context, systemPrompt, userMessage string) (*AgentResult, error) {
	messages := []stream.Message{
		{
			Role: "user",
			Content: []stream.ContentBlock{
				{Type: "text", Text: userMessage},
			},
		},
	}

	result := &AgentResult{}

	for i := 0; i < a.maxIter; i++ {
		// Respect context cancellation before each API call.
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("agent loop cancelled: %w", err)
		}

		req := &stream.Request{
			MaxTokens: agentMaxTokens,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     a.tools.Definitions(),
		}

		resp, err := a.sender.Send(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("agent loop iteration %d: send: %w", i+1, err)
		}

		result.Iterations++

		// Accumulate token usage.
		if resp.Usage != nil {
			result.TotalInputTokens += resp.Usage.InputTokens
			result.TotalOutputTokens += resp.Usage.OutputTokens
		}

		// Append assistant turn to conversation.
		messages = append(messages, stream.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		switch resp.StopReason {
		case "tool_use":
			// Build tool_result blocks for every tool_use block.
			var resultBlocks []stream.ContentBlock

			for _, block := range resp.Content {
				if block.Type != "tool_use" {
					continue
				}

				inputStr := string(block.Input)
				output, toolErr := a.tools.Execute(block.Name, inputStr)
				if toolErr != nil {
					output = fmt.Sprintf("error: %v", toolErr)
				}

				record := ToolCallRecord{
					Name:   block.Name,
					Input:  inputStr,
					Output: truncate(output, truncateMaxLen),
				}
				result.ToolCalls = append(result.ToolCalls, record)

				a.notify("tool_call", map[string]interface{}{
					"task_id": a.taskID,
					"name":    block.Name,
					"input":   truncate(inputStr, truncateMaxLen),
					"output":  truncate(output, truncateMaxLen),
				})

				resultBlocks = append(resultBlocks, stream.ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   output,
				})
			}

			// Append user message with all tool results.
			messages = append(messages, stream.Message{
				Role:    "user",
				Content: resultBlocks,
			})

		case "end_turn":
			// Collect text blocks for the final response.
			var textParts []string
			for _, block := range resp.Content {
				if block.Type == "text" && block.Text != "" {
					textParts = append(textParts, block.Text)
				}
			}
			for _, part := range textParts {
				result.Text += part
			}

			a.notify("end_turn", map[string]interface{}{
				"task_id":    a.taskID,
				"iterations": result.Iterations,
			})

			return result, nil

		default:
			// Treat any other stop reason as terminal.
			for _, block := range resp.Content {
				if block.Type == "text" {
					result.Text += block.Text
				}
			}
			return result, nil
		}
	}

	// Iteration limit reached.
	result.HitLimit = true

	a.notify("hit_limit", map[string]interface{}{
		"task_id":    a.taskID,
		"iterations": result.Iterations,
	})

	return result, nil
}

// truncate returns s truncated to maxLen characters, appending "..." when cut.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
