package context

import "github.com/rishav1305/soul/internal/chat/stream"

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
				InputSchema: mustJSON(`{"type":"object","properties":{"jobTitle":{"type":"string","description":"Job title"},"company":{"type":"string","description":"Company name"},"pipeline":{"type":"string","description":"Pipeline type: job, freelance, contract, consulting, product-dev"},"source":{"type":"string","description":"Where the lead was found"},"sourceUrl":{"type":"string","description":"URL of the job posting"},"location":{"type":"string","description":"Job location"},"remote":{"type":"boolean","description":"Is the position remote"}},"required":["jobTitle","company","pipeline"]}`),
			},
			{
				Name:        "lead_update",
				Description: "Update fields on an existing job lead.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead to update"},"stage":{"type":"string","description":"Updated stage"},"notes":{"type":"string","description":"Updated notes"},"next_action":{"type":"string","description":"Next action to take"},"match_score":{"type":"number","description":"Match score override"}},"required":["lead_id"]}`),
			},
			{
				Name:        "lead_list",
				Description: "List job leads with optional filters.",
				InputSchema: mustJSON(`{"type":"object","properties":{"pipeline":{"type":"string","description":"Filter by pipeline type (job, freelance, contract, consulting, product-dev)"},"active_only":{"type":"boolean","description":"Only show active (non-closed) leads"}}}`),
			},
			{
				Name:        "lead_get",
				Description: "Get detailed information about a specific job lead.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "analytics",
				Description: "View job search analytics and statistics.",
				InputSchema: mustJSON(`{"type":"object","properties":{"pipeline":{"type":"string","description":"Filter analytics by pipeline type"}}}`),
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
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"},"action":{"type":"string","description":"Action to take (e.g., apply, follow_up, archive, reject)"},"notes":{"type":"string","description":"Notes about the action"}},"required":["lead_id","action"]}`),
			},
			{
				Name:        "agent_status",
				Description: "Check the status of autonomous agent runs.",
				InputSchema: mustJSON(`{"type":"object","properties":{"run_id":{"type":"integer","description":"Specific run ID to check (omit for latest)"}}}`),
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
			// AI Tools
			{
				Name:        "resume_match",
				Description: "Score your resume against a job lead's description. Returns a match score (0-100), strengths, gaps, and suggestions.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead to score against"}},"required":["lead_id"]}`),
			},
			{
				Name:        "proposal_gen",
				Description: "Generate a tailored proposal for a freelance/contract lead. Supports platform-specific formatting (upwork, freelancer, general).",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"},"platform":{"type":"string","enum":["upwork","freelancer","general"],"description":"Target platform"}},"required":["lead_id","platform"]}`),
			},
			{
				Name:        "cover_letter",
				Description: "Generate a tailored cover letter for a job lead, matching your experience to the job description keywords.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "cold_outreach",
				Description: "Draft a personalized cold outreach email for a company based on the lead's company data and job posting.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "salary_lookup",
				Description: "Estimate the market salary range for a job lead based on role, seniority, location, and company data.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "referral_finder",
				Description: "Search for LinkedIn connections at a target company who could provide a referral. This tool runs asynchronously — poll agent_status for results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "company_pitch",
				Description: "Generate a team augmentation pitch document for a target company. This tool runs asynchronously — poll agent_status for results.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead"}},"required":["lead_id"]}`),
			},
			// Batch 1 tools (scout strategy implementation)
			{
				Name:        "resume_tailor",
				Description: "Tailor your resume against a specific job lead's description and requirements. Returns the tailored resume in markdown.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the lead to tailor resume for"}},"required":["lead_id"]}`),
			},
			{
				Name:        "freelance_score",
				Description: "Score a freelance gig 0-100 on skill match, budget fit, scope clarity, client quality, and time fit.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the freelance lead to score"}},"required":["lead_id"]}`),
			},
			{
				Name:        "networking_draft",
				Description: "Generate a channel-specific networking outreach draft (LinkedIn, X, or email) for a contact.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the networking contact"},"channel":{"type":"string","description":"Channel: linkedin, x, or email"},"activity_context":{"type":"string","description":"Recent activity or context about the person"}},"required":["lead_id","channel"]}`),
			},
			{
				Name:        "weekly_networking_brief",
				Description: "Generate a weekly networking brief showing warm, dormant, and ready contacts with suggested actions.",
				InputSchema: mustJSON(`{"type":"object","properties":{}}`),
			},
			{
				Name:        "content_series_gen",
				Description: "Generate a 3-part content series (LinkedIn posts, X posts, carousel outline) from a topic and raw insights.",
				InputSchema: mustJSON(`{"type":"object","properties":{"topic":{"type":"string","description":"Content topic"},"insights":{"type":"string","description":"Raw insights or findings to build content from"}},"required":["topic","insights"]}`),
			},
			{
				Name:        "hook_writer",
				Description: "Generate 5 alternative hook lines for a LinkedIn post using proven formulas.",
				InputSchema: mustJSON(`{"type":"object","properties":{"draft":{"type":"string","description":"Draft post to generate hooks for"}},"required":["draft"]}`),
			},
			{
				Name:        "content_topic_gen",
				Description: "Suggest 3 content topics based on this week's work, mapped to content pillars.",
				InputSchema: mustJSON(`{"type":"object","properties":{"week_summary":{"type":"string","description":"Summary of what you worked on this week"}},"required":["week_summary"]}`),
			},
			{
				Name:        "expert_application",
				Description: "Generate a tailored application for an expert consulting network (GLG, Guidepoint, etc).",
				InputSchema: mustJSON(`{"type":"object","properties":{"network_name":{"type":"string","description":"Name of the expert network"},"focus_area":{"type":"string","description":"Your focus area for this network"}},"required":["network_name","focus_area"]}`),
			},
			{
				Name:        "call_prep_brief",
				Description: "Generate a prep brief for an expert consulting call — company background, likely questions, relevant experience.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the consulting lead"}},"required":["lead_id"]}`),
			},
			// Batch 2 tools — contracts
			{
				Name:        "sow_generator",
				Description: "Generate a Statement of Work for a contract lead — scope, deliverables, timeline, pricing, assumptions.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the contract lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "contract_followup",
				Description: "Generate a stage-appropriate follow-up message for a contract lead, using interaction history for context.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the contract lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "case_study_draft",
				Description: "Draft a case study from a completed contract — title, challenge, approach, results, testimonial prompt.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the contract lead"}},"required":["lead_id"]}`),
			},
			// Batch 2 tools — consulting
			{
				Name:        "consulting_followup",
				Description: "Generate a follow-up message for a consulting engagement based on stage and interaction history.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the consulting lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "advisory_proposal",
				Description: "Generate an advisory retainer proposal — executive summary, scope, deliverables, pricing model, terms.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the consulting lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "project_proposal",
				Description: "Generate a project proposal — problem statement, solution, approach, milestones, budget, timeline.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the consulting lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "consulting_upsell_evaluator",
				Description: "Evaluate upsell potential for a consulting engagement — score, opportunities, recommended approach, timing.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the consulting lead"}},"required":["lead_id"]}`),
			},
			// Batch 2 tools — content
			{
				Name:        "thread_converter",
				Description: "Convert a LinkedIn post into an X/Twitter thread with hook tweet, numbered thread, and CTA.",
				InputSchema: mustJSON(`{"type":"object","properties":{"post":{"type":"string","description":"LinkedIn post content to convert"}},"required":["post"]}`),
			},
			{
				Name:        "substack_expander",
				Description: "Expand a LinkedIn post into a long-form Substack article (1500-2000 words).",
				InputSchema: mustJSON(`{"type":"object","properties":{"post":{"type":"string","description":"LinkedIn post to expand"},"topic":{"type":"string","description":"Topic/title for the article"}},"required":["post","topic"]}`),
			},
			{
				Name:        "reactive_content_gen",
				Description: "Generate reactive content (LinkedIn + X posts) based on a news event or industry development.",
				InputSchema: mustJSON(`{"type":"object","properties":{"news_context":{"type":"string","description":"News event or development to react to"},"angle":{"type":"string","description":"Your expert angle on this news"}},"required":["news_context","angle"]}`),
			},
			{
				Name:        "engagement_reply",
				Description: "Generate a thoughtful reply to a LinkedIn or X post that adds genuine value.",
				InputSchema: mustJSON(`{"type":"object","properties":{"post_content":{"type":"string","description":"Content of the post to reply to"},"author_context":{"type":"string","description":"Context about the post author"}},"required":["post_content"]}`),
			},
			// Batch 2 tools — metrics + profile
			{
				Name:        "content_metrics",
				Description: "Aggregate content performance metrics across published posts with AI analysis and recommendations.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Filter by platform (linkedin, x) or empty for all"}}}`),
			},
			{
				Name:        "linkedin_update",
				Description: "Optimize a LinkedIn profile section for AI/ML visibility and consulting opportunities.",
				InputSchema: mustJSON(`{"type":"object","properties":{"section":{"type":"string","description":"Profile section: headline, about, or experience"},"current_content":{"type":"string","description":"Current content of the section"}},"required":["section","current_content"]}`),
			},
			{
				Name:        "github_readme_gen",
				Description: "Generate a compelling GitHub README for an AI/ML project with badges, features, architecture, and quick start.",
				InputSchema: mustJSON(`{"type":"object","properties":{"repo_name":{"type":"string","description":"Repository name"},"description":{"type":"string","description":"Brief project description"}},"required":["repo_name","description"]}`),
			},
			{
				Name:        "profile_audit",
				Description: "Audit a professional profile for completeness, SEO, and positioning. Returns score and recommendations.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Platform: linkedin or github"},"current_profile":{"type":"string","description":"Current profile content to audit"}},"required":["platform","current_profile"]}`),
			},
			{
				Name:        "testimonial_request",
				Description: "Generate a warm testimonial request message for a completed engagement.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the completed engagement lead"}},"required":["lead_id"]}`),
			},
			{
				Name:        "pin_recommendation",
				Description: "Analyze published posts and recommend which to pin/feature on your profile.",
				InputSchema: mustJSON(`{"type":"object","properties":{"platform":{"type":"string","description":"Platform: linkedin or github"}},"required":["platform"]}`),
			},
			{
				Name:        "contract_upsell_detector",
				Description: "Detect upsell opportunities in completed contracts — new services, scope expansion, referral potential.",
				InputSchema: mustJSON(`{"type":"object","properties":{"lead_id":{"type":"integer","description":"ID of the contract lead"}},"required":["lead_id"]}`),
			},
		},
	}
}
