# Scout Strategy: AI Tools Inventory

**Parent doc:** `docs/scout/README.md` Section 10
**Status:** Ready

---

## Summary

**Existing tools:** 7 (already in `internal/scout/ai/`)
**New tools to build:** 28
**Total:** 35 AI tools across all sections

---

## Existing Tools (in codebase)

| Tool | File | Execution | Section |
|---|---|---|---|
| `ResumeMatch` | `ai/match.go` | Sync | Jobs |
| `CoverLetter` | `ai/cover.go` | Sync | Jobs |
| `ColdOutreach` | `ai/outreach.go` | Sync | Networking |
| `SalaryLookup` | `ai/salary.go` | Sync | Jobs |
| `ProposalGen` | `ai/proposal.go` | Sync | Freelance |
| `ReferralFinder` | `ai/referral.go` | Async | Networking |
| `CompanyPitch` | `ai/pitch.go` | Async | Contracts |

---

## New Tools by Section

### Jobs (Section 3) — 1 new tool

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `ResumeTailor` | `ai/resume.go` | Sync | Baseline resume + JD + tech slugs | Tailored resume with matched keywords |

### Networking (Section 4) — 2 new tools

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `NetworkingDraft` | `ai/networking.go` | Sync | Contact + channel + activity context | Outreach/engagement draft per channel |
| `WeeklyNetworkingBrief` | `ai/networking.go` | Sync | All contacts' warmth + interactions | Weekly summary: warm, dormant, ready |

### Freelance (Section 5) — 1 new tool

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `FreelanceScore` | `ai/freelance_score.go` | Sync | Gig description + your profile | 0-100 score on 5 criteria |

### Contracts (Section 5) — 4 new tools

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `SOWGenerator` | `ai/sow.go` | Async | Call notes + lead data | Draft SOW (scope, team, timeline, pricing) |
| `ContractFollowUp` | `ai/contract_followup.go` | Sync | Lead + stage + last message | Follow-up email for negotiation stage |
| `CaseStudyDraft` | `ai/case_study.go` | Async | Project notes + results | Case study for portfolio/Clutch |
| `ContractUpsellDetector` | `ai/upsell.go` | Sync | Freelance lead data | Upsell flag + team proposal draft |

### Consulting (Section 5) — 6 new tools

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `ExpertApplication` | `ai/expert_application.go` | Sync | Network name + focus | Tailored application text |
| `CallPrepBrief` | `ai/call_prep.go` | Sync | Lead data (company, topic) | Prep brief: background, likely Q's, your experience |
| `ConsultingFollowUp` | `ai/consulting_followup.go` | Sync | Lead + call notes | Follow-up email with discussion refs |
| `AdvisoryProposal` | `ai/advisory_proposal.go` | Sync | Lead + call notes + signals | Advisory proposal: diagnosis, scope, rate |
| `ProjectProposal` | `ai/project_proposal.go` | Sync | Lead + advisory history + scope | Project proposal: audit scope, deliverables, fee |
| `ConsultingUpsellEvaluator` | `ai/upsell_evaluator.go` | Sync | Call notes + lead context | Upsell assessment + draft proposal if yes |

### Content (Section 6) — 8 new tools

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `ContentTopicGen` | `ai/content_topic.go` | Sync | Work + backlog + news | 3 topic suggestions with pillar + angle |
| `ContentSeriesGen` | `ai/content_series.go` | Sync | Topic + raw insights | 3 LinkedIn + 3 X + carousel outline |
| `HookWriter` | `ai/hook_writer.go` | Sync | Draft post | 5 hook variations (8 formulas) |
| `ThreadConverter` | `ai/thread_converter.go` | Sync | LinkedIn deep-dive | X thread (5-10 tweets) |
| `SubstackExpander` | `ai/substack_expander.go` | Sync | Best LinkedIn post | 1500-3000 word article with SEO title |
| `EngagementReply` | `ai/engagement_reply.go` | Sync | Comment + post context | Thoughtful reply continuing conversation |
| `ContentMetrics` | `ai/content_metrics.go` | Sync | Mode + content_posts data | Weekly/monthly/quarterly analysis |
| `ReactiveContentGen` | `ai/reactive_content.go` | Sync | News/paper + expertise | Hot-take post draft |

### Profile (Section 7) — 6 new tools

| Tool | File | Execution | Input | Output |
|---|---|---|---|---|
| `LinkedInUpdate` | `ai/linkedin_update.go` | Sync | Trigger event + project | Experience bullet + skills + featured |
| `GitHubREADMEGen` | `ai/github_readme.go` | Sync | Repo code + purpose | Polished README with architecture |
| `ProfileAudit` | `ai/profile_audit.go` | Sync | 3 platform states (user-provided) | Quarterly audit + recommendations |
| `TestimonialRequest` | `ai/testimonial_request.go` | Sync | Client name + project | Polite testimonial request message |
| `PinRecommendation` | `ai/pin_recommendation.go` | Sync | Public repos list | Which 6 to pin and why |
| `CaseStudyDraft` | *(shared from Contracts)* | | | |

---

## Implementation Priority

### P0 — Build first (enable core pipelines)

| Tool | Why First |
|---|---|
| `ResumeTailor` | Gate 1 needs tailored resumes |
| `FreelanceScore` | Gate F1 needs scored gigs |
| `NetworkingDraft` | Gate N1 needs outreach drafts |
| `ContentSeriesGen` | Gate P1 needs content batch |
| `HookWriter` | Every content piece needs hooks |

### P1 — Build second (enable advanced pipelines)

| Tool | Why |
|---|---|
| `ExpertApplication` | Consulting onboarding (apply to 12 networks) |
| `CallPrepBrief` | Before first expert call |
| `SOWGenerator` | Before first contract negotiation |
| `WeeklyNetworkingBrief` | Friday review needs this |
| `ContentTopicGen` | Sunday planning needs this |
| `ThreadConverter` | X engagement needs this |

### P2 — Build third (optimization + scaling)

All remaining tools: `ContractFollowUp`, `ConsultingFollowUp`, `AdvisoryProposal`, `ProjectProposal`, `ConsultingUpsellEvaluator`, `ContractUpsellDetector`, `SubstackExpander`, `EngagementReply`, `ContentMetrics`, `ReactiveContentGen`, `LinkedInUpdate`, `GitHubREADMEGen`, `ProfileAudit`, `TestimonialRequest`, `PinRecommendation`, `CaseStudyDraft`

---

## Infrastructure Required (before any tools)

| Component | Spec Source | What |
|---|---|---|
| Pipeline runner | Job application spec | `internal/scout/runner/` — polling loop, shared by all sections |
| `lead_artifacts` table | Job application spec | Stores all AI-generated artifacts |
| `ValidateTransition` fix | Job application spec | Stage integrity — CRITICAL |
| Schema migration | Networking spec | `tier`, `contact_type`, `intent`, `warmth`, `interaction_count`, `channels`, `last_interaction_at`, `source_ref_id` |
| `interactions` table | Networking spec | Interaction tracking for warmth scoring |
| `content_posts` table | Content spec | Per-post metrics tracking |
| `content_backlog` table | Content spec | Evergreen topic queue |
| New pipelines in `pipelines.go` | Various | `referral`, `networking` + updated `job` and `freelance` stages |
| `knownPipelines` update | Networking spec | Add `"networking"`, `"referral"` to analytics |
