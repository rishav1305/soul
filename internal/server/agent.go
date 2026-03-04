package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/rishav1305/soul/internal/ai"
	"github.com/rishav1305/soul/internal/planner"
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

# Task management
- You have built-in tools to manage the task board: task_create, task_list, task_update.
- When the user asks to create, add, or track tasks — use the task_create tool. Do not say you cannot manage tasks.
- When the user asks to see or list tasks — use the task_list tool.
- When the user asks to update, move, or change a task — use the task_update tool.
- Tasks are created in the "backlog" stage by default. The user can ask to move them to other stages.
- Always confirm what you did after creating or updating tasks.

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

const maxToolIterations = 40

// AgentLoop drives the Claude AI conversation with tool routing through the
// product manager. It streams responses back to the browser via WebSocket events.
type AgentLoop struct {
	ai          *ai.Client
	products    *products.Manager
	sessions    *session.Store
	planner     *planner.Store
	broadcast   func(WSMessage)
	model       string
	projectRoot string // when set, enables code_* tools for file operations
}

// NewAgentLoop creates a new agent loop with the given dependencies.
func NewAgentLoop(aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string) *AgentLoop {
	return &AgentLoop{
		ai:          aiClient,
		products:    pm,
		sessions:    sessions,
		planner:     plannerStore,
		broadcast:   broadcast,
		model:       model,
		projectRoot: projectRoot,
	}
}

// Run executes the agent loop for a single user message. It sends streaming
// events back to the browser via the sendEvent callback.
// chatType selects a mode-specific system prompt extension.
// disabledTools lists tool qualified names that should be excluded.
// thinking enables extended thinking for supported models (currently Opus).
func (a *AgentLoop) Run(ctx context.Context, sessionID, userMessage, chatType string, disabledTools []string, thinking bool, sendEvent func(WSMessage)) {
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

	log.Printf("[agent] run session=%s msg=%q chatType=%s thinking=%v", sessionID, userMessage, chatType, thinking)

	// Get or create session and add the user message.
	sess := a.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", userMessage)

	// No skill injection for autonomous task execution.
	a.runLoop(ctx, sessionID, chatType, disabledTools, thinking, "", sendEvent)
}

// RunWithHistory executes the agent loop for a single user message, seeding
// the in-memory session with DB history first if the session is empty.
// This enables full context on session resume.
// skillContent is appended to the system prompt when non-empty (skills injection).
func (a *AgentLoop) RunWithHistory(ctx context.Context, sessionID, userMessage, chatType string, disabledTools []string, thinking bool, skillContent string, history []ai.Message, sendEvent func(WSMessage)) {
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

	log.Printf("[agent] run-with-history session=%s history=%d msg=%q chatType=%s thinking=%v skillContent=%d bytes",
		sessionID, len(history), userMessage, chatType, thinking, len(skillContent))

	sess := a.sessions.GetOrCreate(sessionID)

	// Seed the in-memory session from DB history if it is empty.
	// This handles the case where the server restarted or a session is being resumed.
	if len(sess.Messages) == 0 && len(history) > 0 {
		for _, h := range history {
			if content, ok := h.Content.(string); ok {
				sess.AddMessage(h.Role, content)
			}
		}
		log.Printf("[agent] seeded session %s with %d history messages", sessionID, len(sess.Messages))
	}

	sess.AddMessage("user", userMessage)

	a.runLoop(ctx, sessionID, chatType, disabledTools, thinking, skillContent, sendEvent)
}

// runLoop is the core agentic iteration loop. It assumes the session already
// has the user message appended (via Run or RunWithHistory).
// skillContent, when non-empty, is appended to the system prompt as an active skill.
func (a *AgentLoop) runLoop(ctx context.Context, sessionID, chatType string, disabledTools []string, thinking bool, skillContent string, sendEvent func(WSMessage)) {
	sess := a.sessions.GetOrCreate(sessionID)

	// Build a set of disabled tool names for fast lookup.
	disabledSet := make(map[string]bool, len(disabledTools))
	for _, name := range disabledTools {
		disabledSet[name] = true
	}

	// Build Claude tools from the product registry, filtering out disabled tools.
	var claudeTools []ai.ClaudeTool
	if a.products != nil {
		registry := a.products.Registry()
		for _, entry := range registry.AllTools() {
			qualifiedName := entry.ProductName + "__" + entry.Tool.GetName()
			if disabledSet[qualifiedName] {
				log.Printf("[agent] tool disabled: %s", qualifiedName)
				continue
			}
			tools := ai.BuildClaudeTools(entry.ProductName, []*soulv1.Tool{entry.Tool})
			claudeTools = append(claudeTools, tools...)
		}
	}

	// Add built-in task management tools.
	if a.planner != nil {
		claudeTools = append(claudeTools, builtinTaskTools()...)
	}

	// Add code tools when project root is configured.
	if a.projectRoot != "" {
		claudeTools = append(claudeTools, builtinCodeTools()...)
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

	// Append chat-type-specific prompt instructions.
	sysPrompt += chatTypePrompt(chatType)

	// Inject active skill content when provided.
	if skillContent != "" {
		sysPrompt += "\n\n---\n# Active Skill\n\n" + skillContent
	}

	// Convert session messages to AI messages for the request.
	messages := buildAIMessages(sess)

	log.Printf("[agent] tools available: %d", len(claudeTools))

	// Determine max tokens and thinking config.
	maxTokens := 16384
	var thinkingConfig *ai.ThinkingConfig
	if thinking && strings.Contains(a.model, "opus") {
		maxTokens = 32000
		thinkingConfig = &ai.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 16000,
		}
		log.Printf("[agent] extended thinking enabled: budget_tokens=16000 max_tokens=%d", maxTokens)
	}

	// Run the agent loop — Claude may call multiple tools iteratively.
	var fullResponse strings.Builder
	for iteration := 0; iteration < maxToolIterations; iteration++ {
		log.Printf("[agent] iteration %d/%d", iteration+1, maxToolIterations)
		req := ai.Request{
			MaxTokens: maxTokens,
			System:    sysPrompt,
			Messages:  messages,
			Tools:     claudeTools,
			Thinking:  thinkingConfig,
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

	// Store the assistant response in the in-memory session.
	if fullResponse.Len() > 0 {
		sess.AddMessage("assistant", fullResponse.String())
	}
}

// chatTypePrompt returns a mode-specific system prompt extension based on the chat type.
func chatTypePrompt(chatType string) string {
	switch strings.ToLower(chatType) {
	case "code":
		return "\n\n# Mode: Code\nFocus on code generation. Be concise. Show code blocks. Minimize prose."
	case "architect":
		return "\n\n# Mode: Architect\nFocus on system design and architecture. Think about scalability, trade-offs, and structure. Use diagrams when helpful."
	case "debug":
		return "\n\n# Mode: Debug\nSystematic debugging workflow. Ask for the error first. Reproduce. Diagnose step by step. Identify root cause before suggesting fixes."
	case "review":
		return "\n\n# Mode: Code Review\nReview code for bugs, security issues, performance, and style. Give structured feedback with severity levels."
	case "tdd":
		return "\n\n# Mode: TDD\nTest-driven development. Write the failing test first, then the minimal implementation to pass it. Red-green-refactor."
	case "brainstorm":
		return `

# Mode: Brainstorm — Clarify Before Acting

You are in brainstorming mode. Your ONLY job right now is to understand what the user wants to build.

**Rules:**
- NEVER write code, create files, or create tasks until the user has answered at least one clarifying question.
- Ask ONE focused question per response. Not a list — just one.
- Prefer multiple-choice questions over open-ended when possible.
- After each answer, ask the next question OR present 2-3 approaches with trade-offs.
- Only move to implementation AFTER the user explicitly approves an approach.
- Use YAGNI: remove unnecessary features from all designs.

**Question sequence:**
1. What is the core purpose / who is the user?
2. What are the constraints? (tech stack, timeline, scale)
3. What does success look like? (acceptance criteria)

Begin by understanding the request, then ask your first clarifying question.`

	case "clarify":
		return `

# Mode: Clarify

Before taking any action, ask one clarifying question to understand the user's intent.
After they answer, proceed with the most sensible interpretation.
Do NOT ask more than 2 questions before acting.`
	default:
		return ""
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
		currentBlock  string // "text", "tool_use", or "thinking"
		currentToolID string
		currentTool   string
		toolInputBuf  strings.Builder
		thinkingBuf   strings.Builder
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
			} else if currentBlock == "thinking" {
				thinkingBuf.Reset()
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
			case "thinking_delta":
				thinkingBuf.WriteString(wrapper.Delta.Thinking)
				sendEvent(WSMessage{
					Type:      "chat.thinking",
					SessionID: sessionID,
					Content:   wrapper.Delta.Thinking,
				})
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

	// Handle built-in task tools.
	if strings.HasPrefix(tc.Name, "task_") {
		return a.executeBuiltinTool(ctx, sessionID, tc, sendEvent)
	}

	// Handle built-in code tools.
	if strings.HasPrefix(tc.Name, "code_") && a.projectRoot != "" {
		result := executeCodeTool(a.projectRoot, tc)
		completeData, _ := json.Marshal(map[string]any{
			"id":      tc.ID,
			"success": !strings.HasPrefix(result, "Error"),
			"output":  result,
		})
		sendEvent(WSMessage{
			Type:      "tool.complete",
			SessionID: sessionID,
			Data:      completeData,
		})
		return result
	}

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

// builtinTaskTools returns the Claude tool definitions for built-in task management.
func builtinTaskTools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "task_create",
			Description: "Create a new task on the task board. Use this when the user asks to add, create, or track a task.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title":       {"type": "string", "description": "Short task title"},
					"description": {"type": "string", "description": "Detailed description of what needs to be done"},
					"priority":    {"type": "integer", "description": "Priority (1=highest, 5=lowest). Default 3.", "default": 3},
					"product":     {"type": "string", "description": "Product name this task belongs to (optional)"}
				},
				"required": ["title"]
			}`),
		},
		{
			Name:        "task_list",
			Description: "List tasks on the task board. Use this when the user asks to see, list, or check tasks.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"stage":   {"type": "string", "description": "Filter by stage: backlog, brainstorm, active, blocked, validation, done"},
					"product": {"type": "string", "description": "Filter by product name"}
				}
			}`),
		},
		{
			Name:        "task_update",
			Description: "Update an existing task. Use this when the user asks to change, move, or edit a task.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id":          {"type": "integer", "description": "Task ID to update"},
					"title":       {"type": "string", "description": "New title"},
					"description": {"type": "string", "description": "New description"},
					"stage":       {"type": "string", "description": "Move to stage: backlog, brainstorm, active, blocked, validation, done"},
					"priority":    {"type": "integer", "description": "New priority (1=highest, 5=lowest)"}
				},
				"required": ["id"]
			}`),
		},
	}
}

// executeBuiltinTool handles built-in task_* tools directly using the planner store.
func (a *AgentLoop) executeBuiltinTool(
	ctx context.Context,
	sessionID string,
	tc toolCall,
	sendEvent func(WSMessage),
) string {
	if a.planner == nil {
		return "Error: task store not available"
	}

	var input map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return fmt.Sprintf("Error parsing input: %v", err)
		}
	}

	var result string
	switch tc.Name {
	case "task_create":
		result = a.toolTaskCreate(input, sendEvent)
	case "task_list":
		result = a.toolTaskList(input)
	case "task_update":
		result = a.toolTaskUpdate(input, sendEvent)
	default:
		result = fmt.Sprintf("Error: unknown built-in tool %q", tc.Name)
	}

	// Send tool.complete event so the frontend updates the status badge.
	completeData, _ := json.Marshal(map[string]any{
		"id":      tc.ID,
		"success": !strings.HasPrefix(result, "Error"),
		"output":  result,
	})
	sendEvent(WSMessage{
		Type:      "tool.complete",
		SessionID: sessionID,
		Data:      completeData,
	})

	return result
}

func (a *AgentLoop) toolTaskCreate(input map[string]any, sendEvent func(WSMessage)) string {
	title, _ := input["title"].(string)
	if title == "" {
		return "Error: title is required"
	}
	description, _ := input["description"].(string)

	task := planner.NewTask(title, description)
	task.Source = "ai"

	if p, ok := input["priority"].(float64); ok {
		task.Priority = int(p)
	}
	if product, ok := input["product"].(string); ok {
		task.Product = product
	}

	id, err := a.planner.Create(task)
	if err != nil {
		return fmt.Sprintf("Error creating task: %v", err)
	}
	task.ID = id

	// Broadcast to all connected clients so the UI updates.
	if a.broadcast != nil {
		raw, _ := json.Marshal(task)
		a.broadcast(WSMessage{Type: "task.created", Data: raw})
	}

	result, _ := json.Marshal(map[string]any{
		"id":    id,
		"title": title,
		"stage": "backlog",
	})
	return string(result)
}

func (a *AgentLoop) toolTaskList(input map[string]any) string {
	filter := planner.TaskFilter{}
	if stage, ok := input["stage"].(string); ok && stage != "" {
		filter.Stage = planner.Stage(stage)
	}
	if product, ok := input["product"].(string); ok {
		filter.Product = product
	}

	tasks, err := a.planner.List(filter)
	if err != nil {
		return fmt.Sprintf("Error listing tasks: %v", err)
	}

	if len(tasks) == 0 {
		return "No tasks found."
	}

	result, _ := json.Marshal(tasks)
	return string(result)
}

func (a *AgentLoop) toolTaskUpdate(input map[string]any, sendEvent func(WSMessage)) string {
	idFloat, ok := input["id"].(float64)
	if !ok {
		return "Error: id is required"
	}
	id := int64(idFloat)

	update := planner.TaskUpdate{}
	if title, ok := input["title"].(string); ok {
		update.Title = &title
	}
	if desc, ok := input["description"].(string); ok {
		update.Description = &desc
	}
	if stage, ok := input["stage"].(string); ok {
		s := planner.Stage(stage)
		update.Stage = &s
	}
	if p, ok := input["priority"].(float64); ok {
		pri := int(p)
		update.Priority = &pri
	}

	if err := a.planner.Update(id, update); err != nil {
		return fmt.Sprintf("Error updating task: %v", err)
	}

	// Fetch updated task for broadcast.
	task, err := a.planner.Get(id)
	if err == nil && a.broadcast != nil {
		raw, _ := json.Marshal(task)
		a.broadcast(WSMessage{Type: "task.updated", Data: raw})
	}

	return fmt.Sprintf(`{"id":%d,"status":"updated"}`, id)
}
