package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func projectsContext() ProductContext {
	return ProductContext{
		System: `You are Soul's Projects assistant. Projects is a skill-building project tracker with milestones, metrics, readiness assessments, and implementation guides. Each project targets specific technical skills.

Available tools let you view dashboards, get project details, update progress, record metrics, and access implementation guides. Use projects_dashboard first to see all projects before diving into specifics.

Key concepts: Projects have milestones (deliverables), metrics (quantitative measurements), readiness scores (can-explain, can-demo, can-tradeoffs), and embedded implementation guides.

The user may reference other Soul products. If the question is about a different product, suggest they switch using the tool button.`,
		Tools: []stream.Tool{
			{
				Name:        "projects_dashboard",
				Description: "Get dashboard showing all projects with status summaries.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "get_project",
				Description: "Get full project detail including milestones, metrics, readiness, and sync history.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"}},"required":["project_id"]}`),
			},
			{
				Name:        "update_project",
				Description: "Update a project's status or hours.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"status":{"type":"string","enum":["not_started","in_progress","completed"]},"hours_actual":{"type":"number"}},"required":["project_id"]}`),
			},
			{
				Name:        "update_milestone",
				Description: "Update a milestone's status.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"milestone_id":{"type":"integer","description":"Milestone ID"},"status":{"type":"string","enum":["not_started","in_progress","done"]}},"required":["project_id","milestone_id","status"]}`),
			},
			{
				Name:        "record_metric",
				Description: "Record a metric value for a project.",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"},"name":{"type":"string","description":"Metric name"},"value":{"type":"number","description":"Metric value"},"unit":{"type":"string","description":"Unit of measurement"}},"required":["project_id","name","value"]}`),
			},
			{
				Name:        "get_guide",
				Description: "Get the implementation guide for a project (markdown).",
				InputSchema: mustJSON(`{"type":"object","properties":{"project_id":{"type":"integer","description":"Project ID"}},"required":["project_id"]}`),
			},
		},
	}
}
