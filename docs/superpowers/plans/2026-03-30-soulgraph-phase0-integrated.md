# SoulGraph Phase 0: ChromaDB Shared Memory — Integrated Plan

**Author:** Shuri | **Status:** FINAL
**Start:** Mar 31, 2026 | **Complete by:** Apr 2, 2026 (3 days)
**Reviewed by:** Pepper (within 24h)

---

## 1. Objective

Replace file-based agent memory (~109 files) and shared knowledge base (~28 files) with searchable ChromaDB vector collections. File-based system remains as primary; ChromaDB is additive and read-through. Zero production risk — fully reversible.

## 2. Architecture

```
                    +-----------------------+
                    |    Agent Session       |
                    |  (Claude Code CLI)     |
                    +-----------+-----------+
                                |
                    +-----------v-----------+
                    |  Memory Query Layer    |
                    |  (curl → FastAPI)      |
                    +-----------+-----------+
                                |
                +---------------+---------------+
                |                               |
    +-----------v-----------+   +-----------v-----------+
    |   ChromaDB (primary   |   |  File System (fallback |
    |   search path)        |   |  + source of truth)    |
    |                       |   |                        |
    |  soul_agent_memory    |   |  ~/.claude/agent-memory/|
    |  soul_shared_kb       |   |  ~/soul-roles/shared/   |
    |  soul_briefs          |   |                        |
    +-----------------------+   +------------------------+
```

**Key principle:** Files are ALWAYS source of truth. ChromaDB is a semantic search index. If ChromaDB is down, agents fall back to existing file reads with zero impact.

## 3. Work Streams (3 parallel tracks, 3 days)

### Stream A: ChromaDB Infrastructure (Banner)
**Scope:** Collections, embeddings, indexing pipeline
**No dependencies on Streams B or C**

| Day | Task | Deliverable |
|-----|------|-------------|
| Day 1 (Mar 31) | Create 3 ChromaDB collections with schema | Collections queryable via API |
| Day 1 (Mar 31) | Write bulk indexer script (`scripts/index_memories.py`) | Script indexes all 109 agent memory files + 28 KB files |
| Day 2 (Apr 1) | Run bulk index, validate counts and metadata | 137+ docs indexed, metadata correct |
| Day 2 (Apr 1) | Write incremental indexer (for new/changed files) | `scripts/index_incremental.py` — diffing on file mtime |
| Day 2 (Apr 1) | Set up systemd timer for incremental indexer every 30min | `soulgraph-reindex.timer` — no manual reindexing needed |
| Day 3 (Apr 2) | Stress test: latency benchmarks, large query volumes | Benchmark report: p50/p95/p99 latency |

**Collection Schema:**

```python
# soul_agent_memory (109 docs)
{
    "id": "{agent}/{filename}",           # e.g., "shuri/project_soulgraph.md"
    "document": "<markdown body>",         # Content without frontmatter
    "metadata": {
        "agent": "shuri",
        "type": "project",                 # user|feedback|project|reference
        "name": "SoulGraph Migration",
        "description": "CEO committed...",
        "file_path": "/home/rishav/.claude/agent-memory/shuri/project_soulgraph.md",
        "updated_at": "2026-03-30T16:00:00"
    }
}

# soul_shared_kb (28 docs)
{
    "id": "{category}/{filename}",
    "document": "<article content>",
    "metadata": {
        "category": "strategy",            # strategy|operations|products|marketing|revenue|playbooks
        "author": "fury",
        "file_path": "/home/rishav/soul-roles/shared/knowledge-base/strategy/...",
        "updated_at": "2026-03-30T16:00:00"
    }
}

# soul_briefs (stretch — index only high-value briefs)
{
    "id": "briefs/{filename}",
    "document": "<brief content>",
    "metadata": {
        "from": "shuri",
        "type": "spec",
        "file_path": "...",
        "created_at": "2026-03-30T16:00:00"
    }
}
```

**Embedding model:** `all-MiniLM-L6-v2` (ChromaDB default). Fast, local, no API dependency. Upgrade path to OpenAI embeddings if recall is poor.

### Stream B: Query API (Shuri)
**Scope:** FastAPI endpoints, SoulGraph integration, agent curl integration
**Depends on:** Stream A Day 1 (collections must exist)

| Day | Task | Deliverable |
|-----|------|-------------|
| Day 1 (Mar 31) | Add memory query endpoints to SoulGraph API (`soulgraph/api.py`) | `POST /memory/query`, `POST /memory/upsert`, `GET /memory/health` |
| Day 2 (Apr 1) | Agent integration: add `memory_search` skill to Xavier + Hawkeye (worst context overflow — pilot agents) | Pilot agents query ChromaDB on session start |
| Day 2 (Apr 1) | Write integration tests | Tests for query, upsert, fallback |
| Day 3 (Apr 2) | Add Shuri + Pepper as Day 3 agents. Rollback testing: stop ChromaDB, verify all agents work without it | 4 agents integrated, documented fallback verification |

**API Endpoints:**

```
POST /memory/query
  Body: {collection: str, query: str, agent_filter?: str, type_filter?: str, top_k: int = 5}
  Response: [{id, content, metadata, score}]

POST /memory/upsert
  Body: {collection: str, id: str, content: str, metadata: dict}
  Response: {status: "ok", id: str}

GET /memory/health
  Response: {chromadb: "up"|"down", collections: {name: doc_count}, last_index: timestamp}
```

### Stream C: Frontend & Monitoring (Happy)
**Scope:** Search UI, collection dashboard, health monitoring
**Depends on:** Stream B Day 1 (API endpoints must exist)

| Day | Task | Deliverable |
|-----|------|-------------|
| Day 1 (Mar 31) | Design memory search component (read-only, for any agent's context) | Mockup/component skeleton |
| Day 2 (Apr 1) | Build search UI: query input, results display, agent filter | Working component wired to `/memory/query` |
| Day 2 (Apr 1) | Collection health dashboard: doc counts, last index time, ChromaDB status | Dashboard widget in Observe page |
| Day 3 (Apr 2) | E2E test: search from frontend, verify results match file content | Passing E2E test |

## 4. Integration Points

### Where agents currently hit file-based memory:

| Operation | Current | Phase 0 Addition |
|---|---|---|
| Session start: read MEMORY.md | File read | + Query ChromaDB for contextually relevant memories |
| Memory write (Write tool) | File write | + Upsert to ChromaDB (async, non-blocking) |
| Knowledge query | File read / Grep | + Semantic search across all agents |
| Cross-agent context | Read other agent's files | + Query by agent_filter (no file path needed) |

### Agent integration approach:
Agents add a `curl` call to ChromaDB query API during session start. This is a prompt/skill change, NOT a code change:

```bash
# Added to agent session-start routine:
curl -s http://localhost:9080/memory/query \
  -H "Content-Type: application/json" \
  -d '{"collection":"soul_agent_memory","query":"current sprint priorities","agent_filter":"shuri","top_k":3}'
```

If the query fails (ChromaDB down, network error), the agent proceeds with file-based memory as before. Zero impact.

## 5. Rollback Procedure

ChromaDB is **additive only**. File-based system is untouched throughout:

1. **ChromaDB down:** Agents fall back to file reads. No code/prompt change needed — the curl call simply fails silently.
2. **Bad results:** File-based MEMORY.md remains authoritative. ChromaDB supplements, never replaces.
3. **Full rollback:** Stop ChromaDB container. Remove curl calls from agent prompts. Total time: <5 minutes. Zero data loss.

**Key constraint:** Files are NEVER deleted when indexing to ChromaDB. Files are source of truth. ChromaDB is a read-through search index.

## 6. Success Criteria (Phase 0 Gate)

| # | Criterion | Measurement | Owner |
|---|-----------|-------------|-------|
| 1 | All agent memories indexed | 109+ docs in `soul_agent_memory` | Banner |
| 2 | All KB articles indexed | 28+ docs in `soul_shared_kb` | Banner |
| 3 | Cross-agent query works | "SoulGraph migration" returns Shuri + Pepper + Fury memories | Shuri |
| 4 | Query latency <500ms | p95 measured on titan-pc | Banner |
| 5 | API endpoints functional | /memory/query, /memory/upsert, /memory/health all 200 | Shuri |
| 6 | Graceful fallback | Agents work unchanged when ChromaDB stopped | Shuri |
| 7 | 4 agents use shared memory | Xavier + Hawkeye (Day 2 pilots), Shuri + Pepper (Day 3 additions) | Shuri |
| 8 | Frontend search works | Search component returns results matching file content | Happy |
| 9 | No file-based regression | All existing agent workflows unchanged | All |
| 10 | Auto-reindex every 30min | systemd timer running, incremental indexer verified | Banner |
| 11 | Guardian monitors /memory/health | Alert on doc count drop or last_index >1hr stale | Shuri |

## 7. Daily Sync Schedule

| Time | What |
|------|------|
| 10:00 IST | 5-min async standup (clawteam messages): blockers, progress, plan for today |
| 16:00 IST | Progress report to Pepper: % complete, any timeline risks |
| EOD | Git commits pushed, status updated |

## 8. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| ChromaDB embedding quality poor | Low | Medium | Default all-MiniLM-L6-v2 is well-tested; upgrade to OpenAI embeddings if recall <70% |
| Large files (>8K tokens) | Medium | Low | Chunk on paragraph boundaries, preserve metadata per chunk |
| sshfs latency on file reads during indexing | Medium | Low | Banner runs indexer on titan-pc (ChromaDB is local); only sshfs for reading source files |
| Stale index after file edits | Medium | Low | systemd timer runs incremental indexer every 30min. Phase 1: Redis pub/sub triggers instant reindex |
| Context window bloat | Medium | Medium | Limit to top 3 results, truncate to 500 tokens each |
| CLI wrapper PoC blocks Phase 1 start | Low | High | Phase 0 is fully independent of CLI wrapper work |

## 9. Dependencies

| Dependency | Status | Owner |
|-----------|--------|-------|
| ChromaDB Docker container | Running (port 8001) | Existing |
| Redis Docker container | Running (port 6379) | Existing |
| SoulGraph Python env | Set up, 91% coverage | Existing |
| Agent memory files readable | Via sshfs mount | Existing |
| titan-pc compute available | Available | Existing |

**Zero new infrastructure required.** All dependencies already exist.

## 10. What This Enables (Phase 1+)

- **Phase 1 (Apr 3-7):** Redis messaging replaces file-based courier. ChromaDB auto-reindex on Redis pub/sub events.
- **Phase 2 (Apr 8-20):** AgentNode CLI wrapper. Agents query ChromaDB for context before CLI launch. Results feed into LangGraph state.
- **Long-term:** Semantic memory search across all agents. Knowledge graph. Automatic context injection. No more "read MEMORY.md" — agents get relevant context automatically.

---

*FINAL — Submitted Mar 30, 2026. Execution begins Mar 31.*
