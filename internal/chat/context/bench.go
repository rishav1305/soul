package context

import "github.com/rishav1305/soul/internal/chat/stream"

func benchContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul Bench, an LLM benchmarking platform. Bench runs standardized evaluations against model endpoints and tracks results over time.

Key capabilities:
- Run full benchmarks across multiple categories (reasoning, coding, safety, etc.) with optional GPU acceleration.
- Run quick smoke tests to verify a model endpoint is responsive and functional.
- List and compare benchmark results to track model performance over time.

Help users set up benchmarks, interpret results, and understand model performance characteristics.`,
		Tools: []stream.Tool{
			{
				Name:        "run_benchmark",
				Description: "Run a full benchmark suite against a model endpoint. Evaluates across multiple categories.",
				InputSchema: mustJSON(`{"type":"object","properties":{"model_endpoint":{"type":"string","description":"URL of the model endpoint to benchmark"},"categories":{"type":"array","items":{"type":"string"},"description":"Categories to evaluate (e.g., reasoning, coding, safety). Defaults to all."},"gpu":{"type":"boolean","description":"Enable GPU acceleration for benchmark execution"}},"required":["model_endpoint"]}`),
			},
			{
				Name:        "run_smoke",
				Description: "Run a quick smoke test to verify a model endpoint is responsive and functional.",
				InputSchema: mustJSON(`{"type":"object","properties":{"model_endpoint":{"type":"string","description":"URL of the model endpoint to test"}},"required":["model_endpoint"]}`),
			},
			{
				Name:        "list_results",
				Description: "List all benchmark results, ordered by most recent.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "compare_results",
				Description: "Compare two benchmark results side by side.",
				InputSchema: mustJSON(`{"type":"object","properties":{"id1":{"type":"string","description":"ID of the first benchmark result"},"id2":{"type":"string","description":"ID of the second benchmark result"}},"required":["id1","id2"]}`),
			},
		},
	}
}
