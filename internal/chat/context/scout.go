package context

import "github.com/rishav1305/soul-v2/internal/chat/stream"

func scoutContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul Scout, an intelligent job search and career management platform. Scout automates lead generation, profile optimization, and application tracking across multiple platforms.

Key capabilities:
- Manage job leads: add, update, list, get details, and take actions (apply, follow-up, archive).
- View analytics on job search progress and lead scoring.
- Sync and sweep job platforms for new opportunities.
- Manage and optimize professional profiles across platforms.
- Track optimization suggestions and apply approved changes.
- Monitor autonomous agent runs and their history.

Help users manage their job search efficiently, find high-quality leads, and optimize their professional presence.`,
		Tools: []stream.Tool{
			{
				Name:        "lead_add",
				Description: "Add a new job lead to the tracker.",
				InputSchema: mustJSON(`{"type":"object","properties":{"title":{"type":"string","description":"Job title"},"company":{"type":"string","description":"Company name"},"type":{"type":"string","description":"Lead type (e.g., full-time, contract, freelance)"},"source":{"type":"string","description":"Where the lead was found"},"source_url":{"type":"string","description":"URL of the job posting"}},"required":["title","company","type"]}`),
			},
			{
				Name:        "lead_update",
				Description: "Update fields on an existing job lead.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"string","description":"ID of the lead to update"},"title":{"type":"string","description":"Updated job title"},"company":{"type":"string","description":"Updated company name"},"status":{"type":"string","description":"Updated status"},"notes":{"type":"string","description":"Updated notes"}},"required":["lead_id"]}`),
			},
			{
				Name:        "lead_list",
				Description: "List job leads with optional filters.",
				InputSchema: mustJSON(`{"type":"object","properties":{"type":{"type":"string","description":"Filter by lead type"},"active_only":{"type":"boolean","description":"Only show active leads"}}}`),
			},
			{
				Name:        "lead_get",
				Description: "Get detailed information about a specific job lead.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"string","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "analytics",
				Description: "View job search analytics and statistics.",
				InputSchema: mustJSON(`{"type":"object","properties":{"filter":{"type":"string","description":"Optional filter for analytics scope"}}}`),
			},
			{
				Name:        "sync",
				Description: "Sync job data from a specific platform.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Platform to sync (e.g., linkedin, indeed, greenhouse)"}},"required":["platform"]}`),
			},
			{
				Name:        "sweep",
				Description: "Sweep multiple platforms for new job opportunities matching saved criteria.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platforms":{"type":"array","items":{"type":"string"},"description":"Platforms to sweep"}},"required":["platforms"]}`),
			},
			{
				Name:        "sweep_now",
				Description: "Trigger an immediate sweep across all configured platforms.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "sweep_status",
				Description: "Check the status of the current or most recent sweep.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "sweep_digest",
				Description: "Get a digest of results from the most recent sweep.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "profile",
				Description: "View professional profile information.",
				InputSchema: mustJSON(`{"type":"object","properties":{"section":{"type":"string","description":"Specific profile section to view (e.g., summary, experience, skills)"}}}`),
			},
			{
				Name:        "profile_pull",
				Description: "Pull the latest profile data from connected platforms.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "profile_push",
				Description: "Push local profile updates to connected platforms.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "optimization_add",
				Description: "Add a profile optimization suggestion.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Target platform for the optimization"},"section":{"type":"string","description":"Profile section to optimize"},"suggestion":{"type":"string","description":"The optimization suggestion"}},"required":["platform","section","suggestion"]}`),
			},
			{
				Name:        "optimization_list",
				Description: "List pending profile optimization suggestions.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "optimize_profile",
				Description: "Generate optimization suggestions for a platform profile.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Platform to optimize profile for"}},"required":["platform"]}`),
			},
			{
				Name:        "optimize_apply",
				Description: "Apply approved optimization changes to a platform profile.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Target platform"},"approved_changes":{"type":"array","items":{"type":"string"},"description":"List of approved change IDs to apply"}},"required":["platform","approved_changes"]}`),
			},
			{
				Name:        "lead_action",
				Description: "Take an action on a job lead (apply, follow-up, archive, reject, etc.).",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"string","description":"ID of the lead"},"action":{"type":"string","description":"Action to take (e.g., apply, follow_up, archive, reject)"},"date":{"type":"string","description":"Date for the action (ISO 8601)"},"notes":{"type":"string","description":"Notes about the action"}},"required":["lead_id","action"]}`),
			},
			{
				Name:        "agent_status",
				Description: "Check the status of autonomous agent runs.",
				InputSchema: mustJSON(`{"type":"object","properties":{"run_id":{"type":"string","description":"Specific run ID to check (omit for latest)"}}}`),
			},
			{
				Name:        "agent_history",
				Description: "View history of autonomous agent runs.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Filter by platform"}}}`),
			},
			{
				Name:        "scored_leads",
				Description: "Get leads ranked by match score.",
				InputSchema: mustJSON(`{"type":"object","properties":{"limit":{"type":"integer","description":"Maximum number of leads to return"}}}`),
			},
		},
	}
}
