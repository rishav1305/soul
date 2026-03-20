# Tutor Interview Prep Upgrade — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Author:** Claude + Rishav

## Summary

Upgrade the Tutor interview prep system from an empty framework with word-overlap evaluation to a fully populated, Claude-evaluated, Claude Code-driven interview prep platform. Five work items: seed 130 questions, Claude semantic evaluation, System Design module, Python enrichment, and Claude Code skills (/drill, /mock).

## Context

### What Exists
- Full SM-2 spaced repetition engine (store + SM2Update algorithm)
- 11-table SQLite schema: topics, quiz_questions, spaced_repetition, progress, question_attempts, daily_activity, confidence_ratings, mock_sessions, mock_session_scores, star_stories, study_plans
- 5 modules: DSA, AI, Behavioral, Mock, Planner + Progress
- REST API: 17 endpoints on port 3006
- Importer: reads Python stdlib cheatsheet format from ~/interview-prep/

### What's Missing
1. No questions loaded — database is empty
2. Evaluation is word overlap at 50% threshold — unusable for open-ended answers
3. No System Design module
4. No Python-specific enrichment
5. Interaction through web UI only — no Claude Code integration

### Target Profile (from profiledb)
- **Python** (primary): Pandas 4.5, SQLAlchemy 4.5, Pyspark 4, Sklearn 4, Selenium 4.5
- **SQL**: SQL Server 5, PostgreSQL 5, Spark SQL 4.5, SnowSQL 4.5
- **Agentic AI**: LangGraph 4, LangChain 4, Ollama 3
- **Cloud/AWS**: AWS 4.5, Lambda 4.5, Glue 4, Batch 4.5, Redshift 4.5
- **MLOps**: Git 5, Docker 4, Kubeflow 3, DVC 3, vLLM 3
- **Role tracks**: Senior AI/ML Engineer + Lead/Staff Data Engineer

## Design

### 1. Question Banks (go:embed JSON)

**Location:** `internal/tutor/questions/`

```
internal/tutor/questions/
  dsa_python.json      # 50 questions
  ai_llm.json          # 50 questions
  system_design.json   # 30 questions
  loader.go            # go:embed + SQLite seeding
```

**Question schema:**
```json
{
  "module": "dsa",
  "category": "arrays",
  "topic": "Two Sum Variations",
  "difficulty": "medium",
  "question": "Given a sorted array, find two numbers that add to a target. What's the optimal Python approach?",
  "answer": "Use two pointers (left=0, right=len-1). Move left up if sum < target, right down if sum > target. O(n) time, O(1) space.",
  "explanation": "Two pointers exploit the sorted property. Unlike hash map approach (O(n) space), this uses O(1) space.",
  "source": "dsa_python:arrays:001"
}
```

**Note on tags/role_track:** These are NOT stored in the database. The `source` field serves as the dedup key (unique per question across JSON files). Python enrichment is achieved through question content and evaluation prompts, not per-question tag storage. The `source` field format is `{file}:{category}:{sequence}` — used by the loader for idempotent seeding.

**DSA Python distribution (50):**
- Arrays/Strings: 10 (two pointers, sliding window, prefix sums)
- Hash Maps/Sets: 6 (frequency counting, grouping, defaultdict/Counter)
- Linked Lists: 4 (cycle detection, reversal, merge)
- Trees/Graphs: 8 (BFS/DFS, trie, topological sort)
- Dynamic Programming: 8 (1D, 2D, state machine)
- Stacks/Queues: 4 (monotonic stack, deque patterns)
- Sorting/Searching: 4 (quickselect, binary search variants)
- Python-specific: 6 (generators, itertools, collections, decorators, async/await, GIL)

**AI/LLM distribution (50):**
- Transformer Architecture: 8 (attention, positional encoding, KV cache)
- RAG: 6 (chunking, embedding, retrieval, reranking)
- LLM Fundamentals: 8 (tokenization, fine-tuning, RLHF, quantization)
- Multi-Agent Systems: 6 (orchestration, LangGraph, tool use)
- MLOps: 6 (model serving, vLLM, monitoring, A/B testing)
- Classical ML: 8 (sklearn, feature engineering, ensemble methods)
- Data Engineering for ML: 4 (feature stores, data pipelines, drift detection)
- Prompt Engineering: 4 (chain-of-thought, few-shot, evaluation)

**System Design distribution (30):**
- ML System Design (15): RAG pipeline at scale, model serving infra, feature store, recommendation engine, real-time inference, ML pipeline orchestration, LLM gateway, embedding search, training infra, A/B testing platform, data quality framework, anomaly detection system, multi-model routing, vector DB architecture, fine-tuning pipeline
- Data System Design (15): data lake architecture, streaming vs batch, warehouse modeling, ETL pipeline at scale, data quality at scale, CDC pipeline, data catalog, cost optimization platform, real-time analytics, event-driven pipeline, data mesh, schema evolution, data lineage, data access layer, observability pipeline

**Loader behavior:**
- `questions.Load(store)` called on server boot
- Reads embedded JSON, creates topics via `store.CreateTopic()` (uses `ON CONFLICT DO NOTHING`)
- Creates quiz questions via `store.CreateQuizQuestion()` — **dedup by source field**: before inserting, checks if a question with the same `source` value already exists for that topic. If yes, skips. This makes the loader fully idempotent without requiring a schema migration.
- Safe to run on every boot — duplicate questions are never created

**Schema migration (minimal):**
- Add `UNIQUE` index on `quiz_questions(topic_id, source)` via `ensureMigrations()` pattern (check index existence before creating). This is the **only** schema change required.
- The `source` field already exists in the table — only the unique index is new.

### 2. Claude Semantic Evaluation

**New package:** `internal/tutor/eval/`

```go
type Evaluator struct {
    streamClient *stream.Client
}

type EvalResult struct {
    Correct   bool     `json:"correct"`
    Score     float64  `json:"score"`      // 0-100 granular
    Quality   int      `json:"quality"`    // SM-2 quality 0-5
    Feedback  string   `json:"feedback"`   // 2-3 sentence explanation
    KeyMissed []string `json:"keyMissed"`  // concepts the answer missed
    KeyHit    []string `json:"keyHit"`     // concepts correctly covered
}
```

**Evaluation flow:**
1. Module calls `evaluator.Evaluate(question, referenceAnswer, userAnswer)`
2. Evaluator builds prompt: reference answer + user answer + rubric
3. Calls Claude via `internal/chat/stream/` (shared Claude API path)
4. Claude returns structured JSON: score, quality, feedback, keyMissed, keyHit
5. Score is granular (0-100), not binary

**SM-2 quality mapping:**
| Score Range | Quality | Meaning |
|-------------|---------|---------|
| 90-100 | 5 | Perfect recall |
| 70-89 | 4 | Correct with hesitation |
| 50-69 | 3 | Barely correct, gaps |
| 30-49 | 2 | Incorrect but close |
| 1-29 | 1 | Completely wrong |
| 0 (blank/skipped) | 0 | Complete blackout — no attempt |

**System prompt:**
```
You are an expert technical interviewer evaluating a candidate's answer.
Given the reference answer and candidate's response, evaluate on:
1. Correctness of core concepts
2. Completeness (key points covered)
3. Technical accuracy of details
4. For Python questions: idiomatic Python usage

Return JSON: {correct, score, quality, feedback, keyMissed, keyHit}
```

**Fallback:** If Claude API is unavailable (network, rate limit) or returns malformed JSON, falls back to `evaluateWordOverlap` so drills don't break. Malformed response detection: if JSON parse fails or required fields are missing, use fallback.

**New endpoint:** `POST /api/tutor/evaluate`
- Input: `{question_id, answer}`
- Implementation steps:
  1. Fetch question from store (`GetQuizQuestion`)
  2. Call `evaluator.Evaluate()` (or word-overlap fallback)
  3. Create progress record via `store.RecordProgress()` — this returns a `progressID`
  4. Record attempt via `store.RecordAttempt(questionID, progressID, ...)` — the `progressID` from step 3 is required as a foreign key
  5. Update SM-2 via `store.UpsertSpacedRep()` using the quality from evaluation
  6. Update daily activity via `store.UpsertDailyActivity()`
- Returns: `EvalResult` + next review date

### 3. System Design Module

**New file:** `internal/tutor/modules/sysdesign.go`

```go
type SystemDesignModule struct {
    store     *store.Store
    evaluator *eval.Evaluator
}
```

**Tools (3):**

| Tool | Input | Purpose |
|------|-------|---------|
| `sysdesign_learn` | `{topic_id}` or `{topic, category}` | Returns structured framework: requirements → estimation → high-level → deep dive → trade-offs |
| `sysdesign_drill` | `{topic_id}` (start) or `{question_id, answer}` (answer) | Start/answer mode with Claude evaluation |
| `sysdesign_generate` | `{category, name}` | Creates new topic + content template |

**Evaluation rubric (system-design-specific):**
System design questions are open-ended. Evaluation scores on 5 dimensions:
1. Requirements gathering (did they clarify scope?)
2. Scalability reasoning (do they reason about load?)
3. Component selection (right databases, queues, caches?)
4. Trade-off analysis (do they discuss alternatives?)
5. Data flow clarity (can they trace a request end-to-end?)

Reference answers serve as rubrics (list of expected components/concepts) rather than exact answers.

**Server route registration:** System Design tools are accessed via the existing `POST /api/tools/{name}/execute` dispatch pattern. Three new cases added to `handleToolExecute` switch: `sysdesign_learn`, `sysdesign_drill`, `sysdesign_generate`. No dedicated REST endpoints needed.

### 4. Python Enrichment

Not a separate module — enrichment across question content:

1. **DSA questions demand Python-specific answers:** Not "implement X" but "implement X using Python idioms." Reference answers include actual Python code.

2. **6 dedicated Python-specific questions** in DSA bank:
   - Generator patterns and `yield` for memory-efficient processing
   - `itertools` for combinatorial problems (product, combinations, chain)
   - `collections` deep dive (defaultdict, Counter, deque, OrderedDict)
   - Decorators and context managers for clean resource handling
   - `asyncio` patterns (gather, semaphore, queue) for concurrent I/O
   - GIL, multiprocessing vs threading, when each matters

3. **AI questions reference Python tooling:** Not "explain RAG" but "implement a RAG pipeline using LangChain in Python."

4. **Python detection in evaluation:** The evaluator checks the question's module (`dsa`) and whether the question text contains Python-related keywords. When detected, the evaluation prompt includes: "The candidate should demonstrate Python-idiomatic solutions where applicable." No per-question tag storage needed — this is prompt-level enrichment.

5. **Reference answers contain Python code snippets** — the `answer` field contains actual Python, not just prose.

### 5. Claude Code Skills

#### Skill 1: `/drill`

**File:** `.claude/skills/drill/SKILL.md`

**Flow:**
1. Call `GET /api/tutor/drill/due` — get SM-2 due reviews
2. If due reviews exist, pick most overdue. If none, pick random unseen topic.
3. Call `POST /api/tutor/drill/start` with topic_id — get question
4. Present question in terminal with metadata (module, category, difficulty)
5. User types answer naturally
6. Call `POST /api/tutor/evaluate` with question_id + answer
7. Present feedback: score, key hits/misses, explanation
8. Ask: "Next question?" or "Rate confidence 1-5?"
9. If confidence given, record via API
10. Loop until user stops

**Options:**
- `/drill` — SM-2 picks next due across all modules
- `/drill dsa` — DSA only
- `/drill ai` — AI/LLM only
- `/drill sysdesign` — System Design only
- `/drill hard` — hard difficulty only
- `/drill status` — progress summary (streak, module stats, due count)

#### Skill 2: `/mock`

**File:** `.claude/skills/mock/SKILL.md`

**Flow:**
1. User provides JD (paste text or pull from Scout lead)
2. Call `POST /api/tutor/mocks` to create session
3. Optionally fetch profile from profiledb to personalize questions
4. Claude generates 5-7 targeted questions from JD + profile gaps
5. Run each question sequentially:
   - Present question with interview context
   - User answers
   - Claude evaluates with interview-style feedback
   - Record dimension scores (technical depth, communication, structured thinking)
6. At session end: overall score, dimension breakdown, top 3 improvement areas
7. Call `POST /api/tutor/mocks/{id}/answer` to save results

**Scout integration:** If user has active Scout leads, skill suggests: "You have 3 leads — drill for which one?" and pulls JD from scout.db lead description.

**Interaction style:** Conversational, not mechanical. Uses profiledb to make questions personal:
> "Let's start with a system design warm-up. You're asked to design a data quality framework for an enterprise with 50+ data sources — similar to what you built at IBM-TWC. Walk me through your approach."

### 6. Files Map

**Create:**

| File | Purpose |
|------|---------|
| `internal/tutor/questions/dsa_python.json` | 50 DSA questions |
| `internal/tutor/questions/ai_llm.json` | 50 AI/LLM questions |
| `internal/tutor/questions/system_design.json` | 30 System Design questions |
| `internal/tutor/questions/loader.go` | go:embed + SQLite seeding |
| `internal/tutor/eval/eval.go` | Claude semantic evaluator |
| `internal/tutor/eval/eval_test.go` | Evaluator tests |
| `internal/tutor/modules/sysdesign.go` | System Design module |
| `internal/tutor/modules/sysdesign_test.go` | Module tests |
| `.claude/skills/drill/SKILL.md` | Drill skill |
| `.claude/skills/mock/SKILL.md` | Mock interview skill |

**Modify:**

| File | Change |
|------|--------|
| `internal/tutor/modules/registry.go` | Add `SystemDesign` field, accept evaluator, pass to modules |
| `internal/tutor/modules/dsa.go` | Add `evaluator` field, replace `evaluateWordOverlap` with `eval.Evaluator` call |
| `internal/tutor/modules/ai.go` | Add `evaluator` field, replace `evaluateWordOverlap` with `eval.Evaluator` call |
| `internal/tutor/server/server.go` | Add `evaluator` field + `WithEvaluator` option, add `/api/tutor/evaluate` route, add `sysdesign_learn`/`sysdesign_drill`/`sysdesign_generate` cases to `handleToolExecute` |
| `internal/tutor/store/store.go` | Add `CREATE UNIQUE INDEX IF NOT EXISTS idx_quiz_questions_source ON quiz_questions(topic_id, source)` in `migrate()` |
| `cmd/tutor/main.go` | Import `pkg/auth`, create `OAuthTokenSource` from `~/.claude/.credentials.json`, init `stream.Client`, create `eval.Evaluator`, call `questions.Load(store)`, pass evaluator to server via `WithEvaluator`. If credentials missing, server still boots (evaluator is nil, modules fall back to word overlap). |

**Not touched:**
- `web/` — no frontend changes
- `internal/chat/context/` — no Chat product registration
- `specs/` — skills not spec-driven

### 7. Dependency Flow

```
cmd/tutor/main.go
  → questions.Load(store)          [seed on boot, idempotent]
  → stream.NewClient(credentials)  [Claude API client]
  → eval.New(streamClient)         [semantic evaluator]
  → server.New(
      WithStore(store),
      WithEvaluator(evaluator),    [new option]
    )

/drill skill
  → GET  /api/tutor/drill/due
  → POST /api/tutor/drill/start
  → POST /api/tutor/evaluate       [new]
  → GET  /api/tutor/dashboard

/mock skill
  → POST /api/tutor/mocks
  → POST /api/tutor/evaluate       [new]
  → POST /api/tutor/mocks/{id}/answer
  → GET  scout API (profiledb)     [optional]
```

## Non-Goals

- No web UI changes — Claude Code skills are the interaction layer
- No Chat product tool registration — can add later if needed
- One minimal schema change: unique index on `quiz_questions(topic_id, source)` for loader idempotency — no new tables
- No Go module — Python enrichment instead
- No frontend gate components — backend + skills only
