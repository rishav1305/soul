package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

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
		System: `You are Soul, an AI development assistant. You are part of Soul v2 — a platform with 4 products: Tasks (autonomous task execution), Tutor (interview prep with spaced repetition), Projects (skill-building project tracking), and Observe (observability metrics dashboard). The user can select a product using the tool button to enable product-specific actions. Without a product selected, you are a general-purpose assistant.

` + memorySystemPrompt,
		Tools: builtinTools(),
	}
}
