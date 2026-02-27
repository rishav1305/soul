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

const systemPrompt = `You are Soul, an AI infrastructure assistant built by Rishav. You help users with compliance scanning, security analysis, fixing, and monitoring.

# Integrity
- NEVER fabricate, invent, or hallucinate tool results, file paths, findings, or scan data.
- ONLY report information that comes directly from tool execution results. Do not guess what a tool would return.
- If no tools are available, say so clearly. Do not pretend to run a scan or produce fake output.
- If a tool call fails or returns an error, report the error honestly. Do not replace it with made-up results.
- If you are unsure, say so. Do not guess.
- Do not make claims about files, code, or infrastructure you have not observed through tool output.

# Using tools
- Use the available tools to scan, analyze, or fix issues when the user asks.
- Report findings from actual tool output only. Do not add findings that are not in the data.
- Suggest fixes only when the tool data supports it.
- If the user asks for something outside your tool capabilities, say so.
- After a scan completes, individual findings are already shown in the user's compliance side panel. Do NOT list individual findings, tables, or details in chat. Instead, give a brief summary (total count, severity breakdown) and suggest next steps.

# Tone and style
- Be concise and direct. Short responses are better than padded ones.
- Do not use emojis unless the user uses them first.
- Do not be a chatbot. No filler phrases like "Great question!", "Let's get started!", "What would you like to do today?", or "How can I help?". Just answer.
- Use markdown tables and lists for structured data.
- When reporting scan results, lead with a summary (total findings, severity breakdown), then details.

# Self-awareness
- Answer questions about yourself honestly. You know your model, version, and capabilities.
- If asked what you are, say you are Soul, powered by the model specified below. Do not say "I don't have information about my model."
- Soul version: 0.2.0-alpha.`

const maxToolIterations = 10

// AgentLoop drives the Claude AI conversation with tool routing through the
// product manager. It streams responses back to the browser via WebSocket events.
type AgentLoop struct {
	ai       *ai.Client
	products *products.Manager
	sessions *session.Store
	model    string
}

// NewAgentLoop creates a new agent loop with the given dependencies.
func NewAgentLoop(aiClient *ai.Client, pm *products.Manager, sessions *session.Store, model string) *AgentLoop {
	return &AgentLoop{
		ai:       aiClient,
		products: pm,
		sessions: sessions,
		model:    model,
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

	log.Printf("[agent] run session=%s msg=%q", sessionID, userMessage)

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

	// Build the system prompt with model identity and available tool names.
	sysPrompt := systemPrompt + fmt.Sprintf("\n\nYou are powered by %s.", a.model)
	if len(claudeTools) > 0 {
		var toolNames []string
		for _, t := range claudeTools {
			toolNames = append(toolNames, t.Name)
		}
		sysPrompt += fmt.Sprintf("\n\nAvailable tools: %s", strings.Join(toolNames, ", "))
	}

	// Convert session messages to AI messages for the request.
	messages := buildAIMessages(sess)

	log.Printf("[agent] tools available: %d", len(claudeTools))

	// Run the agent loop — Claude may call multiple tools iteratively.
	var fullResponse strings.Builder
	for iteration := 0; iteration < maxToolIterations; iteration++ {
		log.Printf("[agent] iteration %d/%d", iteration+1, maxToolIterations)
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

		log.Printf("[agent] stream done stop_reason=%s tool_calls=%d text_len=%d", stopReason, len(toolCalls), len(textContent))
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
	log.Printf("[agent] tool.call id=%s name=%s input=%s", tc.ID, tc.Name, tc.Input)

	// Send tool.call event to the browser.
	callData, _ := json.Marshal(map[string]string{
		"id":    tc.ID,
		"name":  tc.Name,
		"input": tc.Input,
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
		log.Printf("[agent] ExecuteToolStream error: %v", err)
		return fmt.Sprintf("Error executing tool: %v", err)
	}

	var (
		result       string
		findingCount int
		sevCounts    = make(map[string]int)
	)
	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("[agent] stream recv error: %v", err)
			return fmt.Sprintf("Error reading tool stream: %v", err)
		}

		// Forward events to the browser.
		switch ev := event.GetEvent().(type) {
		case *soulv1.ToolEvent_Progress:
			log.Printf("[agent] tool.progress analyzer=%s percent=%.0f", ev.Progress.GetAnalyzer(), ev.Progress.GetPercent())
			progressData, _ := json.Marshal(map[string]any{
				"id":       tc.ID,
				"analyzer": ev.Progress.GetAnalyzer(),
				"progress": ev.Progress.GetPercent(),
				"message":  ev.Progress.GetMessage(),
			})
			sendEvent(WSMessage{
				Type:      "tool.progress",
				SessionID: sessionID,
				Data:      progressData,
			})

		case *soulv1.ToolEvent_Finding:
			findingCount++
			sev := strings.ToLower(ev.Finding.GetSeverity())
			sevCounts[sev]++
			log.Printf("[agent] tool.finding #%d id=%s sev=%s title=%s", findingCount, ev.Finding.GetId(), sev, ev.Finding.GetTitle())
			findingData, _ := json.Marshal(map[string]any{
				"tool_call_id": tc.ID,
				"finding": map[string]any{
					"id":       ev.Finding.GetId(),
					"title":    ev.Finding.GetTitle(),
					"severity": ev.Finding.GetSeverity(),
					"file":     ev.Finding.GetFile(),
					"line":     ev.Finding.GetLine(),
					"evidence": ev.Finding.GetEvidence(),
				},
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
			log.Printf("[agent] tool.complete success=%v output_len=%d", ev.Complete.GetSuccess(), len(result))
			completeData, _ := json.Marshal(map[string]any{
				"id":      tc.ID,
				"success": ev.Complete.GetSuccess(),
				"output":  ev.Complete.GetOutput(),
			})
			sendEvent(WSMessage{
				Type:      "tool.complete",
				SessionID: sessionID,
				Data:      completeData,
			})

		case *soulv1.ToolEvent_Error:
			result = fmt.Sprintf("Error: %s", ev.Error.GetMessage())
			log.Printf("[agent] tool.error code=%s msg=%s", ev.Error.GetCode(), ev.Error.GetMessage())
			errorData, _ := json.Marshal(map[string]any{
				"id":      tc.ID,
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

	// If findings were streamed to the browser side panel, return only a
	// compact summary to the AI so it doesn't repeat individual findings.
	if findingCount > 0 {
		var parts []string
		for sev, n := range sevCounts {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
		result = fmt.Sprintf("Scan complete: %d findings (%s). Individual findings are already displayed in the user's compliance side panel — do NOT list them again in chat.", findingCount, strings.Join(parts, ", "))
		log.Printf("[agent] tool result summarized: %s", result)
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
