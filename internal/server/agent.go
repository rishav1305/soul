package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/products"
	"github.com/rishav1305/soul/internal/session"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

const systemPrompt = `You are Soul, an AI infrastructure assistant. Use tools to help users with compliance scanning, fixing, and monitoring.

When a user asks about compliance, security, or code quality:
1. Use the available tools to scan, analyze, or fix issues
2. Report findings clearly with file paths and severity
3. Suggest fixes when possible

Always be concise and actionable.`

const maxToolIterations = 10

// AgentLoop drives the Claude AI conversation with tool routing through the
// product manager. It streams responses back to the browser via WebSocket events.
type AgentLoop struct {
	ai       *ai.Client
	products *products.Manager
	sessions *session.Store
}

// NewAgentLoop creates a new agent loop with the given dependencies.
func NewAgentLoop(aiClient *ai.Client, pm *products.Manager, sessions *session.Store) *AgentLoop {
	return &AgentLoop{
		ai:       aiClient,
		products: pm,
		sessions: sessions,
	}
}

// Run executes the agent loop for a single user message. It sends streaming
// events back to the browser via the sendEvent callback.
func (a *AgentLoop) Run(ctx context.Context, sessionID, userMessage string, sendEvent func(WSMessage)) {
	// Validate dependencies.
	if a.ai == nil {
		sendEvent(WSMessage{
			Type:      "chat.token",
			SessionID: sessionID,
			Content:   "AI is not configured. Please set the ANTHROPIC_API_KEY environment variable and restart Soul.",
		})
		sendEvent(WSMessage{
			Type:      "chat.done",
			SessionID: sessionID,
		})
		return
	}

	// Get or create session.
	sess := a.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", userMessage)

	// Build Claude tools from the product registry.
	var claudeTools []ai.ClaudeTool
	if a.products != nil {
		registry := a.products.Registry()
		for _, entry := range registry.AllTools() {
			tools := ai.BuildClaudeTools(entry.ProductName, []*soulv1.Tool{entry.Tool})
			claudeTools = append(claudeTools, tools...)
		}
	}

	// Build the system prompt with available tool names.
	sysPrompt := systemPrompt
	if len(claudeTools) > 0 {
		var toolNames []string
		for _, t := range claudeTools {
			toolNames = append(toolNames, t.Name)
		}
		sysPrompt += fmt.Sprintf("\n\nAvailable tools: %s", strings.Join(toolNames, ", "))
	}

	// Convert session messages to AI messages for the request.
	messages := buildAIMessages(sess)

	// Run the agent loop — Claude may call multiple tools iteratively.
	var fullResponse strings.Builder
	for iteration := 0; iteration < maxToolIterations; iteration++ {
		req := ai.Request{
			MaxTokens: 4096,
			System:    sysPrompt,
			Messages:  messages,
			Tools:     claudeTools,
		}

		body, err := a.ai.SendStream(ctx, req)
		if err != nil {
			log.Printf("agent: AI stream error: %v", err)
			sendEvent(WSMessage{
				Type:      "chat.token",
				SessionID: sessionID,
				Content:   fmt.Sprintf("Error contacting AI: %v", err),
			})
			sendEvent(WSMessage{
				Type:      "chat.done",
				SessionID: sessionID,
			})
			return
		}

		stopReason, toolCalls, textContent := a.processStream(ctx, sessionID, body, sendEvent)
		body.Close()

		fullResponse.WriteString(textContent)

		// If no tool calls, we're done.
		if stopReason != "tool_use" || len(toolCalls) == 0 {
			break
		}

		// Build the assistant message with all content blocks (text + tool_use).
		assistantContent := buildAssistantContent(textContent, toolCalls)
		messages = append(messages, ai.Message{
			Role:    "assistant",
			Content: assistantContent,
		})

		// Execute each tool call and build tool_result blocks.
		var toolResults []any
		for _, tc := range toolCalls {
			result := a.executeTool(ctx, sessionID, tc, sendEvent)
			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": tc.ID,
				"content":     result,
			})
		}

		// Add tool results as a user message.
		messages = append(messages, ai.Message{
			Role:    "user",
			Content: toolResults,
		})
	}

	// Signal completion.
	sendEvent(WSMessage{
		Type:      "chat.done",
		SessionID: sessionID,
	})

	// Store the assistant response.
	if fullResponse.Len() > 0 {
		sess.AddMessage("assistant", fullResponse.String())
	}
}

// toolCall holds accumulated data about a tool_use block.
type toolCall struct {
	ID    string
	Name  string
	Input string // accumulated JSON input
}

// processStream reads the SSE stream from Claude and dispatches events.
// Returns the stop reason, any tool calls, and accumulated text content.
func (a *AgentLoop) processStream(
	ctx context.Context,
	sessionID string,
	body io.Reader,
	sendEvent func(WSMessage),
) (string, []toolCall, string) {
	events := make(chan ai.StreamEvent, 64)
	go ai.ParseSSEStream(body, events)

	var (
		stopReason    string
		toolCalls     []toolCall
		textContent   strings.Builder
		currentBlock  string // "text" or "tool_use"
		currentToolID string
		currentTool   string
		toolInputBuf  strings.Builder
	)

	for ev := range events {
		switch ev.Type {
		case "content_block_start":
			var wrapper struct {
				ContentBlock ai.ContentBlockStart `json:"content_block"`
			}
			if err := json.Unmarshal(ev.Data, &wrapper); err != nil {
				log.Printf("agent: failed to parse content_block_start: %v", err)
				continue
			}
			currentBlock = wrapper.ContentBlock.Type
			if currentBlock == "tool_use" {
				currentToolID = wrapper.ContentBlock.ID
				currentTool = wrapper.ContentBlock.Name
				toolInputBuf.Reset()
			}

		case "content_block_delta":
			var wrapper struct {
				Delta ai.ContentBlockDelta `json:"delta"`
			}
			if err := json.Unmarshal(ev.Data, &wrapper); err != nil {
				log.Printf("agent: failed to parse content_block_delta: %v", err)
				continue
			}

			switch wrapper.Delta.Type {
			case "text_delta":
				text := wrapper.Delta.Text
				textContent.WriteString(text)
				sendEvent(WSMessage{
					Type:      "chat.token",
					SessionID: sessionID,
					Content:   text,
				})
			case "input_json_delta":
				toolInputBuf.WriteString(wrapper.Delta.PartialJSON)
			}

		case "content_block_stop":
			if currentBlock == "tool_use" {
				toolCalls = append(toolCalls, toolCall{
					ID:    currentToolID,
					Name:  currentTool,
					Input: toolInputBuf.String(),
				})
			}
			currentBlock = ""

		case "message_delta":
			var wrapper struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(ev.Data, &wrapper); err == nil {
				if wrapper.Delta.StopReason != "" {
					stopReason = wrapper.Delta.StopReason
				}
			}

		case "message_stop":
			// Final event — the stop reason should already be set from message_delta.
			if stopReason == "" {
				stopReason = "end_turn"
			}
		}
	}

	return stopReason, toolCalls, textContent.String()
}

// executeTool routes a tool call through the product manager and streams
// progress events to the browser. Returns the tool result text.
func (a *AgentLoop) executeTool(
	ctx context.Context,
	sessionID string,
	tc toolCall,
	sendEvent func(WSMessage),
) string {
	// Send tool.call event to the browser.
	callData, _ := json.Marshal(map[string]string{
		"tool_id": tc.ID,
		"name":    tc.Name,
		"input":   tc.Input,
	})
	sendEvent(WSMessage{
		Type:      "tool.call",
		SessionID: sessionID,
		Data:      callData,
	})

	if a.products == nil {
		return "Error: no product manager configured"
	}

	// Parse the qualified tool name: product__tool.
	registry := a.products.Registry()
	entry, found := registry.FindTool(tc.Name)
	if !found {
		return fmt.Sprintf("Error: tool %q not found", tc.Name)
	}

	// Get the gRPC client for this product.
	client, ok := a.products.GetClient(entry.ProductName)
	if !ok {
		return fmt.Sprintf("Error: product %q not available", entry.ProductName)
	}

	// Execute via streaming gRPC.
	toolReq := &soulv1.ToolRequest{
		Tool:      entry.Tool.GetName(),
		InputJson: tc.Input,
		SessionId: sessionID,
	}

	stream, err := client.ExecuteToolStream(ctx, toolReq)
	if err != nil {
		log.Printf("agent: ExecuteToolStream error: %v", err)
		return fmt.Sprintf("Error executing tool: %v", err)
	}

	var result string
	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("agent: stream recv error: %v", err)
			return fmt.Sprintf("Error reading tool stream: %v", err)
		}

		// Forward events to the browser.
		switch ev := event.GetEvent().(type) {
		case *soulv1.ToolEvent_Progress:
			progressData, _ := json.Marshal(map[string]any{
				"tool_id":  tc.ID,
				"analyzer": ev.Progress.GetAnalyzer(),
				"percent":  ev.Progress.GetPercent(),
				"message":  ev.Progress.GetMessage(),
			})
			sendEvent(WSMessage{
				Type:      "tool.progress",
				SessionID: sessionID,
				Data:      progressData,
			})

		case *soulv1.ToolEvent_Finding:
			findingData, _ := json.Marshal(map[string]any{
				"tool_id":  tc.ID,
				"id":       ev.Finding.GetId(),
				"title":    ev.Finding.GetTitle(),
				"severity": ev.Finding.GetSeverity(),
				"file":     ev.Finding.GetFile(),
				"line":     ev.Finding.GetLine(),
				"evidence": ev.Finding.GetEvidence(),
			})
			sendEvent(WSMessage{
				Type:      "tool.finding",
				SessionID: sessionID,
				Data:      findingData,
			})

		case *soulv1.ToolEvent_Complete:
			result = ev.Complete.GetOutput()
			if ev.Complete.GetStructuredJson() != "" {
				result = ev.Complete.GetStructuredJson()
			}
			completeData, _ := json.Marshal(map[string]any{
				"tool_id": tc.ID,
				"success": ev.Complete.GetSuccess(),
				"output":  ev.Complete.GetOutput(),
			})
			sendEvent(WSMessage{
				Type:      "tool.done",
				SessionID: sessionID,
				Data:      completeData,
			})

		case *soulv1.ToolEvent_Error:
			result = fmt.Sprintf("Error: %s", ev.Error.GetMessage())
			errorData, _ := json.Marshal(map[string]any{
				"tool_id": tc.ID,
				"code":    ev.Error.GetCode(),
				"message": ev.Error.GetMessage(),
			})
			sendEvent(WSMessage{
				Type:      "tool.error",
				SessionID: sessionID,
				Data:      errorData,
			})
		}
	}

	if result == "" {
		result = "Tool completed with no output"
	}
	return result
}

// buildAIMessages converts session messages to AI API format.
func buildAIMessages(sess *session.Session) []ai.Message {
	msgs := make([]ai.Message, len(sess.Messages))
	for i, m := range sess.Messages {
		msgs[i] = ai.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return msgs
}

// buildAssistantContent creates the content array for an assistant message
// that contains both text and tool_use blocks.
func buildAssistantContent(text string, calls []toolCall) []any {
	var content []any

	if text != "" {
		content = append(content, map[string]any{
			"type": "text",
			"text": text,
		})
	}

	for _, tc := range calls {
		// Parse the accumulated input JSON.
		var input any
		if tc.Input != "" {
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				input = map[string]any{}
			}
		} else {
			input = map[string]any{}
		}
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Name,
			"input": input,
		})
	}

	return content
}
