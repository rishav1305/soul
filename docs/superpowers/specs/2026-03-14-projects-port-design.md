# Projects Product — Soul v2 Port Design

## Overview

Port the Projects skill-building product from Soul v1 to v2. Tracks 11 AI/ML implementation projects with pre-written guides, project-specific milestones with acceptance criteria, resume keywords, interview readiness, and platform sync tracking. Both chat tool integration and standalone interactive UI.

## Architecture

Projects runs as a **standalone Go server process** on port 3008, with its own SQLite database at `~/.soul-v2/projects.db`. Implementation guides stored as markdown at `~/.soul-v2/projects/content/{name}/guide.md`.

```
cmd/projects/main.go                       Server entrypoint (:3008) — serve subcommand
internal/projects/
  server/server.go                         HTTP server + REST API handlers
  store/
    store.go                               SQLite CRUD — schema, migrations, queries
    seed.go                                Auto-seed 11 projects + milestones + keywords + syncs
    store_test.go                          Store unit tests
  content/                                 Embedded guide files (go:embed)
    rag-pipeline/guide.md
    fine-tuning/guide.md
    llm-evaluation/guide.md
    mlops-pipeline/guide.md
    model-serving/guide.md
    data-quality/guide.md
    agent-framework/guide.md
    knowledge-graph/guide.md
    multimodal-ai/guide.md
    streaming-ai/guide.md
    ai-safety/guide.md
web/src/
  pages/
    ProjectsPage.tsx                       4-tab page (Dashboard, Projects, Timeline, Keywords)
    ProjectDetailPage.tsx                  Single project (Milestones, Guide, Readiness, Metrics)
  hooks/
    useProjects.ts                         Dashboard/list data fetching
    useProjectDetail.ts                    Single project + guide fetching
  components/
    ProjectCard.tsx                        Project summary card
```

### Process Isolation

- **Systemd**: `soul-v2-projects.service` (same pattern as `soul-v2-tutor.service`)
- **Port**: 3008 (env: `SOUL_PROJECTS_PORT`)
- **Database**: `~/.soul-v2/projects.db` (isolated from chat.db, tasks.db, tutor.db)
- **Chat proxy**: Chat server forwards `/api/projects/*` to `SOUL_PROJECTS_URL` (default `http://127.0.0.1:3008`)

### Chat Tool Integration

Static tool registry in the chat server (same pattern as tutor tools):
- Tool definitions hardcoded in `internal/chat/server/projects_tools.go`
- When Claude calls `projects__dashboard`, chat server proxies to `POST SOUL_PROJECTS_URL/api/tools/dashboard/execute`
- If Projects server is down, tool call returns graceful error ("Projects service unavailable")

## Data Model

### Tables (7)

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `projects` | 11 AI/ML projects | id, name, description, phase (1-4), status, week_planned, hours_estimated, hours_actual, github_repo, readme_url, created_at, updated_at |
| `milestones` | Project-specific deliverables | id, project_id (FK), name, description, acceptance_criteria, status (pending/in_progress/done/skipped), completed_at, sort_order |
| `metrics` | Performance/quality measurements | id, project_id (FK), name, value, unit, captured_at |
| `keywords` | Resume keywords | id, project_id (FK nullable), keyword (UNIQUE), status (claimed/building/shipped), claimed_at, shipped_at |
| `profile_syncs` | Platform sync tracking (7 platforms: linkedin, naukri, indeed, wellfound, instahyre, portfolio, github) | id, project_id (FK), platform, synced (bool), synced_at, notes, created_at; UNIQUE(project_id, platform) |
| `interview_readiness` | Per-project self-assessment | id, project_id (FK), can_explain (bool), can_demo (bool), can_tradeoffs (bool), self_score (1-5), assessed_at |
| `daily_activity` | Aggregated daily metrics | id, date, time_spent_seconds, projects_worked, milestones_completed |

### Constraints

- Foreign keys enabled (`PRAGMA foreign_keys = ON`)
- WAL mode (`PRAGMA journal_mode = WAL`)
- UNIQUE on `projects(name)`
- UNIQUE on `keywords(keyword)`
- UNIQUE on `profile_syncs(project_id, platform)`
- Indexes on: project_id (all FK tables), status (projects, milestones, keywords), date (daily_activity)

### Status Flow

Projects: `backlog` → `active` → `measuring` → `documenting` → `shipped`

Milestones: `pending` → `in_progress` → `done` (or `skipped`)

## The 11 Projects — Milestones & Keywords

### Project 1: rag-pipeline

**Description:** Build a production-grade Retrieval-Augmented Generation pipeline with document ingestion, embedding, vector storage, and retrieval chain.

**Milestones:**
1. Document ingestion pipeline — Parse PDF/markdown into chunks with metadata. AC: Process 100+ documents without error.
2. Embedding pipeline — Generate embeddings using sentence-transformers. AC: Batch embed 1000 chunks in <60s.
3. Vector store integration — Store/query embeddings with FAISS or ChromaDB. AC: recall@10 > 0.80 on test queries.
4. Retrieval chain — Build query→retrieve→augment→generate chain. AC: End-to-end query returns relevant, grounded answer.
5. Evaluation framework — Measure retrieval quality (recall, precision, MRR). AC: Automated eval script with baseline metrics.
6. Reranking layer — Add cross-encoder reranking to improve precision. AC: precision@5 improves by >10% over baseline.

**Keywords:** RAG, Vector Database, Embeddings, FAISS, ChromaDB, Semantic Search, Document Chunking

### Project 2: fine-tuning

**Description:** Fine-tune open-source LLMs (LoRA/QLoRA) on custom datasets with evaluation and deployment.

**Milestones:**
1. Dataset preparation — Curate and format training data (instruction/response pairs). AC: 1000+ clean examples in JSONL.
2. LoRA adapter training — Fine-tune base model with LoRA on single GPU. AC: Training completes, loss decreases.
3. QLoRA 4-bit training — Quantized fine-tuning for memory efficiency. AC: Train 7B model on 16GB GPU.
4. Evaluation pipeline — Compare fine-tuned vs base on benchmark tasks. AC: Measurable improvement on target task.
5. Merge and export — Merge adapter weights, export to GGUF/HF format. AC: Merged model loads and runs inference.
6. Hyperparameter sweep — Experiment with rank, alpha, learning rate. AC: Document best config with metrics.

**Keywords:** Fine-tuning, LoRA, QLoRA, PEFT, Instruction Tuning, Model Merging

### Project 3: llm-evaluation

**Description:** Build systematic LLM evaluation framework — automated benchmarks, human eval pipelines, and quality metrics.

**Milestones:**
1. Benchmark harness — Run models against standard benchmarks (MMLU, HellaSwag). AC: Reproducible scores for 2+ models.
2. Custom eval tasks — Design domain-specific evaluation prompts. AC: 50+ eval prompts with rubrics.
3. Automated scoring — LLM-as-judge scoring pipeline. AC: Judge agreement >80% with human labels.
4. Human eval interface — Side-by-side comparison tool. AC: Rate 100 pairs, compute inter-annotator agreement.
5. Metrics dashboard — Aggregate scores by model, task, dimension. AC: Single view showing model comparison.

**Keywords:** LLM Evaluation, Benchmarking, MMLU, LLM-as-Judge, Human Evaluation

### Project 4: mlops-pipeline

**Description:** End-to-end ML pipeline with experiment tracking, model registry, CI/CD, and monitoring.

**Milestones:**
1. Experiment tracking — Set up MLflow/W&B for logging params, metrics, artifacts. AC: 10+ runs tracked with comparison view.
2. Training pipeline — Reproducible training with config-driven runs. AC: Same config → same results (seed fixed).
3. Model registry — Version, stage, and promote models. AC: Model flows through staging→production.
4. CI/CD pipeline — Automated test→train→evaluate→deploy. AC: Git push triggers full pipeline.
5. Model monitoring — Track drift, latency, error rates in production. AC: Alert fires on synthetic drift injection.
6. Data versioning — Track dataset versions alongside model versions. AC: Reproduce any past training run.

**Keywords:** MLOps, Experiment Tracking, MLflow, Model Registry, CI/CD, Data Versioning, Model Monitoring

### Project 5: model-serving

**Description:** Deploy ML models as production APIs with batching, caching, autoscaling, and observability.

**Milestones:**
1. REST API server — Serve model inference via FastAPI/Flask. AC: <100ms p95 latency on test input.
2. Request batching — Batch concurrent requests for GPU efficiency. AC: 2x+ throughput vs unbatched.
3. Response caching — Cache frequent queries (Redis/in-memory). AC: Cache hit rate >50% on repeated queries.
4. Load testing — Benchmark with concurrent users. AC: Sustain 100 RPS without degradation.
5. Container deployment — Docker image with model baked in. AC: `docker run` → serving in <30s.
6. Autoscaling config — K8s HPA or similar. AC: Scale 1→3 replicas under load, back down at idle.

**Keywords:** Model Serving, FastAPI, Inference Optimization, Batching, Docker, Kubernetes, Autoscaling

### Project 6: data-quality

**Description:** Build data quality framework — validation, profiling, anomaly detection, and automated cleaning pipelines.

**Milestones:**
1. Data profiling — Generate statistical profiles of datasets. AC: Profile report for 1M+ row dataset in <60s.
2. Validation rules — Schema + semantic validation with Great Expectations or custom. AC: 20+ rules catching known bad data.
3. Anomaly detection — Statistical outlier detection on numeric/categorical columns. AC: Flag injected anomalies with >90% recall.
4. Cleaning pipeline — Automated fix/flag/reject workflow. AC: Pipeline cleans test dataset, logs all actions.
5. Quality dashboard — Track quality metrics over time. AC: Trend view showing improvement after fixes.
6. Integration tests — Validate pipeline on real-world messy data. AC: Process 5 real datasets without crash.

**Keywords:** Data Quality, Data Validation, Great Expectations, Anomaly Detection, Data Profiling, Data Cleaning

### Project 7: agent-framework

**Description:** Build multi-agent framework with tool use, planning, memory, and orchestration.

**Milestones:**
1. Single agent loop — Tool-calling agent with ReAct pattern. AC: Agent solves 3-step task using 2+ tools.
2. Tool registry — Dynamic tool registration with schema validation. AC: Add tool at runtime, agent discovers and uses it.
3. Memory system — Short-term (conversation) + long-term (vector) memory. AC: Agent recalls facts from 10+ turns ago.
4. Multi-agent orchestration — Coordinator dispatches to specialist agents. AC: 2+ agents collaborate on a task.
5. Planning module — Agent decomposes complex task into subtasks. AC: Plan + execute 5-step task autonomously.
6. Guardrails — Token limits, tool call limits, output validation. AC: Agent gracefully stops at limits, no runaway loops.
7. Evaluation — Benchmark on agent tasks (SWE-bench lite, tool-use). AC: Documented scores with failure analysis.

**Keywords:** AI Agents, Multi-Agent Systems, ReAct, Tool Use, Agent Memory, Orchestration, Guardrails

### Project 8: knowledge-graph

**Description:** Build knowledge graph from unstructured text — entity extraction, relation mapping, graph storage, and querying.

**Milestones:**
1. Entity extraction — NER pipeline for domain entities. AC: F1 >0.80 on test corpus.
2. Relation extraction — Extract entity relationships from text. AC: Identify 5+ relation types with >0.70 precision.
3. Graph storage — Store in Neo4j or NetworkX. AC: 1000+ nodes, 5000+ edges loaded and queryable.
4. Graph querying — Natural language to graph query translation. AC: Answer 10 test questions from graph data.
5. Visualization — Interactive graph explorer. AC: Render subgraph with zoom/filter/search.
6. GraphRAG integration — Combine graph context with LLM retrieval. AC: Better answers than pure vector RAG on relationship questions.

**Keywords:** Knowledge Graph, NER, Relation Extraction, Neo4j, GraphRAG, Entity Resolution

### Project 9: multimodal-ai

**Description:** Build multimodal pipeline processing text + images — captioning, VQA, document understanding.

**Milestones:**
1. Image captioning — Generate captions from images using vision model. AC: Captions for 100 images, >70% human-rated "good".
2. Visual QA — Answer questions about image content. AC: >60% accuracy on VQA test set.
3. Document understanding — Extract structured data from document images. AC: Parse invoices/receipts with >80% field accuracy.
4. Multimodal embeddings — Joint text-image embedding space. AC: Cross-modal retrieval (text→image, image→text) works.
5. Pipeline integration — End-to-end: image in → structured output. AC: Process 50 documents, output JSON.

**Keywords:** Multimodal AI, Vision-Language Models, VQA, Document Understanding, Image Captioning, CLIP

### Project 10: streaming-ai

**Description:** Real-time AI inference with streaming — SSE, WebSocket delivery, token-by-token generation, and backpressure.

**Milestones:**
1. SSE streaming endpoint — Stream LLM tokens via Server-Sent Events. AC: First token <500ms, smooth delivery.
2. WebSocket streaming — Bidirectional streaming with cancellation. AC: Client can cancel mid-generation.
3. Backpressure handling — Slow client doesn't block server. AC: Slow consumer doesn't increase server memory.
4. Multi-model routing — Route requests to different models by task. AC: 3+ models behind single endpoint.
5. Stream processing — Transform/filter token stream (PII redaction, formatting). AC: Redact emails in real-time stream.
6. Load testing — Concurrent streaming sessions. AC: 50 concurrent streams without degradation.

**Keywords:** Streaming AI, SSE, WebSocket, Real-time Inference, Backpressure, Token Streaming

### Project 11: ai-safety

**Description:** AI safety toolkit — content filtering, prompt injection detection, output validation, and red-teaming.

**Milestones:**
1. Content classifier — Detect harmful/toxic content in inputs and outputs. AC: >90% recall on standard toxicity benchmark.
2. Prompt injection detector — Identify injection attempts. AC: Detect 80%+ of known injection patterns.
3. Output validator — Verify LLM output meets format/safety constraints. AC: Reject malformed or unsafe outputs.
4. Red-team harness — Automated adversarial testing framework. AC: Generate 100+ attack prompts, measure model robustness.
5. Guardrail pipeline — Chain: input filter → model → output filter. AC: End-to-end pipeline blocks unsafe content, passes safe.

**Keywords:** AI Safety, Content Filtering, Prompt Injection, Red Teaming, Guardrails, Output Validation

## API Design

### Projects REST API (port 3008)

**Dashboard & Lists:**
```
GET  /api/projects/dashboard              Overview: status counts, keyword coverage, hours, avg readiness, project list
GET  /api/projects/:id                    Full project: milestones, metrics, keywords, syncs, readiness
GET  /api/projects/keywords               All keywords grouped by status
```

Note: `/api/projects/dashboard` and `/api/projects/keywords` are literal path matches registered before the `/:id` wildcard in the mux, avoiding routing ambiguity. The dashboard endpoint returns the full project list (with milestone progress + keyword counts), eliminating the need for a separate `/api/projects/list` endpoint.

**Updates:**
```
PATCH /api/projects/:id                   Update project (status, hours_actual, github_repo, readme_url)
PATCH /api/projects/:id/milestones/:mid   Update milestone status (pending/in_progress/done/skipped)
POST  /api/projects/:id/metrics           Record a metric (name, value, unit)
POST  /api/projects/:id/syncs             Mark platform synced (platform, notes)
POST  /api/projects/:id/readiness         Record/update readiness assessment (upsert per project)
```

**Content:**
```
GET  /api/projects/:id/guide              Serve project's guide.md content as JSON {content: "..."}, max 1MB
```

**Chat Tool Execution + Health:**
```
POST /api/tools/:name/execute             Chat tool proxy endpoint
GET  /api/health                          Health check
```

### Chat Server Proxy

Chat server adds proxy routes:
- `internal/chat/server/proxy.go` — add `projectsProxy` + `WithProjectsProxy()` option
- `internal/chat/server/server.go` — register `/api/projects/` reverse proxy
- `cmd/chat/main.go` — wire `WithProjectsProxy()`

Frontend calls `/api/projects/*` on the chat server (port 3002), which forwards to projects server (port 3008). If projects server is down, proxy returns 502 and frontend shows graceful error.

### 5 Chat Tools (static registry)

Each tool maps 1:1 to a `POST /api/tools/{name}/execute` call on the projects server.

| Tool | Input | Action |
|------|-------|--------|
| `projects__dashboard` | `{view: "dashboard\|keywords"}` | Dashboard view returns project list + stats; keywords view returns grouped keywords |
| `projects__project_detail` | `{project_id \| project_name}` | Full project with milestones, metrics, readiness, syncs |
| `projects__update_progress` | `{project_id, status?, milestone_id?, milestone_status?, hours_actual?, github_repo?, readiness?}` | Updates are applied sequentially: project fields first, then milestone, then readiness. Each maps to its corresponding REST endpoint internally. |
| `projects__record_metric` | `{project_id, name, value, unit}` | Record measurement |
| `projects__sync_profile` | `{project_id, platform, notes?}` | Mark platform synced |

## Frontend

### Route Structure

```
/projects              → ProjectsPage (4 tabs: Dashboard, Projects, Timeline, Keywords)
/projects/:id          → ProjectDetailPage (Milestones, Guide, Readiness, Metrics)
```

All routes lazy-loaded, wrapped in AppLayout, with per-route error boundaries.

### ProjectsPage — 4 Tabs

**Dashboard:**
- Overall progress bar (shipped/total projects)
- Status cards: backlog, active, measuring, documenting, shipped counts
- Keyword coverage: shipped/total keywords
- Total hours: estimated vs actual
- Average readiness score across all projects

**Projects:**
- Project cards: name, status badge, phase, description, milestone progress bar (done/total), keyword count, hours
- Click card → `/projects/:id`

**Timeline:**
- 10-week Gantt-style grid, projects positioned by `week_planned`
- Phase grouping (Phase 1-4 rows)
- Status-colored bars

**Keywords:**
- Grouped by status: shipped (emerald), building (amber), claimed (zinc)
- Each keyword shows linked project name if any

### ProjectDetailPage

- Project header: name, status badge, phase, hours (estimated vs actual), github link
- **Milestones tab:** Milestone list with status badges, acceptance criteria text, mark done/in_progress/skipped
- **Guide tab:** Rendered markdown from guide.md
- **Readiness tab:** Self-assessment form (can_explain, can_demo, can_tradeoffs toggles + self_score 1-5 slider), platform sync checkboxes (7 platforms)
- **Metrics tab:** Recorded metrics table + form to add new metric (name, value, unit)
- Back button → `/projects`

### Hooks

**useProjects():**
- States: dashboard (includes project list for Projects + Timeline tabs), keywords, loading, error, activeTab
- Methods: refresh(), setActiveTab(tab: 'dashboard' | 'projects' | 'timeline' | 'keywords')
- Fetches dashboard on mount; fetches keywords when keywords tab selected
- Timeline tab renders from same project list data in dashboard (uses week_planned, phase, status)

**useProjectDetail(projectId: number):**
- States: project, milestones, guide (string), metrics, readiness, syncs, loading, error
- Methods: updateProject(fields), updateMilestone(milestoneId, status), recordMetric(name, value, unit), updateReadiness(assessment), syncPlatform(platform, notes?), refresh()
- Fetches project detail + guide content on mount

### Color Scheme

Dark theme (zinc palette):
- Status: backlog=zinc, active=blue, measuring=amber, documenting=purple, shipped=emerald
- Milestone status: pending=zinc, in_progress=blue, done=emerald, skipped=zinc-600

## Guide Content

### Storage

Guides embedded in the binary via `go:embed` at `internal/projects/content/`. On first run (alongside DB seed), copied to `~/.soul-v2/projects/content/{name}/guide.md`. This makes them editable post-install while ensuring they always ship with the binary.

### Guide Structure (per project)

Each guide.md follows a consistent structure:
- **Overview** — What this project builds and why it matters
- **Architecture** — System design, component diagram, data flow
- **Key Concepts** — Theory and background needed
- **Implementation Steps** — Detailed walkthrough with code snippets and library recommendations
- **Testing & Evaluation** — How to verify correctness and measure quality
- **Interview Angles** — Common interview questions about this topic, how to discuss tradeoffs

## Seed & Guide Initialization

On first server startup, if `ProjectCount() == 0`:

1. **Database seed** (atomic transaction): Insert all 11 projects with milestones, keywords (including 5 pre-shipped: "Prompt Engineering", "Multi-Agent Systems", "Agentic AI", "LLM Orchestration", "Production AI"), and 7 platform sync rows per project. Idempotent — skips if any projects exist.
2. **Guide copy**: For each project, copy embedded `internal/projects/content/{name}/guide.md` to `~/.soul-v2/projects/content/{name}/guide.md`. Skip if file already exists on disk (preserves user edits). Fall back to embedded version if disk file is missing at read time.

Triggered automatically in `cmd/projects/main.go` after DB initialization, same pattern as Tutor's auto-import.

## Pillar Compliance

### Performant
- WAL mode + indexed queries for sub-200ms responses
- Lazy-loaded routes for minimal bundle impact (verify < 300KB gzipped gate post-implementation)
- Dashboard aggregates in single SQL query (no N+1)
- Guide file reads capped at 1MB (`io.LimitReader`)
- `usePerformance` on ProjectsPage, ProjectDetailPage, ProjectCard

### Robust
- Foreign keys + UNIQUE constraints prevent invalid data states
- Input validation on all API endpoints
- Defined behavior for empty state (zero projects, zero milestones)
- Error boundaries per route

### Resilient
- Projects server down → frontend shows status, Chat/Tasks/Tutor unaffected
- Atomic transactions for multi-table writes (seed, updates)
- Chat server gracefully handles projects proxy failures

### Secure
- Parameterized SQL only
- Input sanitized at API boundary
- CSP headers on all responses

### Sovereign
- SQLite local storage, zero external dependencies
- Guides embedded in binary via go:embed
- No CDN/external assets

### Transparent
- `reportUsage`: page views (projects, project_detail), feature actions (project.update, milestone.complete, metric.record, readiness.assess, sync.platform)
- `reportError` on all failure paths
- `usePerformance` on all pages and key components
- Backend: request logging middleware (method, path, status, latency) using `pkg/events` Logger
- Backend: tool execution logged as structured events (tool name, latency, success/error)
- Backend: server start/stop events logged

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SOUL_PROJECTS_PORT` | `3008` | Projects server port |
| `SOUL_PROJECTS_HOST` | `127.0.0.1` | Projects server bind address |
| `SOUL_PROJECTS_URL` | `http://127.0.0.1:3008` | Projects URL (for chat server proxy) |
| `SOUL_V2_DATA_DIR` | `~/.soul-v2` | Data directory (projects.db lives here) |

## Build & Deploy

### Makefile Targets

- `build-projects`: `go build -o soul-projects ./cmd/projects`
- Update `build` target to include `build-projects`
- Update `clean` target to remove `soul-projects` binary
- Update `serve` target: add `./soul-projects serve &` to the `&`-chained process group
- Update `deploy` target to install `soul-v2-projects.service`

### Systemd Service

```ini
[Unit]
Description=Soul v2 — Projects Server
After=network.target

[Service]
Type=simple
User=rishav
Group=rishav
WorkingDirectory=/home/rishav/soul-v2
ExecStart=/home/rishav/soul-v2/soul-projects serve
Environment=SOUL_PROJECTS_HOST=127.0.0.1
Environment=SOUL_PROJECTS_PORT=3008
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/rishav/.soul-v2
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Files Modified in Existing Codebase

| File | Change |
|------|--------|
| `internal/chat/server/server.go` | Add `/api/projects/` reverse proxy block |
| `internal/chat/server/proxy.go` | Add `projectsProxy` + `WithProjectsProxy()` |
| `cmd/chat/main.go` | Wire `WithProjectsProxy()` |
| `internal/chat/server/projects_tools.go` | Static tool definitions for chat integration (new file) |
| `web/src/layouts/AppLayout.tsx` | Add Projects NavLink |
| `web/src/router.tsx` | Add /projects, /projects/:id routes |
| `web/src/lib/types.ts` | Add Projects type definitions |
| `Makefile` | Add build-projects, update build/clean/serve/deploy |
| `deploy/soul-v2-projects.service` | New systemd unit file |
| `CLAUDE.md` | Update architecture section |
