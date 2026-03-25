package context

import "github.com/rishav1305/soul/internal/chat/stream"

// ProductContext holds system prompt and tool definitions for a product.
type ProductContext struct {
	System string
	Tools  []stream.Tool
}

// ForProduct returns the context for a named product.
// Built-in tools (memory, custom tools, subagent) are prepended to product-specific tools.
// Returns Default() for empty or unknown product names.
func ForProduct(product string) ProductContext {
	var ctx ProductContext
	switch product {
	case "tasks":
		ctx = tasksContext()
	case "tutor":
		ctx = tutorContext()
	case "projects":
		ctx = projectsContext()
	case "observe":
		ctx = observeContext()
	case "devops", "dba", "migrate":
		ctx = infraContext()
	case "compliance", "qa", "analytics":
		ctx = qualityContext()
	case "dataeng", "costops", "viz":
		ctx = dataprodContext()
	case "docs", "api":
		ctx = docsprodContext()
	case "sentinel":
		ctx = sentinelContext()
	case "bench":
		ctx = benchContext()
	case "mesh":
		ctx = meshContext()
	case "scout":
		ctx = scoutContext()
	default:
		return Default()
	}
	// Prepend built-in tools to product-specific tools.
	ctx.System = ctx.System + "\n\n" + memorySystemPrompt
	ctx.Tools = append(builtinTools(), ctx.Tools...)
	return ctx
}

// Default returns a lightweight system prompt with built-in tools (memory, custom tools, subagent).
func Default() ProductContext {
	return ProductContext{
		System: `You are Soul, an AI development assistant built by Rishav. You are part of Soul v2 — a Go + React/TypeScript monorepo with 13 microservices, 21 products, and 93 chat tools.

Core products:
- Tasks (:3004) — autonomous task execution with dependencies, substeps, brainstorm stage, comment watcher, hooks, phases, merge gates
- Tutor (:3006) — interview prep with 5 modules (DSA, AI, Behavioral, Mock, Planner) and SM-2 spaced repetition
- Projects (:3008) — skill-building project tracking with implementation guides
- Observe (:3010) — pillar-based observability metrics (7 pillars)

Smart agents:
- Scout (:3020) — lead pipeline CRM with 5 pipeline types, browser automation, PostgreSQL profile sync
- Sentinel (:3022) — CTF challenge platform with 14 challenges across 7 attack categories
- Mesh (:3024) — distributed compute mesh with hub election, peer discovery, WebSocket transport
- Bench (:3026) — LLM benchmarking with 7 scoring methods and CARS metrics

Quality & infrastructure:
- Compliance (:3014) — 5 analyzers (secret, config, git, deps, AST), fix engine, 4 reporters, SOC2/HIPAA/GDPR rules
- QA, Analytics, DevOps, DBA, Migrate, DataEng, CostOps, Viz, Docs, API — stub products ready for implementation

Built-in tools: memories (persistent key-value), custom tools (user-defined bash templates), subagent (read-only code exploration)

The user can select a product using the tool selector to enable product-specific actions. Without a product selected, you are a general-purpose assistant with access to memory and custom tool management.

` + memorySystemPrompt,
		Tools: builtinTools(),
	}
}
