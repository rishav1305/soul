package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func infraContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul's infrastructure tools. Three products are available:

- DevOps: Analyze infrastructure configurations, deployment pipelines, and operational health.
- DBA: Analyze database schemas, query performance, and storage patterns.
- Migrate: Analyze migration plans, data transfer strategies, and version compatibility.

Use the appropriate tool prefix (devops__, dba__, migrate__) based on what the user needs. Use analyze tools to inspect targets and report tools to generate formatted output.`,
		Tools: []stream.Tool{
			{
				Name:        "devops__analyze",
				Description: "Analyze infrastructure configuration, deployment pipelines, or operational health for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., service name, config path, pipeline)"}},"required":["target"]}`),
			},
			{
				Name:        "devops__report",
				Description: "Generate a formatted report on infrastructure analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "dba__analyze",
				Description: "Analyze database schemas, query performance, or storage patterns for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., database name, table, query)"}},"required":["target"]}`),
			},
			{
				Name:        "dba__report",
				Description: "Generate a formatted report on database analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "migrate__analyze",
				Description: "Analyze migration plans, data transfer strategies, or version compatibility for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., migration name, source/dest pair)"}},"required":["target"]}`),
			},
			{
				Name:        "migrate__report",
				Description: "Generate a formatted report on migration analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
		},
	}
}
