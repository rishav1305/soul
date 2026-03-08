package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
- You have built-in tools to manage the task board: task_create, task_list, task_update, quick_task.
- When the user asks to create, add, or track tasks — use the task_create tool. Do not say you cannot manage tasks.
- When the user asks to see or list tasks — use the task_list tool.
- When the user asks to update, move, or change a task — use the task_update tool.
- When the user wants a quick code change done autonomously — use the quick_task tool. It creates and immediately starts an autonomous agent to make the change.
- Tasks are created in the "backlog" stage by default. The user can ask to move them to other stages.
- Always confirm what you did after creating or updating tasks.

# Task creation standards
When creating tasks with task_create, ALWAYS follow these rules:
- Title must be specific and actionable (verb + object, 10+ chars). Bad: "Fix bug". Good: "Fix login timeout on session expiry"
- Description is REQUIRED. Include: what needs to change, why, and acceptance criteria as a markdown checklist (- [ ]).
- Set priority based on impact: 3=Critical (broken/blocking), 2=High (important), 1=Normal (default), 0=Low (nice-to-have).
- Set product when known (soul, scout, compliance).
- Before creating, check if a similar task already exists: use task_list to verify.
- NEVER create tasks or subtasks unless the user explicitly asks. If a task looks large, propose decomposition and wait for user approval.
- When decomposing, create a parent task first, then subtasks referencing the parent.

# Board management
When the user asks to groom, triage, plan, or review the board:
- Use task_list to get current board state.
- Flag stale tasks (>7 days in backlog) — suggest removing or reprioritizing.
- Flag stuck tasks (active >48h with no progress, blocked >5 days).
- Identify tasks missing descriptions or acceptance criteria.
- For sprint planning: recommend 3-5 tasks based on priority (highest first), then age (oldest first).
- Act directly on safe actions (add comments, fix priorities). Ask permission for all task creation and destructive actions (delete, merge duplicates, create subtasks).

# Persistent memory
- You have persistent memory that survives across conversations: memory_store, memory_search, memory_list, memory_delete.
- When the user asks you to remember something, use memory_store. When asked to recall, use memory_search or memory_list.
- Your memories are automatically loaded into your context at the start of each conversation (shown below in the Persistent Memory section if any exist).
- Do NOT say you cannot remember things or that you lose context between sessions. You have persistent memory.

# Task tracking consistency
- If you start tracking or managing tasks in a conversation, you MUST continue until the conversation ends.
- Before any context gets compressed, summarize current task status.
- Update task stage/status in real-time as work progresses.
- If you use tools to work on a task, always report completion or failure — never leave a task in limbo.

# Custom tools
- You can create new tools using tool_create. Custom tools persist across sessions and execute shell commands with parameter substitution.
- When the user asks you to create a tool, use tool_create with a name, description, input schema, and a bash command template with {{param}} placeholders.
- Custom tools appear with a "custom_" prefix in your tool list. You can use them like any other tool.
- Do NOT say you cannot create tools. You can.

# E2E testing
- You have built-in E2E testing tools that run Playwright on a remote machine (titan-pc) to verify UI changes.
- ALWAYS verify your UI changes after implementation using e2e_assert or e2e_dom. Do not claim something is done without testing.
- e2e_assert: Run assertions against the page (exists, visible, text_contains, count, eval). Use this to verify specific UI elements.
- e2e_dom: Get a structured DOM snapshot of the page. Use this to understand what's actually rendered.
- e2e_screenshot: Take a screenshot and save it. Useful for visual verification.
- e2e_check: Check if a CSS selector exists and get its text content.
- Default test URL is http://192.168.0.128:3000 (prod). For dev server use http://192.168.0.128:3001.
- After making ANY UI change (frontend code, vite build), run at least one e2e_assert or e2e_dom to confirm the change is visible.

# E2E quality standards
Test like a senior reviewer, not just "does it render":
- Check ALL state permutations. If a component has N toggleable states, test all combinations — not just the default.
- Verify visual quality: alignment, spacing, sizing, borders. If it looks "stuck on" or out of place, it fails.
- Test interactions: click targets work, hover states exist, expand/collapse transitions are smooth, resize handles drag.
- Check edge cases: empty state, overflow with lots of content, minimum/maximum widths.
- After fixing one thing, check that you didn't break something else (regression).
- "It renders" is NOT the same as "it looks correct and polished." Trust your instinct if something looks wrong.
- NEVER mark a UI task complete with only one e2e check. Test at least 3 different states/interactions.

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
	ai            *ai.Client
	products      *products.Manager
	sessions      *session.Store
	planner       *planner.Store
	broadcast     func(WSMessage)
	model         string
	projectRoot   string // when set, enables code_* tools for file operations
	autonomous    bool   // strips E2E self-verify instructions from prompt
	maxIter       int    // max tool iterations (0 = use default maxToolIterations)
	modelOverride string // when set, overrides the client's default model for this agent run
	contextBudget int    // estimated max context tokens (default 200000)
	taskMemory    map[string]string // task-scoped key-value memory
	hooks         *HookRunner       // pre/post tool execution hooks
	startTask     func(int64)       // callback to start autonomous task execution
	processor     *TaskProcessor    // for quick_task progress streaming
	pm            *PMService       // PM service for after-create hooks
	filesRead        map[string]bool // tracks files read during execution (for fingerprinting)
	iterationsUsed   int             // tracks iterations completed
	totalInputTokens  int            // cumulative input tokens across iterations
	totalOutputTokens int            // cumulative output tokens across iterations
}

// NewAgentLoop creates a new agent loop with the given dependencies.
func NewAgentLoop(aiClient *ai.Client, pm *products.Manager, sessions *session.Store, plannerStore *planner.Store, broadcast func(WSMessage), model, projectRoot string) *AgentLoop {
	budget := 200000
	if strings.Contains(model, "opus") {
		budget = 200000
	} else if strings.Contains(model, "haiku") {
		budget = 200000
	}
	return &AgentLoop{
		ai:            aiClient,
		products:      pm,
		sessions:      sessions,
		planner:       plannerStore,
		broadcast:     broadcast,
		model:         model,
		projectRoot:   projectRoot,
		contextBudget: budget,
		taskMemory:    make(map[string]string),
		hooks:         NewHookRunner(),
		filesRead:     make(map[string]bool),
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

	// Determine the effective model: modelOverride takes precedence over the default.
	effectiveModel := a.model
	if a.modelOverride != "" {
		effectiveModel = a.modelOverride
	}

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

	// Add built-in memory tools.
	if a.planner != nil {
		claudeTools = append(claudeTools, builtinMemoryTools()...)
	}

	// Add built-in E2E testing tools.
	claudeTools = append(claudeTools, builtinE2ETools()...)

	// Add built-in meta-tools for custom tool management.
	if a.planner != nil {
		claudeTools = append(claudeTools, builtinMetaTools()...)
	}

	// Load custom tools from the database.
	if a.planner != nil {
		customTools, err := a.planner.ListCustomTools()
		if err == nil {
			for _, ct := range customTools {
				schema := json.RawMessage(ct.InputSchema)
				if !json.Valid(schema) {
					schema = json.RawMessage(`{"type":"object","properties":{}}`)
				}
				claudeTools = append(claudeTools, ai.ClaudeTool{
					Name:        "custom_" + ct.Name,
					Description: ct.Description,
					InputSchema: schema,
				})
			}
		}
	}

	// Add code tools when project root is configured.
	if a.projectRoot != "" {
		claudeTools = append(claudeTools, builtinCodeTools()...)
	}

	// Build the system prompt with model identity and available tool names.
	sysPrompt := systemPrompt + fmt.Sprintf("\n\nYou are powered by %s.", effectiveModel)
	if len(claudeTools) > 0 {
		var toolNames []string
		for _, t := range claudeTools {
			toolNames = append(toolNames, t.Name)
		}
		sysPrompt += fmt.Sprintf("\n\nAvailable tools: %s", strings.Join(toolNames, ", "))
	}

	// Append chat-type-specific prompt instructions.
	sysPrompt += chatTypePrompt(chatType)

	// In autonomous mode, strip E2E self-verify instructions since the pipeline handles verification.
	if a.autonomous {
		sysPrompt = strings.Replace(sysPrompt,
			"- ALWAYS verify your UI changes after implementation using e2e_assert or e2e_dom. Do not claim something is done without testing.\n", "", 1)
		sysPrompt = strings.Replace(sysPrompt,
			"- After making ANY UI change (frontend code, vite build), run at least one e2e_assert or e2e_dom to confirm the change is visible.\n",
			"- Do NOT run E2E verification yourself — the autonomous pipeline handles build and verification automatically after your changes.\n", 1)
		sysPrompt = strings.Replace(sysPrompt,
			"- NEVER mark a UI task complete with only one e2e check. Test at least 3 different states/interactions.",
			"- The pipeline runs multi-state E2E verification after your changes. Focus on correct implementation.", 1)
	}

	// Inject active skill content when provided.
	if skillContent != "" {
		sysPrompt += "\n\n---\n# Active Skill\n\n" + skillContent
	}

	// Inject persistent memories into system prompt.
	if a.planner != nil {
		memories, err := a.planner.ListMemories(50)
		if err == nil && len(memories) > 0 {
			memSection := "\n\n# Persistent Memory\nThese are facts you have remembered from previous conversations. Use memory_store to add new memories, memory_search to find specific ones.\n"
			totalLen := len(memSection)
			for _, m := range memories {
				entry := fmt.Sprintf("- **%s**: %s", m.Key, m.Content)
				if m.Tags != "" {
					entry += fmt.Sprintf(" (tags: %s)", m.Tags)
				}
				entry += "\n"
				if totalLen+len(entry) > 8000 {
					memSection += "- ... (more memories available — use memory_search to find specific ones)\n"
					break
				}
				memSection += entry
				totalLen += len(entry)
			}
			sysPrompt += memSection
		}
	}

	// Convert session messages to AI messages for the request.
	messages := buildAIMessages(sess)

	log.Printf("[agent] tools available: %d", len(claudeTools))

	// Determine max tokens and thinking config.
	maxTokens := 16384
	var thinkingConfig *ai.ThinkingConfig
	if thinking && strings.Contains(effectiveModel, "opus") {
		maxTokens = 32000
		thinkingConfig = &ai.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 16000,
		}
		log.Printf("[agent] extended thinking enabled: budget_tokens=16000 max_tokens=%d", maxTokens)
	}

	// Run the agent loop — Claude may call multiple tools iteratively.
	iterLimit := maxToolIterations
	if a.maxIter > 0 {
		iterLimit = a.maxIter
	}

	// Stuck detection via LoopDetector.
	detector := NewLoopDetector()

	var fullResponse strings.Builder
	for iteration := 0; iteration < iterLimit; iteration++ {
		log.Printf("[agent] iteration %d/%d", iteration+1, iterLimit)

		// Context budget check.
		budget := a.contextBudget
		if budget == 0 {
			budget = 200000
		}
		usage := estimateTokens(messages, sysPrompt)
		usagePct := float64(usage) / float64(budget) * 100

		if usagePct > 85 {
			log.Printf("[agent] context emergency: %.0f%% used, restarting with brief", usagePct)
			var brief strings.Builder
			brief.WriteString("CONTEXT LIMIT REACHED. Here is a summary of your work so far:\n\n")
			count := 0
			for i := len(messages) - 1; i >= 0 && count < 3; i-- {
				if messages[i].Role == "assistant" {
					if text, ok := messages[i].Content.(string); ok && text != "" {
						brief.WriteString(text + "\n\n")
						count++
					}
				}
			}
			brief.WriteString("\nContinue from where you left off. Do NOT re-read files you already read. Do NOT repeat searches.")
			messages = []ai.Message{{Role: "user", Content: brief.String()}}
		} else if usagePct > 70 {
			messages = compressHistory(messages, iteration)
			log.Printf("[agent] context high (%.0f%%), aggressive compression applied", usagePct)
		} else if usagePct > 50 {
			messages = compressHistory(messages, iteration)
		}

		req := ai.Request{
			Model:     a.modelOverride, // empty string lets ai.Client use its default
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

		stopReason, toolCalls, textContent, inTok, outTok := a.processStream(ctx, sessionID, body, sendEvent)
		body.Close()
		a.totalInputTokens += inTok
		a.totalOutputTokens += outTok

		log.Printf("[agent] stream done stop_reason=%s tool_calls=%d text_len=%d tokens=%d/%d", stopReason, len(toolCalls), len(textContent), inTok, outTok)
		fullResponse.WriteString(textContent)

		// If no tool calls, we're done.
		if stopReason != "tool_use" || len(toolCalls) == 0 {
			break
		}

		// Adaptive planning: check if agent skipped the plan on first iteration.
		if iteration == 0 && a.autonomous && len(textContent) > 0 {
			if !strings.Contains(textContent, "## Plan") && !strings.Contains(textContent, "Files to modify") {
				planWarning := "WARNING: You did not output a plan before acting. You MUST plan first. Output your plan now before making any more tool calls."
				messages = append(messages, ai.Message{
					Role:    "user",
					Content: planWarning,
				})
			} else if a.hooks != nil {
				a.hooks.RunWorkflowHook("after:plan", map[string]string{
					"worktree": a.projectRoot,
				})
			}
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
			// Extract hook variables from tool call.
			hookVars := map[string]string{
				"tool_name": tc.Name,
				"worktree":  a.projectRoot,
			}
			if tc.Input != "" {
				var inp map[string]any
				if json.Unmarshal([]byte(tc.Input), &inp) == nil {
					if f, ok := inp["path"].(string); ok {
						hookVars["file"] = f
					}
					if c, ok := inp["command"].(string); ok {
						hookVars["input"] = c
					}
				}
			}

			// Before hook.
			var result string
			if a.hooks != nil {
				blocked, msg, _ := a.hooks.RunToolHook("before", tc.Name, hookVars)
				if blocked {
					result = fmt.Sprintf("BLOCKED: %s", msg)
					toolResults = append(toolResults, map[string]any{
						"type":        "tool_result",
						"tool_use_id": tc.ID,
						"content":     result,
					})
					detector.Record(tc.Name, tc.Input, result, iteration)
					continue
				}
			}

			result = a.executeTool(ctx, sessionID, tc, sendEvent)

			// After hook.
			if a.hooks != nil {
				_, _, hookOutput := a.hooks.RunToolHook("after", tc.Name, hookVars)
				if hookOutput != "" {
					result = result + "\n" + hookOutput
				}
			}

			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": tc.ID,
				"content":     result,
			})
			// Record in loop detector.
			detector.Record(tc.Name, tc.Input, result, iteration)
		}

		// Stuck detection: check for patterns after all tool calls in this iteration.
		if detected, warning := detector.Check(iteration, iterLimit); detected {
			log.Printf("[agent] stuck detected: %s", warning)
			if last, ok := toolResults[len(toolResults)-1].(map[string]any); ok {
				last["content"] = fmt.Sprintf("%s%s", last["content"], warning)
			}
		}

		// Add tool results as a user message.
		messages = append(messages, ai.Message{
			Role:    "user",
			Content: toolResults,
		})
		a.iterationsUsed = iteration + 1
	}

	// Signal completion with token usage.
	usagePctFinal := 0
	if a.contextBudget > 0 {
		usage := estimateTokens(messages, sysPrompt)
		usagePctFinal = int(float64(usage) / float64(a.contextBudget) * 100)
	}
	usageJSON, _ := json.Marshal(map[string]int{
		"input_tokens":  a.totalInputTokens,
		"output_tokens": a.totalOutputTokens,
		"context_pct":   usagePctFinal,
	})
	sendEvent(WSMessage{
		Type:      "chat.done",
		SessionID: sessionID,
		Data:      usageJSON,
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
) (string, []toolCall, string, int, int) {
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
		inputTokens   int
		outputTokens  int
	)

	for ev := range events {
		switch ev.Type {
		case "message_start":
			var wrapper struct {
				Message struct {
					Usage struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(ev.Data, &wrapper); err == nil {
				inputTokens += wrapper.Message.Usage.InputTokens
				outputTokens += wrapper.Message.Usage.OutputTokens
			}

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
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(ev.Data, &wrapper); err == nil {
				if wrapper.Delta.StopReason != "" {
					stopReason = wrapper.Delta.StopReason
				}
				if wrapper.Usage.OutputTokens > 0 {
					outputTokens += wrapper.Usage.OutputTokens
				}
			}

		case "message_stop":
			// Final event — the stop reason should already be set from message_delta.
			if stopReason == "" {
				stopReason = "end_turn"
			}
		}
	}

	return stopReason, toolCalls, textContent.String(), inputTokens, outputTokens
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

	// Handle task-scoped memory tool.
	if tc.Name == "task_memory" && a.projectRoot != "" {
		var input map[string]any
		if tc.Input != "" {
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				return fmt.Sprintf("Error parsing input: %v", err)
			}
		}
		result := a.executeTaskMemory(input)
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

	// Handle subagent tool.
	if tc.Name == "subagent" && a.projectRoot != "" {
		var input map[string]any
		if tc.Input != "" {
			if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
				return fmt.Sprintf("Error parsing input: %v", err)
			}
		}
		result := a.executeSubagent(input)
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

	// Handle built-in task tools.
	if strings.HasPrefix(tc.Name, "task_") || tc.Name == "quick_task" {
		return a.executeBuiltinTool(ctx, sessionID, tc, sendEvent)
	}

	// Handle built-in memory tools.
	if strings.HasPrefix(tc.Name, "memory_") {
		return a.executeBuiltinTool(ctx, sessionID, tc, sendEvent)
	}

	// Handle built-in meta-tools for custom tool management.
	if strings.HasPrefix(tc.Name, "tool_") {
		return a.executeBuiltinTool(ctx, sessionID, tc, sendEvent)
	}

	// Handle built-in E2E testing tools.
	if strings.HasPrefix(tc.Name, "e2e_") {
		result := a.executeE2ETool(tc)
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

	// Handle custom tools (dynamically created, stored in DB).
	if strings.HasPrefix(tc.Name, "custom_") && a.planner != nil {
		result := a.executeCustomTool(sessionID, tc)
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

	// Handle built-in code tools.
	if strings.HasPrefix(tc.Name, "code_") && a.projectRoot != "" {
		result := executeCodeTool(a.projectRoot, tc)
		// Track files read for fingerprinting.
		if tc.Name == "code_read" && a.filesRead != nil {
			var inp map[string]any
			if json.Unmarshal([]byte(tc.Input), &inp) == nil {
				if p, ok := inp["path"].(string); ok {
					a.filesRead[p] = true
				}
			}
		}
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

// ── LoopDetector — stuck pattern detection ──

// loopEntry records a single tool call for stuck detection.
type loopEntry struct {
	name      string
	input     string // first 200 chars of input
	file      string // extracted file path if any
	errSig    string // first 100 chars of result if error
	iteration int    // which iteration this came from
}

// LoopDetector tracks tool calls and detects stuck patterns.
type LoopDetector struct {
	history []loopEntry
	maxHist int
}

// NewLoopDetector creates a new LoopDetector that keeps the last 10 entries.
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{maxHist: 10}
}

// Record stores a tool call entry for pattern detection.
func (ld *LoopDetector) Record(name, input, result string, iteration int) {
	entry := loopEntry{
		name:      name,
		iteration: iteration,
	}
	if len(input) > 200 {
		entry.input = input[:200]
	} else {
		entry.input = input
	}
	// Extract file path from input JSON.
	var parsed map[string]any
	if json.Unmarshal([]byte(input), &parsed) == nil {
		if f, ok := parsed["file"].(string); ok {
			entry.file = f
		} else if f, ok := parsed["path"].(string); ok {
			entry.file = f
		} else if f, ok := parsed["file_path"].(string); ok {
			entry.file = f
		}
	}
	// Capture error signature.
	if strings.HasPrefix(result, "Error") || strings.HasPrefix(result, "Exit error") {
		if len(result) > 100 {
			entry.errSig = result[:100]
		} else {
			entry.errSig = result
		}
	}
	ld.history = append(ld.history, entry)
	if len(ld.history) > ld.maxHist {
		ld.history = ld.history[len(ld.history)-ld.maxHist:]
	}
}

// Check detects stuck patterns and returns a warning if found.
func (ld *LoopDetector) Check(iteration, maxIter int) (bool, string) {
	h := ld.history

	// (e) Running low: current iteration > 70% of max iterations.
	if maxIter > 0 && float64(iteration) > float64(maxIter)*0.7 {
		return true, fmt.Sprintf(
			"\n\nWARNING: You're at %d/%d iterations. Wrap up: commit what works, document what's left in a task_comment.",
			iteration, maxIter)
	}

	// (a) Search loop: same tool name 3+ times in last 6 calls with similar input.
	if len(h) >= 3 {
		window := h
		if len(window) > 6 {
			window = window[len(window)-6:]
		}
		// Group by tool name.
		byName := make(map[string][]loopEntry)
		for _, e := range window {
			byName[e.name] = append(byName[e.name], e)
		}
		for name, entries := range byName {
			if len(entries) >= 3 {
				// Check for similar input in any pair.
				hasSimilar := false
				for i := 0; i < len(entries) && !hasSimilar; i++ {
					for j := i + 1; j < len(entries); j++ {
						if stringSimilarity(entries[i].input, entries[j].input) > 0.7 {
							hasSimilar = true
							break
						}
					}
				}
				if hasSimilar {
					return true, fmt.Sprintf(
						"\n\nWARNING — STUCK: You've searched with `%s` 3 times with similar input. Use what you have or try a completely different approach. Consider reading a file directly with code_read.",
						name)
				}
			}
		}
	}

	// (b) Edit thrashing: code_edit on same file 3+ times in last 5 calls.
	if len(h) >= 3 {
		window := h
		if len(window) > 5 {
			window = window[len(window)-5:]
		}
		fileCounts := make(map[string]int)
		for _, e := range window {
			if e.name == "code_edit" && e.file != "" {
				fileCounts[e.file]++
			}
		}
		for file, count := range fileCounts {
			if count >= 3 {
				return true, fmt.Sprintf(
					"\n\nWARNING — STUCK: You've edited %s %d times. Stop. Re-read the full file with code_read, then make ONE correct edit.",
					file, count)
			}
		}
	}

	// (c) Build loop: code_exec returns similar error 2+ times in last 4 calls.
	if len(h) >= 2 {
		window := h
		if len(window) > 4 {
			window = window[len(window)-4:]
		}
		var execErrors []loopEntry
		for _, e := range window {
			if e.name == "code_exec" && e.errSig != "" {
				execErrors = append(execErrors, e)
			}
		}
		if len(execErrors) >= 2 {
			for i := 0; i < len(execErrors); i++ {
				for j := i + 1; j < len(execErrors); j++ {
					if stringSimilarity(execErrors[i].errSig, execErrors[j].errSig) > 0.6 {
						return true, "\n\nWARNING — STUCK: Same error twice from code_exec. Read the error carefully. Identify the root cause. Don't retry the same fix."
					}
				}
			}
		}
	}

	// (d) Blind execution: 5+ consecutive tool calls with no assistant text between them.
	if len(h) >= 5 {
		last5 := h[len(h)-5:]
		allSameIter := true
		iter := last5[0].iteration
		for _, e := range last5[1:] {
			if e.iteration != iter {
				allSameIter = false
				break
			}
		}
		if allSameIter {
			return true, "\n\nWARNING: Pause. Summarize what you've learned so far and what you're trying to do next before making more tool calls."
		}
	}

	return false, ""
}

// stringSimilarity computes a simple character frequency overlap ratio.
func stringSimilarity(a, b string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	freq := make(map[byte]int)
	for i := 0; i < len(a); i++ {
		freq[a[i]]++
	}
	matches := 0
	for i := 0; i < len(b); i++ {
		if freq[b[i]] > 0 {
			matches++
			freq[b[i]]--
		}
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	return float64(matches) / float64(maxLen)
}

// ── Task-Scoped Memory ──

// executeTaskMemory handles the task_memory tool for storing/recalling facts.
func (a *AgentLoop) executeTaskMemory(input map[string]any) string {
	action, _ := input["action"].(string)
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)

	switch action {
	case "store":
		if key == "" || value == "" {
			return "Error: key and value required for store"
		}
		a.taskMemory[key] = value
		return fmt.Sprintf("Stored: %s = %s", key, value)
	case "recall":
		if key == "" {
			return "Error: key required for recall"
		}
		if v, ok := a.taskMemory[key]; ok {
			return fmt.Sprintf("%s = %s", key, v)
		}
		return fmt.Sprintf("No memory found for key: %s", key)
	case "list":
		if len(a.taskMemory) == 0 {
			return "No facts stored yet."
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d facts stored:\n", len(a.taskMemory))
		for k, v := range a.taskMemory {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
		return b.String()
	default:
		return "Error: action must be store, recall, or list"
	}
}

// ── Subagent ──

// executeSubagent spawns a fresh AgentLoop for focused codebase exploration.
func (a *AgentLoop) executeSubagent(input map[string]any) string {
	task, _ := input["task"].(string)
	if task == "" {
		return "Error: task is required"
	}

	maxIter := 5
	if m, ok := input["max_iterations"].(float64); ok && int(m) > 0 {
		maxIter = int(m)
		if maxIter > 10 {
			maxIter = 10
		}
	}

	log.Printf("[subagent] spawning for task: %s (max_iter=%d)", task, maxIter)

	// Create a fresh agent with read-only tools only.
	sub := &AgentLoop{
		ai:          a.ai,
		products:    nil,
		sessions:    session.NewStore(),
		planner:     nil,
		broadcast:   func(WSMessage) {},
		model:       a.model,
		projectRoot: a.projectRoot,
		taskMemory:  make(map[string]string),
	}
	sub.maxIter = maxIter

	// Create a temporary session and run.
	sessionID := fmt.Sprintf("subagent-%d", time.Now().UnixNano())
	sess := sub.sessions.GetOrCreate(sessionID)
	sess.AddMessage("user", task)

	// Build read-only code tools (no code_write, code_edit, code_exec).
	readOnlyTools := []ai.ClaudeTool{}
	for _, t := range builtinCodeTools() {
		if t.Name == "code_read" || t.Name == "code_search" || t.Name == "code_grep" || t.Name == "code_glob" {
			readOnlyTools = append(readOnlyTools, t)
		}
	}

	// Build minimal system prompt.
	sysPrompt := fmt.Sprintf("You are a codebase exploration agent. Answer the question using the available tools. Be concise and direct. Project root: %s\n\nYou are powered by %s.", a.projectRoot, a.model)

	messages := buildAIMessages(sess)

	var result strings.Builder
	for iteration := 0; iteration < maxIter; iteration++ {
		req := ai.Request{
			MaxTokens: 4096,
			System:    sysPrompt,
			Messages:  messages,
			Tools:     readOnlyTools,
		}

		body, err := a.ai.SendStream(context.Background(), req)
		if err != nil {
			return fmt.Sprintf("Subagent error: %v", err)
		}

		// Process stream silently (no sendEvent).
		stopReason, toolCalls, textContent, _, _ := sub.processStream(context.Background(), sessionID, body, func(WSMessage) {})
		body.Close()

		result.WriteString(textContent)

		if stopReason != "tool_use" || len(toolCalls) == 0 {
			break
		}

		// Build assistant message.
		assistantContent := buildAssistantContent(textContent, toolCalls)
		messages = append(messages, ai.Message{Role: "assistant", Content: assistantContent})

		// Execute read-only tools.
		var toolResults []any
		for _, tc := range toolCalls {
			var toolResult string
			switch tc.Name {
			case "code_read", "code_search", "code_grep", "code_glob":
				toolResult = executeCodeTool(a.projectRoot, tc)
			default:
				toolResult = fmt.Sprintf("Error: tool %s not available in subagent", tc.Name)
			}
			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": tc.ID,
				"content":     toolResult,
			})
		}
		messages = append(messages, ai.Message{Role: "user", Content: toolResults})
	}

	finalResult := result.String()
	if len(finalResult) > 3000 {
		finalResult = finalResult[:3000] + "\n... (truncated)"
	}

	log.Printf("[subagent] completed, result length=%d", len(finalResult))
	return finalResult
}

// ── Tool Result Compression ──

// compressHistory shrinks old tool results to keep context fresh.
// - Last 3 iterations: full output
// - 4-8 iterations ago: keep first 10 + last 5 lines, middle replaced
// - 9+ iterations ago: replace with 1-line summary
func compressHistory(messages []ai.Message, currentIter int) []ai.Message {
	if len(messages) == 0 {
		return messages
	}

	// Walk backwards through messages, counting iterations by assistant/user pairs.
	// Each iteration produces an assistant message (with tool_use) + a user message (with tool_result).
	iterFromEnd := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" {
			iterFromEnd++
		}

		// Only compress user messages (they contain tool results).
		if msg.Role != "user" {
			continue
		}

		contentSlice, ok := msg.Content.([]any)
		if !ok {
			continue
		}

		// Skip recent iterations — keep full output.
		if iterFromEnd <= 3 {
			continue
		}

		for j, block := range contentSlice {
			bm, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if bm["type"] != "tool_result" {
				continue
			}
			content, ok := bm["content"].(string)
			if !ok || len(content) < 500 {
				continue
			}

			if iterFromEnd >= 9 {
				// Full compression: 1-line summary.
				toolID, _ := bm["tool_use_id"].(string)
				summary := compressSummary(content, toolID, messages, i)
				bm["content"] = summary
				contentSlice[j] = bm
			} else {
				// Partial compression: keep first 10 + last 5 lines.
				lines := strings.Split(content, "\n")
				if len(lines) > 15 {
					omitted := len(lines) - 15
					var compressed strings.Builder
					for _, l := range lines[:10] {
						compressed.WriteString(l)
						compressed.WriteByte('\n')
					}
					compressed.WriteString(fmt.Sprintf("... (%d lines omitted)\n", omitted))
					for _, l := range lines[len(lines)-5:] {
						compressed.WriteString(l)
						compressed.WriteByte('\n')
					}
					bm["content"] = compressed.String()
					contentSlice[j] = bm
				}
			}
		}
		messages[i].Content = contentSlice
	}

	return messages
}

// compressSummary generates a 1-line summary for a tool result.
func compressSummary(content, toolUseID string, messages []ai.Message, userMsgIdx int) string {
	// Try to find the tool name from the preceding assistant message.
	toolName := ""
	if userMsgIdx > 0 {
		prevMsg := messages[userMsgIdx-1]
		if prevMsg.Role == "assistant" {
			if blocks, ok := prevMsg.Content.([]any); ok {
				for _, b := range blocks {
					if bm, ok := b.(map[string]any); ok {
						if bm["type"] == "tool_use" {
							if id, ok := bm["id"].(string); ok && id == toolUseID {
								toolName, _ = bm["name"].(string)
							}
						}
					}
				}
			}
		}
	}

	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	switch {
	case toolName == "code_grep" || toolName == "code_search":
		// Count files mentioned — lines with file paths.
		fileSet := make(map[string]bool)
		for _, l := range lines {
			if strings.Contains(l, "/") || strings.Contains(l, ".go") || strings.Contains(l, ".ts") {
				parts := strings.Fields(l)
				if len(parts) > 0 {
					fileSet[parts[0]] = true
				}
			}
		}
		return fmt.Sprintf("[compressed] %s: found matches in %d files (%d lines of output)", toolName, len(fileSet), lineCount)

	case toolName == "code_read":
		return fmt.Sprintf("[compressed] Read file, %d lines", lineCount)

	case toolName == "code_exec":
		// Try to detect exit code.
		if strings.HasPrefix(content, "Exit error") || strings.HasPrefix(content, "Error") {
			firstLine := lines[0]
			if len(firstLine) > 80 {
				firstLine = firstLine[:80]
			}
			return fmt.Sprintf("[compressed] Executed command, error: %s", firstLine)
		}
		return fmt.Sprintf("[compressed] Executed command, %d lines of output", lineCount)

	default:
		if toolName != "" {
			return fmt.Sprintf("[compressed] %s completed (%d lines)", toolName, lineCount)
		}
		return fmt.Sprintf("[compressed] Tool completed (%d lines)", lineCount)
	}
}

// ── Context Budget ──

// estimateTokens gives a rough token count (4 chars per token approximation).
func estimateTokens(messages []ai.Message, sysPrompt string) int {
	total := len(sysPrompt) / 4
	for _, m := range messages {
		switch c := m.Content.(type) {
		case string:
			total += len(c) / 4
		case []any:
			for _, block := range c {
				if bm, ok := block.(map[string]any); ok {
					if content, ok := bm["content"].(string); ok {
						total += len(content) / 4
					}
					if input, ok := bm["input"].(string); ok {
						total += len(input) / 4
					}
				}
			}
		}
	}
	return total
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
			Description: "Update an existing task's stage, title, or priority. Use this when the user asks to change, move, or edit a task. Do NOT use this to write findings, gaps, or analysis — post those as comments instead.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id":       {"type": "integer", "description": "Task ID to update"},
					"title":    {"type": "string", "description": "New title"},
					"stage":    {"type": "string", "description": "Move to stage: backlog, brainstorm, active, blocked, validation, done"},
					"priority": {"type": "integer", "description": "New priority (1=highest, 5=lowest)"}
				},
				"required": ["id"]
			}`),
		},
		{
			Name:        "quick_task",
			Description: "Create and immediately start an autonomous task. Use this when the user wants a quick code change done — one message turns into a working change on the dev server. The task is auto-classified as micro workflow and starts executing immediately.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title":       {"type": "string", "description": "Short task title describing the change"},
					"description": {"type": "string", "description": "Detailed description of what to implement"},
					"product":     {"type": "string", "description": "Product name (e.g. soul, compliance, scout)"}
				},
				"required": ["title"]
			}`),
		},
		{
			Name:        "task_comment",
			Description: "Post a comment on a task. Use this to record findings, implementation gaps, analysis, or status updates — never overwrite the task description for these purposes. You can attach screenshot filenames from e2e_screenshot to show visual evidence.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"task_id":     {"type": "integer", "description": "Task ID to comment on"},
					"body":        {"type": "string", "description": "Comment body (markdown supported)"},
					"type":        {"type": "string", "description": "Comment type: feedback, status, verification, error", "enum": ["feedback", "status", "verification", "error"]},
					"attachments": {"type": "array", "items": {"type": "string"}, "description": "Screenshot filenames from e2e_screenshot to attach (e.g. [\"task-35-20260305-120000.png\"])"}
				},
				"required": ["task_id", "body"]
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
	case "task_comment":
		result = a.toolTaskComment(input, sendEvent)
	case "quick_task":
		result = a.toolQuickTask(input, sendEvent)
	case "memory_store":
		result = a.toolMemoryStore(input)
	case "memory_search":
		result = a.toolMemorySearch(input)
	case "memory_list":
		result = a.toolMemoryList(input)
	case "memory_delete":
		result = a.toolMemoryDelete(input)
	case "tool_create":
		result = a.toolToolCreate(input)
	case "tool_list":
		result = a.toolToolList(input)
	case "tool_delete":
		result = a.toolToolDelete(input)
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
	if len(strings.TrimSpace(title)) < 10 {
		return "Error: Title too short (minimum 10 chars). Use a specific, actionable title with a verb and object. Example: 'Add logout button to sidebar'"
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

	// Run PM checks asynchronously.
	if a.pm != nil {
		a.pm.AfterCreate(task)
	}

	result, _ := json.Marshal(map[string]any{
		"id":    id,
		"title": title,
		"stage": "backlog",
	})
	return string(result)
}

func (a *AgentLoop) toolQuickTask(input map[string]any, sendEvent func(WSMessage)) string {
	title, _ := input["title"].(string)
	if title == "" {
		return "Error: title is required"
	}
	description, _ := input["description"].(string)

	task := planner.NewTask(title, description)
	task.Source = "ai"
	task.Priority = 2 // high priority for quick actions

	if product, ok := input["product"].(string); ok {
		task.Product = product
	}

	// Set autonomous + micro workflow in metadata.
	meta := map[string]any{
		"autonomous": true,
		"workflow":   "micro",
	}
	metaJSON, _ := json.Marshal(meta)
	task.Metadata = string(metaJSON)

	id, err := a.planner.Create(task)
	if err != nil {
		return fmt.Sprintf("Error creating task: %v", err)
	}
	task.ID = id

	// Broadcast to UI.
	if a.broadcast != nil {
		raw, _ := json.Marshal(task)
		a.broadcast(WSMessage{Type: "task.created", Data: raw})
	}

	// Start autonomous execution and stream progress to chat.
	if a.startTask != nil {
		// Register listener to forward task activity as chat tokens.
		done := make(chan struct{})
		if a.processor != nil {
			a.processor.AddListener(id, func(actType, content string) {
				if actType == "done" {
					close(done)
					return
				}
				if actType == "status" || actType == "stage" {
					sendEvent(WSMessage{
						Type:    "chat.token",
						Content: fmt.Sprintf("\n> **[Task #%d]** %s\n", id, content),
					})
				}
			})
		}

		a.startTask(id)

		// Wait for task to complete (with timeout).
		select {
		case <-done:
			// Task finished.
		case <-time.After(5 * time.Minute):
			// Timeout — don't block the chat forever.
		}

		if a.processor != nil {
			a.processor.RemoveListeners(id)
		}

		// Check final state.
		if updated, err := a.planner.Get(id); err == nil {
			return fmt.Sprintf("Quick task #%d completed (stage: %s): %s", id, updated.Stage, title)
		}
		return fmt.Sprintf("Quick task #%d finished: %s", id, title)
	}

	return fmt.Sprintf("Quick task #%d created: %s (start manually — autonomous processor not available)", id, title)
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
	// description is intentionally not accepted here — the original plan must
	// never be overwritten by the agent. Findings and gaps go in comments.
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

func (a *AgentLoop) toolTaskComment(input map[string]any, sendEvent func(WSMessage)) string {
	taskIDFloat, ok := input["task_id"].(float64)
	if !ok {
		return "Error: task_id is required"
	}
	taskID := int64(taskIDFloat)

	body, _ := input["body"].(string)
	if body == "" {
		return "Error: body is required"
	}

	commentType, _ := input["type"].(string)
	if commentType == "" {
		commentType = "feedback"
	}

	// Parse attachments array (screenshot filenames).
	var attachments []string
	if rawAttach, ok := input["attachments"].([]any); ok {
		for _, v := range rawAttach {
			if s, ok := v.(string); ok && s != "" {
				attachments = append(attachments, s)
			}
		}
	}
	if attachments == nil {
		attachments = []string{}
	}

	comment := planner.Comment{
		TaskID:      taskID,
		Author:      "soul",
		Type:        commentType,
		Body:        body,
		Attachments: attachments,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	id, err := a.planner.CreateComment(comment)
	if err != nil {
		return fmt.Sprintf("Error creating comment: %v", err)
	}
	comment.ID = id

	if a.broadcast != nil {
		raw, _ := json.Marshal(comment)
		a.broadcast(WSMessage{Type: "task.comment.added", Data: raw})
	}

	return fmt.Sprintf(`{"id":%d,"task_id":%d,"status":"comment posted"}`, id, taskID)
}

// ── Memory Tools ──

func builtinMemoryTools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "memory_store",
			Description: "Save or update a persistent memory that survives across conversations. Use this to remember important facts, preferences, project conventions, or context. If a memory with this key already exists, it will be updated.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key":     {"type": "string", "description": "Unique identifier for this memory (e.g., 'project_stack', 'user_preference_timezone')"},
					"content": {"type": "string", "description": "The information to remember"},
					"tags":    {"type": "string", "description": "Comma-separated tags for categorization (e.g., 'project,config')"}
				},
				"required": ["key", "content"]
			}`),
		},
		{
			Name:        "memory_search",
			Description: "Search persistent memories by keyword. Searches across keys, content, and tags.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Search keyword to find in memory keys, content, or tags"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "memory_list",
			Description: "List all persistent memories.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "memory_delete",
			Description: "Delete a persistent memory by key.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {"type": "string", "description": "Key of the memory to delete"}
				},
				"required": ["key"]
			}`),
		},
	}
}

func (a *AgentLoop) toolMemoryStore(input map[string]any) string {
	key, _ := input["key"].(string)
	if key == "" {
		return "Error: key is required"
	}
	content, _ := input["content"].(string)
	if content == "" {
		return "Error: content is required"
	}
	tags, _ := input["tags"].(string)

	mem, err := a.planner.UpsertMemory(key, content, tags)
	if err != nil {
		return fmt.Sprintf("Error storing memory: %v", err)
	}
	result, _ := json.Marshal(map[string]any{
		"key":    mem.Key,
		"status": "stored",
	})
	return string(result)
}

func (a *AgentLoop) toolMemorySearch(input map[string]any) string {
	query, _ := input["query"].(string)
	if query == "" {
		return "Error: query is required"
	}
	memories, err := a.planner.SearchMemories(query)
	if err != nil {
		return fmt.Sprintf("Error searching memories: %v", err)
	}
	if len(memories) == 0 {
		return "No memories found matching query."
	}
	result, _ := json.Marshal(memories)
	return string(result)
}

func (a *AgentLoop) toolMemoryList(input map[string]any) string {
	memories, err := a.planner.ListMemories(50)
	if err != nil {
		return fmt.Sprintf("Error listing memories: %v", err)
	}
	if len(memories) == 0 {
		return "No memories stored yet."
	}
	result, _ := json.Marshal(memories)
	return string(result)
}

func (a *AgentLoop) toolMemoryDelete(input map[string]any) string {
	key, _ := input["key"].(string)
	if key == "" {
		return "Error: key is required"
	}
	err := a.planner.DeleteMemory(key)
	if err != nil {
		return fmt.Sprintf("Error deleting memory: %v", err)
	}
	return fmt.Sprintf(`{"key":%q,"status":"deleted"}`, key)
}

// ── Meta-Tools for Custom Tool Management ──

func builtinMetaTools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "tool_create",
			Description: "Create a new custom tool that persists across sessions. The tool executes a shell command template with parameter substitution. Use {{param_name}} placeholders in the command template that will be replaced with actual values when the tool is called.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name":             {"type": "string", "description": "Tool name (alphanumeric and underscores only, e.g., 'fetch_history')"},
					"description":      {"type": "string", "description": "What the tool does (shown to the AI in future sessions)"},
					"input_schema":     {"type": "string", "description": "JSON Schema string defining the tool's parameters"},
					"command_template": {"type": "string", "description": "Bash command with {{param}} placeholders (e.g., 'sqlite3 ~/.soul/planner.db \"SELECT * FROM chat_messages WHERE session_id={{session_id}}\"')"}
				},
				"required": ["name", "description", "input_schema", "command_template"]
			}`),
		},
		{
			Name:        "tool_list",
			Description: "List all custom tools that have been created.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "tool_delete",
			Description: "Delete a custom tool by name.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Name of the custom tool to delete"}
				},
				"required": ["name"]
			}`),
		},
	}
}

func (a *AgentLoop) toolToolCreate(input map[string]any) string {
	name, _ := input["name"].(string)
	if name == "" {
		return "Error: name is required"
	}
	// Validate name: alphanumeric and underscores only.
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return "Error: name must be alphanumeric with underscores only"
		}
	}
	// Prevent collisions with built-in prefixes.
	for _, prefix := range []string{"task_", "memory_", "code_", "tool_", "custom_"} {
		if strings.HasPrefix(name, prefix) {
			return fmt.Sprintf("Error: name cannot start with reserved prefix %q", prefix)
		}
	}

	description, _ := input["description"].(string)
	if description == "" {
		return "Error: description is required"
	}
	inputSchema, _ := input["input_schema"].(string)
	if inputSchema == "" {
		return "Error: input_schema is required"
	}
	if !json.Valid([]byte(inputSchema)) {
		return "Error: input_schema must be valid JSON"
	}
	commandTemplate, _ := input["command_template"].(string)
	if commandTemplate == "" {
		return "Error: command_template is required"
	}

	ct, err := a.planner.CreateCustomTool(name, description, inputSchema, commandTemplate)
	if err != nil {
		return fmt.Sprintf("Error creating tool: %v", err)
	}
	result, _ := json.Marshal(map[string]any{
		"name":   ct.Name,
		"status": "created",
		"note":   "Tool is now available as custom_" + ct.Name,
	})
	return string(result)
}

func (a *AgentLoop) toolToolList(input map[string]any) string {
	tools, err := a.planner.ListCustomTools()
	if err != nil {
		return fmt.Sprintf("Error listing tools: %v", err)
	}
	if len(tools) == 0 {
		return "No custom tools defined yet."
	}
	result, _ := json.Marshal(tools)
	return string(result)
}

func (a *AgentLoop) toolToolDelete(input map[string]any) string {
	name, _ := input["name"].(string)
	if name == "" {
		return "Error: name is required"
	}
	err := a.planner.DeleteCustomTool(name)
	if err != nil {
		return fmt.Sprintf("Error deleting tool: %v", err)
	}
	return fmt.Sprintf(`{"name":%q,"status":"deleted"}`, name)
}

// ── Custom Tool Execution ──

// shellescape wraps a string in single quotes for safe shell interpolation.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (a *AgentLoop) executeCustomTool(sessionID string, tc toolCall) string {
	toolName := strings.TrimPrefix(tc.Name, "custom_")

	ct, err := a.planner.GetCustomTool(toolName)
	if err != nil {
		return fmt.Sprintf("Error: custom tool %q not found", toolName)
	}

	// Parse input parameters.
	var params map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &params); err != nil {
			return fmt.Sprintf("Error parsing input: %v", err)
		}
	}

	// Substitute {{param}} placeholders in the command template.
	command := ct.CommandTemplate
	for key, val := range params {
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		case float64:
			strVal = fmt.Sprintf("%v", v)
		case bool:
			strVal = fmt.Sprintf("%v", v)
		default:
			b, _ := json.Marshal(v)
			strVal = string(b)
		}
		strVal = shellescape(strVal)
		command = strings.ReplaceAll(command, "{{"+key+"}}", strVal)
	}

	// Check for unresolved placeholders.
	if strings.Contains(command, "{{") {
		return "Error: unresolved placeholders in command template. Provide all required parameters."
	}

	log.Printf("[custom_tool] executing %q: %s", toolName, command)

	// Execute via bash -c with timeout (same pattern as code_exec).
	cmd := exec.Command("bash", "-c", command)
	if a.projectRoot != "" {
		cmd.Dir = a.projectRoot
	}

	done := make(chan error, 1)
	var out []byte
	go func() {
		var execErr error
		out, execErr = cmd.CombinedOutput()
		done <- execErr
	}()

	select {
	case execErr := <-done:
		result := string(out)
		if len(result) > 5000 {
			result = result[:5000] + "\n... (output truncated)"
		}
		if execErr != nil {
			return fmt.Sprintf("Exit error: %v\n%s", execErr, result)
		}
		if result == "" {
			result = "Command completed with no output."
		}
		return result
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return "Error: command timed out after 60 seconds"
	}
}

// ── E2E Testing Tools ──

const e2eTestRunner = "ssh titan-pc 'cd ~/soul-e2e && node test-runner.js'"
const e2eDefaultURL = "http://192.168.0.128:3000"

func builtinE2ETools() []ai.ClaudeTool {
	return []ai.ClaudeTool{
		{
			Name:        "e2e_assert",
			Description: "Run E2E assertions against the Soul UI via headless browser on titan-pc. Returns pass/fail for each assertion. Use this to verify UI changes are visible.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {"type": "string", "description": "Page URL to test (default: http://192.168.0.128:3000)"},
					"assertions": {
						"type": "array",
						"description": "List of assertions to check",
						"items": {
							"type": "object",
							"properties": {
								"type": {"type": "string", "enum": ["exists", "visible", "text_contains", "count", "eval"], "description": "Assertion type"},
								"selector": {"type": "string", "description": "CSS selector to check"},
								"expected": {"type": "string", "description": "Expected text (for text_contains) or count (for count)"},
								"min": {"type": "integer", "description": "Minimum count (for count assertions)"},
								"expression": {"type": "string", "description": "JS expression (for eval assertions)"}
							},
							"required": ["type"]
						}
					}
				},
				"required": ["assertions"]
			}`),
		},
		{
			Name:        "e2e_dom",
			Description: "Get a structured DOM snapshot of the Soul UI page. Shows tag names, IDs, classes, and text content in a tree format. Use this to understand what is actually rendered.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {"type": "string", "description": "Page URL (default: http://192.168.0.128:3000)"},
					"selector": {"type": "string", "description": "CSS selector to snapshot (default: #root)"}
				}
			}`),
		},
		{
			Name:        "e2e_check",
			Description: "Check if elements matching a CSS selector exist and get their text content.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {"type": "string", "description": "Page URL (default: http://192.168.0.128:3000)"},
					"selector": {"type": "string", "description": "CSS selector to check"}
				},
				"required": ["selector"]
			}`),
		},
		{
			Name:        "e2e_screenshot",
			Description: "Take a screenshot of the Soul UI page via headless browser. Returns a local filename that can be used as an attachment in task_comment. Always attach screenshots to verification comments so reviewers can see the actual UI.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {"type": "string", "description": "Page URL (default: http://192.168.0.128:3000)"},
					"selector": {"type": "string", "description": "CSS selector to screenshot (omit for full page)"},
					"task_id": {"type": "integer", "description": "Task ID to associate the screenshot with (for naming)"}
				}
			}`),
		},
	}
}

func (a *AgentLoop) executeE2ETool(tc toolCall) string {
	var input map[string]any
	if tc.Input != "" {
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
			return fmt.Sprintf("Error parsing input: %v", err)
		}
	}

	url := e2eDefaultURL
	if u, ok := input["url"].(string); ok && u != "" {
		url = u
	}

	toolName := strings.TrimPrefix(tc.Name, "e2e_")

	// Build args for the test runner. We pass them via a temp JSON file
	// to avoid shell quoting issues with SSH.
	runnerArgs := map[string]any{
		"action": toolName,
		"url":    url,
	}

	switch toolName {
	case "assert":
		assertions, _ := input["assertions"]
		runnerArgs["assertions"] = assertions

	case "dom":
		selector := "#root"
		if s, ok := input["selector"].(string); ok && s != "" {
			selector = s
		}
		runnerArgs["selector"] = selector

	case "check":
		selector, _ := input["selector"].(string)
		if selector == "" {
			return "Error: selector is required"
		}
		runnerArgs["selector"] = selector

	case "screenshot":
		if selector, ok := input["selector"].(string); ok && selector != "" {
			runnerArgs["selector"] = selector
		}

	default:
		return fmt.Sprintf("Error: unknown e2e tool %q", toolName)
	}

	argsJSON, _ := json.Marshal(runnerArgs)
	// Write args to a temp file, then SSH to titan-pc and run the test using that file.
	command := fmt.Sprintf(
		"echo %s | ssh titan-pc 'cat > /tmp/soul-e2e-args.json && cd ~/soul-e2e && node test-runner.js --json /tmp/soul-e2e-args.json'",
		shellescape(string(argsJSON)),
	)

	log.Printf("[e2e] executing: %s %s %s", toolName, url, string(argsJSON))

	cmd := exec.Command("bash", "-c", command)
	done := make(chan error, 1)
	var out []byte
	go func() {
		var execErr error
		out, execErr = cmd.CombinedOutput()
		done <- execErr
	}()

	select {
	case execErr := <-done:
		result := string(out)
		if len(result) > 10000 {
			result = result[:10000] + "\n... (output truncated)"
		}
		if execErr != nil {
			return fmt.Sprintf("E2E error: %v\n%s", execErr, result)
		}
		if result == "" {
			return "E2E test completed with no output."
		}

		// For screenshots, SCP the file back from titan-pc and return the local filename.
		if toolName == "screenshot" {
			localFile := a.scpScreenshot(input)
			if localFile != "" {
				result += fmt.Sprintf("\n\nScreenshot saved locally: %s\nUse this filename in task_comment attachments to show this screenshot to reviewers.", localFile)
			}
		}

		return result
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return "Error: E2E test timed out after 60 seconds"
	}
}

// scpScreenshot copies /tmp/soul-e2e-screenshot.png from titan-pc to ~/.soul/screenshots/
// and returns the filename (not full path) for use in API URLs.
func (a *AgentLoop) scpScreenshot(input map[string]any) string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[e2e] failed to get home dir: %v", err)
		return ""
	}

	screenshotDir := filepath.Join(home, ".soul", "screenshots")
	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		log.Printf("[e2e] failed to create screenshots dir: %v", err)
		return ""
	}

	// Build filename: task-{id}-{timestamp}.png
	taskID := 0
	if tid, ok := input["task_id"].(float64); ok {
		taskID = int(tid)
	}
	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("task-%d-%s.png", taskID, ts)
	if taskID == 0 {
		filename = fmt.Sprintf("screenshot-%s.png", ts)
	}

	localPath := filepath.Join(screenshotDir, filename)

	// SCP from titan-pc
	scpCmd := exec.Command("scp", "titan-pc:/tmp/soul-e2e-screenshot.png", localPath)
	if out, err := scpCmd.CombinedOutput(); err != nil {
		log.Printf("[e2e] SCP screenshot failed: %v\n%s", err, string(out))
		return ""
	}

	log.Printf("[e2e] screenshot saved: %s", localPath)
	return filename
}
