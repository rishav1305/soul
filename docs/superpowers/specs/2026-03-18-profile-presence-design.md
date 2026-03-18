# Profile & Online Presence Strategy — Design Spec

**Date:** 2026-03-18
**Status:** Approved
**Scope:** `docs/scout/profile.md` — LinkedIn search optimization, portfolio app optimization, GitHub profile curation, and continuous AI sync across all three.

---

## Overview

Three-platform presence strategy using a funnel model (LinkedIn discovery → Portfolio depth → GitHub proof) with audience segmentation. LinkedIn optimized for recruiter search (role + skill + industry keywords across all sections). Portfolio optimized for elegance and conversion (case studies, testimonials, CTAs per intent). GitHub curated strategically (6 pinned repos demonstrating target skills, active contribution graph). AI continuously syncs all three platforms when work completes in any Scout pipeline.

## Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Platform model | Funnel (LinkedIn→Portfolio→GitHub) + audience segmentation | Each platform optimized for who lands there AND links deeper |
| LinkedIn search | Maximum surface area — role, skill, and industry keywords across all sections | Recruiter search behavior uses different keyword types in different sections |
| Portfolio | Already live, needs optimization for elegance and detail | Existing app, focus on content and structure optimization |
| GitHub strategy | Active profile + strategically chosen projects to fill gaps | Green squares signal activity; pinned repos signal the RIGHT skills |
| Maintenance | Continuous AI sync — new work auto-triggers profile updates across all three | Manual updates across 3 platforms never happens. Must be automated. |

---

## 7.1: The Funnel Model

```
LinkedIn (discovery) → Portfolio (depth) → GitHub (proof)
```

| Platform | Audience | They See | Conversion Goal |
|---|---|---|---|
| **LinkedIn** | Recruiters, HMs, business people | Value proposition, keywords, proof points, featured links | Click portfolio link or message you |
| **Portfolio** | Clients, founders, deep evaluators | Case studies, architecture, results, testimonials, CTAs | Contact you with specific intent |
| **GitHub** | Engineers, researchers, technical evaluators | Code, READMEs, architecture diagrams, contribution graph | "This person writes real code" |

Each platform links to the others. Visitor can enter at any point and go deeper.

---

## 7.2: LinkedIn Search Optimization

### Keyword Placement Strategy

| Section | Weight | Keywords | Purpose |
|---|---|---|---|
| **Headline** (220 chars) | Highest | Role-based: "Senior AI Engineer", "LLM", "ML", "AI Architect" + intent: "Open to opportunities" | Primary search ranking factor |
| **About** (2600 chars) | High | Industry-based in natural prose: "healthcare AI", "legal AI", "RAG pipeline", "enterprise AI", "production ML systems" | Long-tail search + conversion copy |
| **Experience** | High | Skill-based in achievements: "Built RAG pipeline serving 10K queries/day" (not "Used Python") | Context-rich keyword density |
| **Skills** (50 max) | High | All searchable tags: ML, LLM, RAG, PyTorch, LangChain, Claude, Go, Python, React, AI Strategy, etc. | Direct skill matching by recruiters |
| **Banner** | Zero (not searchable) | Your name + "AI Engineer • LLM Systems • Production AI" + portfolio URL | Visual conversion, not search |
| **Featured** | Low | Portfolio link, best post, Substack, top GitHub repo | Click-through conversion |
| **Services** | Medium | "AI/LLM Development", "AI Consulting", "AI Architecture Review", "Team Augmentation" | LinkedIn service search results |
| **Location** | Medium | "Delhi, India" + "Open to remote" + "Open to relocate" (if applicable) | Geo-targeted search |
| **Open to Work** | High | Recruiter-only mode (NOT green badge). Titles: AI Engineer, ML Engineer, AI Architect, Research Engineer. Locations: India, Remote, US, UK, Singapore. | Dramatically increases recruiter search visibility |

### Headline Formula

```
[Primary Role] | [2-3 skill keywords] | [Key differentiator] | [Intent]

Example:
"Senior AI Engineer | LLM, RAG, AI Agents | Building production AI systems | Open to AI Lab & Consulting roles"
```

### About Section Structure

```
P1: What I do + who I help (with keywords)
    "I build production AI systems — RAG pipelines, LLM agents, and
     enterprise AI architectures — for companies scaling their AI capabilities."

P2: Proof (projects, numbers, scale)
    "[X] projects delivered across legal AI, healthcare AI, sales AI,
     and e-commerce AI. [Numbers: queries processed, cost reduced, etc.]"

P3: What I'm looking for (intent keywords)
    "Currently exploring full-time AI engineering roles, contract AI
     development, and AI strategy consulting. Open to remote and
     India-based opportunities."
```

### LinkedIn SEO Rules

1. Headline has highest search weight — front-load important keywords
2. Skills section is second — add all 50, get endorsements on top 5
3. "Open to Work" (recruiter-only mode) dramatically increases visibility
4. Profile completeness affects ranking — fill EVERY section
5. Activity (posts, comments) boosts profile in search results
6. Connection count matters — accept relevant connection requests
7. Custom URL: linkedin.com/in/yourname-ai (short and keyword-rich)

---

## 7.3: Portfolio App Optimization

### Structure

| Section | Purpose | Content |
|---|---|---|
| **Hero** | Hook in 5 seconds | Name, photo, one-line value prop, 3-4 key metrics, CTA buttons ([View Work] [Hire Me] [Book a Call]), social links |
| **What I Do** | Service clarity | 3 cards: AI/LLM Development, AI Strategy & Consulting, Team & Contract AI Development. Each links to relevant case studies. |
| **Case Studies** | Depth + proof | 4-6 detailed project breakdowns: problem, approach, architecture diagram, tech stack, results with numbers, key learnings. Each linked to GitHub repo where applicable. |
| **Expertise** | Skills organized | Grouped by domain (AI/LLM, Frameworks, Backend, Frontend, Infra, Industry Verticals). Visual tags or bars, not a plain list. |
| **Testimonials** | Social proof | Client quotes from freelance + consulting. Target: 3-5 within 6 months. Even short quotes work. |
| **Writing** | Thought leadership | Latest Substack articles, best LinkedIn posts. Shows "I think deeply, not just code." Drives consulting inbound. |
| **Contact** | Clear CTAs per intent | Three paths: "Looking to hire?" (resume + LinkedIn), "Need AI development?" (book discovery call), "Need AI consulting?" (book consulting call). |

### Case Study Template

Each case study includes:
- Project title, client (name or "Enterprise client"), industry vertical, duration, your role
- **The Problem:** What the client needed, why it was hard
- **The Approach:** Architecture diagram + tech decisions + trade-offs
- **Tech Stack:** Visual tags
- **The Results:** Numbers (latency, cost reduction, accuracy, scale)
- **Key Learnings:** What surprised you, what you'd do differently
- **[View on GitHub]** link (if public repo)

Target: at least 1 case study per service type (build, consult, team), at least 2 different verticals, mix of project sizes.

### Portfolio Optimization Rules

1. Mobile-first — 60%+ visitors on phone (recruiters checking LinkedIn on mobile)
2. Load time < 2 seconds — judged in 5 seconds
3. No stock photos — architecture diagrams, code screenshots, real outputs
4. SEO title tags with keywords ("Rishav — Senior AI Engineer | AI Consulting")
5. Analytics: track which case studies get views, where visitors come from
6. Update case studies as projects complete (continuous sync from AI)

---

## 7.4: GitHub Profile Curation

### Profile README

Short landing page at `github.com/{username}/{username}`:
- Name + one-line description
- "Currently working on" (update monthly)
- "Ask me about" (RAG, LLM evaluation, AI agents, production ML, Go, Python)
- Portfolio URL + Substack URL
- Featured projects (1-liner per pinned repo)
- Tech stack badges (visual)
- Recent writing (auto-updated list of Substack posts)

### Pinned Repos (6 Strategic Slots)

| Slot | Repo Type | What It Demonstrates | Why It Matters |
|---|---|---|---|
| 1 | RAG System | Production RAG with eval metrics, chunking strategies, vector DB integration | Most in-demand AI skill |
| 2 | AI Agent / Tool-use | Multi-step agent with tool calling, error recovery, production patterns | Agentic AI is the hot topic |
| 3 | LLM Evaluation | Benchmark suite, custom metrics, comparison framework | Shows rigor, not just hacking |
| 4 | Full System | End-to-end AI application (e.g., soul-v2) | Architecture at scale |
| 5 | Analysis / Notebook | Reproducible analysis from your content (Substack backing) | Research credibility |
| 6 | Industry-Specific | AI applied to a vertical (legal, healthcare, etc.) | Consulting credibility |

### Existing Repo Audit

For each public repo, decide:
- Polished (good README, clean code)? → candidate for pinning
- Demonstrates a target skill? → keep public, consider pinning
- Half-finished / experimental? → archive (hidden from profile)
- Embarrassingly old / bad code? → make private or delete

### Strategic Repos to Build (Fill Gaps)

Assess what's missing from public profile:
- "Do I have a public RAG project?" → If no, build one
- "Do I have a public agent project?" → If no, build one
- "Do I have an LLM evaluation framework?" → If no, build one
- "Do I have a vertical AI demo?" → If no, build one

**Timeline:**
- Month 1: Audit + polish existing repos. Pin best 6.
- Month 2-3: Build 1 strategic repo (RAG or agent — highest demand)
- Month 4-6: Build 1 more (evaluation or vertical)
- Ongoing: Each content analysis with code → new repo or notebook

### Repo README Template

Every pinned repo includes:
- One-line description
- Architecture diagram (Mermaid or image)
- Key features (3-5 bullets)
- Tech stack
- Quick start (clone, install, run)
- Results / benchmarks (numbers, charts)
- Blog post link (Substack article explaining the thinking)
- License (MIT)

### Contribution Graph Strategy

Sources of public activity:
- Content analysis notebooks → commit to GitHub
- Strategic repo development → regular commits
- Open source contributions (LangChain, HuggingFace) → bonus credibility
- README/doc updates count as commits

Target: 4+ days/week with commits.
Rule: commit real work publicly. Never fake commits for green squares.

---

## 7.5: Continuous AI Sync

### Trigger Events

| Event | LinkedIn Update | Portfolio Update | GitHub Update |
|---|---|---|---|
| Freelance project completed (Gate F2) | Experience bullet + skills | New case study | Polish repo README |
| Contract delivered (Gate C3) | Experience bullet | New case study (team angle) | Pin if public repo |
| Consulting engagement done (Gate E2) | Add advisory role | Add to consulting section | Usually N/A |
| Content published (Substack) | Update Featured | Add to writing section | Profile README "Recent Writing" |
| New skill acquired | Add to Skills (50) | Update skills grid | Profile README badges |
| Testimonial received | Request as recommendation | Add to testimonials section | N/A |
| New strategic repo built | Add to projects | Link in case study | Pin + polish README |

### AI Pipeline

**Phase 1: DETECT (automated — hooks into other pipelines)**
When any pipeline reaches a completion gate (F2, C3, E2, content published):
- AI evaluates: "does this warrant a profile update?"
- If yes: generates update drafts for all 3 platforms

**Phase 2: GENERATE (automated)**
Per platform: LinkedIn (experience bullet + skills + featured), Portfolio (case study draft + skills update), GitHub (README update + pin recommendation). Stored in `lead_artifacts` type=`"profile_update"`.

**Gate PR1: PROFILE UPDATE REVIEW (as triggered)**
Scout shows: "Project X completed — 3 profile updates ready"
Actions: `[Apply]` `[Edit & Apply]` `[Skip]` `[Defer]`

**How "Apply" works per platform:**
- **LinkedIn:** Manual copy-paste (no API integration in v1). Scout shows the exact text to paste into each LinkedIn section.
- **GitHub:** Manual copy-paste for profile README. For repo READMEs, could push via git if repo is on GitHub (stretch goal, not v1).
- **Portfolio:** Automated via `profiledb` (PostgreSQL already integrated in Scout). Scout updates portfolio data directly — case studies, skills, testimonials flow through `profiledb.GetFullProfile()` / push. This is the only platform that CAN be automated in v1.

**Gate PR2: QUARTERLY PROFILE AUDIT (every 3 months)**
AI audits all three platforms:
- LinkedIn: headline freshness, skills count (target 50), Featured recency, Open to Work accuracy
- Portfolio: case study age, testimonial count (target 5), skills completeness, mobile performance
- GitHub: pinned repo activity, README freshness, contribution graph average, archive recommendations
Actions per suggestion: `[Apply]` `[Edit]` `[Dismiss]`

### AI Tools

| Tool | Input | Output | Storage |
|---|---|---|---|
| `CaseStudyDraft` | Project notes + results | Full case study (exists in contracts spec) | `lead_artifacts` type=`"case_study"` |
| `LinkedInUpdate` | Trigger event + project data | Experience bullet + skills list + featured update | `lead_artifacts` type=`"profile_update"` |
| `GitHubREADMEGen` | Repo code + purpose | Polished README with architecture diagram placeholder | `lead_artifacts` type=`"profile_update"` |
| `ProfileAudit` | User provides: LinkedIn headline + skills count + recent post impressions. Portfolio: pageviews + top case study. GitHub: pinned repos list + avg commit days/week. Format: prompted text fields in Scout UI, not free-form. | Quarterly audit with per-platform recommendations | `lead_artifacts` type=`"profile_audit"` |
| `TestimonialRequest` | Client name + project summary | Polite testimonial request message | `lead_artifacts` type=`"profile_update"` |
| `PinRecommendation` | All public repos (user-provided list) | Which 6 repos to pin and why | `lead_artifacts` type=`"profile_audit"` |

### Time Budget

| Activity | Time | When |
|---|---|---|
| Profile updates (Gate PR1) | ~15 min each | As triggered by other pipelines |
| Quarterly audit (Gate PR2) | ~1 hr | Every 3 months |
| **Weekly average** | **~0 hrs** | Triggered, not scheduled |

---

## Profile Lifecycle (Not a Pipeline)

Profile sync does NOT use `pipelines.go` stages. Like content, profiles are not leads — they are a cross-cutting concern that reacts to completion gates in other pipelines.

Profile update state is tracked via `lead_artifacts` records:
- `type="profile_update:{platform}"` — e.g., `"profile_update:linkedin"`, `"profile_update:github"`, `"profile_update:portfolio"`, `"profile_update:testimonial"`
- `type="profile_audit"` — quarterly audit recommendations
- Each artifact has implicit status based on whether the user actioned it at Gate PR1/PR2

The sub-typed `type` field avoids collision: `WHERE type LIKE 'profile_update:%'` gets all updates, `WHERE type = 'profile_update:linkedin'` gets only LinkedIn updates.

## Error Handling

- AI tool generation failure → artifact not created, surfaced in Actions tab: "Profile update failed for [platform]. Retry or skip."
- On retry failure → skip and surface at next quarterly audit
- Quarterly audit failure → surface raw platform stats for manual review
- Missing trigger data (e.g., no project notes for case study) → skip that platform, generate what's possible from available data

## Prerequisites

**Blocking:**
- Pipeline runner (`internal/scout/runner/`) — hooks into completion gates of other pipelines
- `lead_artifacts` table — stores generated updates and audits
- `ValidateTransition` enforcement in `server.go` — **CRITICAL.** Profile sync triggers fire when leads reach completion stages (F2, C3, E2). Without stage validation, bogus transitions cause false triggers.
- `CaseStudyDraft` tool — defined in contracts spec, shared by profile spec

**Non-blocking:**
- Freelance, contracts, consulting pipelines operational — profile updates trigger from their completion gates. Without them, profile updates are manual.

**Already exists:**
- Portfolio app (live)
- GitHub profile with repos
- LinkedIn profile (Premium active)
- `profiledb` in Scout (PostgreSQL portfolio data)

## Relationship to Other Specs

- **Content (Section 6):** Content drives profile visits. Content spec handles publishing and metrics. Profile spec handles converting visitors who arrive. Substack articles trigger Portfolio "Writing" section and GitHub README updates.
- **Freelance (Section 5):** Gate F2 (post-project) triggers case study + LinkedIn + GitHub updates via profile sync.
- **Contracts (Section 5):** Gate C3 (post-contract) triggers case study + LinkedIn updates. `CaseStudyDraft` tool is shared.
- **Consulting (Section 5):** Gate E2 (post-engagement) triggers LinkedIn advisory role + Portfolio consulting section updates.
- **Networking (Section 4):** LinkedIn profile optimization directly impacts networking effectiveness — better profile = higher connection acceptance rate + better first impression.
- **Jobs (Section 3):** LinkedIn search optimization directly impacts recruiter discovery. GitHub pinned repos are evaluated during technical screening.
