package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

// ProductContext holds system prompt and tool definitions for a product.
type ProductContext struct {
	System string
	Tools  []stream.Tool
}

// ForProduct returns the context for a named product.
// Returns Default() for empty or unknown product names.
func ForProduct(product string) ProductContext {
	switch product {
	case "tasks":
		return tasksContext()
	case "tutor":
		return tutorContext()
	case "projects":
		return projectsContext()
	case "observe":
		return observeContext()
	default:
		return Default()
	}
}

// Default returns a lightweight system prompt with no tools.
func Default() ProductContext {
	return ProductContext{
		System: `You are Soul, an AI development assistant. You are part of Soul v2 — a platform with 4 products: Tasks (autonomous task execution), Tutor (interview prep with spaced repetition), Projects (skill-building project tracking), and Observe (observability metrics dashboard). The user can select a product using the tool button to enable product-specific actions. Without a product selected, you are a general-purpose assistant.`,
	}
}
