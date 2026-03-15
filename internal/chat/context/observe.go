package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func observeContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Observe assistant. Observe is the observability dashboard showing system health across 6 design pillars: Performant, Robust, Resilient, Secure, Sovereign, and Transparent. It aggregates metrics from all Soul products.

Available tools let you check system overview, pillar health, recent events, and active alerts. Use observe_overview for a quick health check. Use observe_pillars for detailed constraint compliance.

Key concepts: Each pillar has constraints with targets (e.g., "first-token < 200ms"). Constraints are pass/warn/fail. Events are stored as JSONL and filterable by product and type.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "observe_overview",
				Description: "Get system overview: status, cost summary, and active alerts.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product (chat, tasks, tutor, projects)"}}}`),
			},
			{
				Name:        "observe_pillars",
				Description: "Get pillar constraint health across all 6 design pillars.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product"}}}`),
			},
			{
				Name:        "observe_tail",
				Description: "Get recent events, newest first. Optionally filter by event type prefix.",
				InputSchema: mustJSON(`{"type":"object","properties":{"type":{"type":"string","description":"Event type prefix filter (e.g., 'api.request')"},"limit":{"type":"integer","description":"Max events to return (default 50, max 500)"},"product":{"type":"string","description":"Filter by product"}}}`),
			},
			{
				Name:        "observe_alerts",
				Description: "Get active alerts — threshold violations detected in metrics.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Filter by product"}}}`),
			},
		},
	}
}
