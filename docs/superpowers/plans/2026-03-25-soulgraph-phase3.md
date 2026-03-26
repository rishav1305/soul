# SoulGraph Phase 3 — Full POC + Eval Pipeline + Notebook

*Date: 2026-03-25 | Status: Draft | Author: shuri*

## Goal

Deliver a production-quality, demoable POC of SoulGraph. T1 hiring managers will review github.com/rishav1305/soulgraph starting Mar 28. Phase 3 must make the repo "I'd hire this person" obvious in 5 minutes.

**Deadline: April 11**

## Current State (end of Phase 2)

- ✅ Supervisor + RAG Agent + Evaluator (Phase 1)
- ✅ Redis checkpoint, LiteLLM router, ToolAgent, LangFuse tracing, FastAPI, RAGAS eval, HTML reports (Phase 2)
- 91 tests passing, 81% coverage, `make ci` green
- `/query` and `/ws/query` endpoints serve the full supervisor graph

## Phase 3 Deliverables

### T1: Feedback Collection Module (`soulgraph/feedback.py`)

A persistent store for (question, answer, eval_scores) triples. Every query that passes through RAGAS evaluation automatically records a training triple.

**Why:** Fine-tuning pipeline is the differentiator. Shows the system is production-thinking, not just a demo.

```
soulgraph/feedback.py          — FeedbackStore class (SQLite-backed)
tests/test_feedback.py         — unit tests
```

API: `POST /feedback` — accept manual score corrections
`GET /feedback/export` — export JSONL for fine-tuning

FeedbackStore:
- `record(question, answer, eval_scores, session_id)` → auto-called by API after RAGAS eval
- `export_jsonl(path)` → write training data to .jsonl
- SQLite table: `feedback(id, question, answer, scores_json, session_id, ts)`

### T2: Eval Pipeline Integration (`soulgraph/pipeline.py`)

Connects the supervisor graph output → RAGAS eval → FeedbackStore → report in one call.

```
soulgraph/pipeline.py          — EvalPipeline class
tests/test_pipeline.py         — integration test with mock graph
```

EvalPipeline:
- `run(question)` → (answer, EvalReport, feedback_id)
- Handles Redis Stack → MemorySaver fallback transparently
- Used by the `/query` endpoint (replace current ad-hoc flow)

### T3: `/query` API Enhancement

Update `soulgraph/api.py` to use EvalPipeline internally. Add:
- `POST /feedback` — POST {question_id, score_override, comment} to correct a triple
- `GET /feedback/export` — return JSONL as download
- `GET /eval/report/{session_id}` — return the most recent EvalReport for a session as HTML

### T4: Jupyter Notebook (`docs/soulgraph_poc_demo.ipynb`)

10-cell, end-to-end walkthrough:
1. System overview (markdown)
2. Infrastructure startup (`make infra-up`, Docker status)
3. First query + answer
4. RAGAS evaluation scores (bar chart)
5. Feedback recording (show the stored triple)
6. Export training data (JSONL preview)
7. Router verification (show LiteLLM routing decision)
8. LangFuse trace (link to trace URL)
9. Redis checkpoint inspection
10. What's next (Phase 4 - fine-tuning)

Uses the live API via `httpx.AsyncClient`.

### T5: README Polish

Update README.md for the "5-minute HM skim":
- Add architecture diagram (ASCII or image)
- Add animated GIF or screenshot of a query in action
- Highlight the fine-tuning flywheel (unique differentiator)
- Add "Quick demo" section: `make infra-up && uvicorn soulgraph.api:app --reload && python docs/demo_query.py`

### T6: Coverage to 85%

Current: 81%. Identified gaps:
- `soulgraph/cli.py` — 0% (CLI commands untested)
- `soulgraph/tracing.py` — 55% (async tracing paths)
- `soulgraph/api.py` — 80% (WebSocket path untested)

Add tests for CLI `query` and `serve` commands.
Add async WebSocket test using `starlette.testclient`.

## Task Sequencing

```
T1 (feedback.py)          → no deps
T2 (pipeline.py)          → needs T1
T3 (api.py enhancements)  → needs T2
T4 (notebook)             → needs T3 (live API)
T5 (README polish)        → needs T4 (has screenshot)
T6 (coverage)             → parallel with T1-T3
```

T1 + T6 in parallel first, then T2, T3, T4, T5 sequentially.

## Acceptance Criteria

- [ ] `make ci` still green (91+ tests, ≥85% coverage)
- [ ] `POST /query` returns answer + eval_scores in response
- [ ] `GET /feedback/export` returns valid JSONL with ≥1 triple after a query
- [ ] Notebook runs top-to-bottom without errors
- [ ] README has architecture section + quick demo command
- [ ] All new code has unit tests

## Estimated Effort

- T1: 2h
- T2: 1h
- T3: 2h
- T4: 3h
- T5: 1h
- T6: 2h

**Total: ~11h. At 3-4h/session: 3 sessions. Fits Apr 11 comfortably.**

## Risk

- LangFuse trace URL access requires LangFuse running — notebook cell should gracefully skip if not available
- RAGAS can be slow (~5-10s/query) — notebook should use cached results for demo cells
