package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func docsprodContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul's documentation tools. Two products are available:

- Docs: Analyze documentation coverage, quality, freshness, and consistency across the codebase.
- API: Analyze API specifications, endpoint coverage, schema validation, and versioning.

Use the appropriate tool prefix (docs__, api__) based on what the user needs. Use analyze tools to inspect targets and report tools to generate formatted output.`,
		Tools: []stream.Tool{
			{
				Name:        "docs__analyze",
				Description: "Analyze documentation coverage, quality, or freshness for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., package, module, directory)"}},"required":["target"]}`),
			},
			{
				Name:        "docs__report",
				Description: "Generate a formatted report on documentation analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "api__analyze",
				Description: "Analyze API specifications, endpoint coverage, or schema validation for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., API spec path, endpoint group, version)"}},"required":["target"]}`),
			},
			{
				Name:        "api__report",
				Description: "Generate a formatted report on API analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
		},
	}
}
