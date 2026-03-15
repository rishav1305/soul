package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func tutorContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Tutor assistant. Tutor is an interview preparation system with 5 modules: DSA, AI/ML, Behavioral, Mock Interview, and Study Planner. It uses SM-2 spaced repetition for drill scheduling.

Available tools let you view progress, browse topics, run drills, and manage mock interviews. Use tutor_dashboard first to understand the user's current progress before suggesting actions. Use due_reviews to check what's ready for review.

Key concepts: Topics belong to modules. Drills use spaced repetition (SM-2) — questions come due based on past performance. Mock interviews have dimension scoring.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "tutor_dashboard",
				Description: "Get the tutor dashboard showing overall progress across all modules.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "list_topics",
				Description: "List topics, optionally filtered by module (dsa, ai, behavioral).",
				InputSchema: mustJSON(`{"type":"object","properties":{"module":{"type":"string","enum":["dsa","ai","behavioral"],"description":"Filter by module"}}}`),
			},
			{
				Name:        "start_drill",
				Description: "Start a spaced-repetition drill session for a topic.",
				InputSchema: mustJSON(`{"type":"object","properties":{"topic_id":{"type":"integer","description":"Topic ID to drill"}},"required":["topic_id"]}`),
			},
			{
				Name:        "answer_drill",
				Description: "Submit an answer to a drill question.",
				InputSchema: mustJSON(`{"type":"object","properties":{"question_id":{"type":"integer","description":"Question ID"},"answer":{"type":"string","description":"User's answer"}},"required":["question_id","answer"]}`),
			},
			{
				Name:        "due_reviews",
				Description: "Get topics and questions that are due for review based on SM-2 schedule.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "create_mock",
				Description: "Create a new mock interview session.",
				InputSchema: mustJSON(`{"type":"object","properties":{"type":{"type":"string","description":"Interview type (e.g., technical, behavioral, system-design)"}},"required":["type"]}`),
			},
			{
				Name:        "list_mocks",
				Description: "List all mock interview sessions with scores.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
		},
	}
}
