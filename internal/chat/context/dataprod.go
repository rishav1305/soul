package context

import "github.com/rishav1305/soul/internal/chat/stream"

func dataprodContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul's data tools. Three products are available:

- DataEng: Analyze data pipelines, ETL processes, schema evolution, and data quality.
- CostOps: Analyze infrastructure costs, resource utilization, and optimization opportunities.
- Viz: Analyze data visualizations, dashboard configurations, and charting patterns.

Use the appropriate tool prefix (dataeng__, costops__, viz__) based on what the user needs. Use analyze tools to inspect targets and report tools to generate formatted output.`,
		Tools: []stream.Tool{
			{
				Name:        "dataeng__analyze",
				Description: "Analyze data pipelines, ETL processes, or data quality for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., pipeline name, data source, schema)"}},"required":["target"]}`),
			},
			{
				Name:        "dataeng__report",
				Description: "Generate a formatted report on data engineering analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "costops__analyze",
				Description: "Analyze infrastructure costs, resource utilization, or optimization opportunities for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., service, resource group, billing account)"}},"required":["target"]}`),
			},
			{
				Name:        "costops__report",
				Description: "Generate a formatted report on cost analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
			{
				Name:        "viz__analyze",
				Description: "Analyze data visualizations, dashboard configurations, or charting patterns for a target.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to analyze (e.g., dashboard name, chart type, data source)"}},"required":["target"]}`),
			},
			{
				Name:        "viz__report",
				Description: "Generate a formatted report on visualization analysis results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"target":{"type":"string","description":"Target to report on"},"format":{"type":"string","enum":["terminal","json","html"],"description":"Output format"}},"required":["target"]}`),
			},
		},
	}
}
