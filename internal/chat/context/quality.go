package context

import "github.com/rishav1305/soul/internal/chat/stream"

func qualityContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul's quality tools. Three products are available:

- Compliance: Scan codebases for compliance violations, auto-fix issues, generate compliance badges, and produce compliance reports.
- QA: Analyze test coverage, quality metrics, and testing patterns.
- Analytics: Analyze usage patterns, performance metrics, and system telemetry.

Use the appropriate tool prefix (compliance__, qa__, analytics__) based on what the user needs. Compliance tools have specialized schemas for scanning and fixing. Use analyze tools to inspect targets and report tools to generate formatted output.`,
		Tools: []stream.Tool{
			{
				Name:        "compliance__scan",
				Description: "Scan a directory for compliance violations across configured rules.",
				InputSchema: mustJSON(`{"type":"object","properties":{"directory":{"type":"string","description":"Directory path to scan"}},"required":["directory"]}`),
			},
			{
				Name:        "compliance__fix",
				Description: "Auto-fix compliance violations in a directory. Use dry_run to preview changes.",
				InputSchema: mustJSON(`{"type":"object","properties":{"directory":{"type":"string","description":"Directory path to fix"},"dry_run":{"type":"boolean","description":"If true, preview fixes without applying them","default":true}},"required":["directory"]}`),
			},
			{
				Name:        "compliance__badge",
				Description: "Generate a compliance badge for a directory showing pass/fail status.",
				InputSchema: mustJSON(`{"type":"object","properties":{"directory":{"type":"string","description":"Directory path to generate badge for"}},"required":["directory"]}`),
			},
			{
				Name:        "compliance__report",
				Description: "Generate a formatted compliance report for a directory.",
				InputSchema: mustJSON(`{"type":"object","properties":{"directory":{"type":"string","description":"Directory path to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["directory"]}`),
			},
			{
				Name:        "qa__analyze",
				Description: "Analyze test coverage, quality metrics, or testing patterns for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., package, module, test suite)"}},"required":["target"]}`),
			},
			{
				Name:        "qa__report",
				Description: "Generate a formatted report on QA analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "analytics__analyze",
				Description: "Analyze usage patterns, performance metrics, or system telemetry for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., endpoint, service, metric name)"}},"required":["target"]}`),
			},
			{
				Name:        "analytics__report",
				Description: "Generate a formatted report on analytics results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
		},
	}
}
