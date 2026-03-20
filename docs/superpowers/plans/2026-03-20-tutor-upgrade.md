# Tutor Interview Prep Upgrade — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Tutor from empty framework to a fully populated, Claude-evaluated, Claude Code-driven interview prep system with 130 questions across 3 modules.

**Architecture:** go:embed JSON question banks loaded on boot → Claude semantic evaluation via `internal/chat/stream` replacing word overlap → System Design module mirroring DSA/AI pattern → two Claude Code skills (`/drill`, `/mock`) as the primary interaction layer.

**Tech Stack:** Go 1.24, SQLite (modernc.org/sqlite), Claude API via `internal/chat/stream`, go:embed, Claude Code skills

**Spec:** `docs/superpowers/specs/2026-03-20-tutor-upgrade-design.md`

---

## File Structure

**Create:**
| File | Responsibility |
|------|---------------|
| `internal/tutor/questions/dsa_python.json` | 50 DSA Python questions (go:embed) |
| `internal/tutor/questions/ai_llm.json` | 50 AI/LLM questions (go:embed) |
| `internal/tutor/questions/system_design.json` | 30 System Design questions (go:embed) |
| `internal/tutor/questions/loader.go` | Embed JSON files + idempotent SQLite seeding |
| `internal/tutor/questions/loader_test.go` | Loader tests |
| `internal/tutor/eval/eval.go` | Claude semantic evaluator with word-overlap fallback |
| `internal/tutor/eval/eval_test.go` | Evaluator tests with mock sender |
| `internal/tutor/modules/sysdesign.go` | System Design module (learn, drill, generate) |
| `internal/tutor/modules/sysdesign_test.go` | System Design module tests |
| `.claude/skills/drill/SKILL.md` | `/drill` skill definition |
| `.claude/skills/mock/SKILL.md` | `/mock` skill definition |

**Modify:**
| File | Change |
|------|--------|
| `internal/tutor/store/store.go` | Add unique index on `quiz_questions(topic_id, source)` |
| `internal/tutor/modules/registry.go` | Add `SystemDesign` + `Evaluator` fields |
| `internal/tutor/modules/dsa.go` | Add `evaluator` field, use it in `evaluateAnswer` |
| `internal/tutor/modules/ai.go` | Add `evaluator` field, use it in `evaluateAnswer` |
| `internal/tutor/server/server.go` | Add `WithEvaluator`, `/api/tutor/evaluate` route, sysdesign tool cases |
| `cmd/tutor/main.go` | Init auth + stream + evaluator + question loader |

---

## Agent Mandate (include in every subagent prompt)

Go: standard lib preferred, Claude via internal/chat/stream/, parameterized SQL (?), no secrets, error returns not panics, race-safe.
Testing: unit test every public fn, property tests for parsers, integration tests for endpoints, `go test -race`.
Frontend: not applicable for this plan.
Security: no innerHTML, no direct anthropic import, no hardcoded secrets, no unprotected endpoints, no SQL concat.
Commits: prefix (feat/fix/test), one logical change, make verify-static before commit.
Verification: `go vet ./internal/tutor/...` + `go test -race ./internal/tutor/...` before every commit.

---

## Task 1: Schema Migration — Unique Index on quiz_questions

**Files:**
- Modify: `internal/tutor/store/store.go:179-310`

- [ ] **Step 1: Write failing test**

Create `internal/tutor/store/store_dedup_test.go`:
```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuizQuestionSourceDedup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Create a topic.
	topic, err := s.CreateTopic("dsa", "arrays", "two-sum", "medium", "")
	if err != nil {
		t.Fatal(err)
	}

	// Insert a question with source.
	q1, err := s.CreateQuizQuestion(topic.ID, "medium", "What is two sum?", "Use hash map", "O(n)", "dsa_python:arrays:001")
	if err != nil {
		t.Fatal(err)
	}

	// Insert duplicate source — should not create a new row.
	q2, err := s.CreateQuizQuestion(topic.ID, "medium", "What is two sum?", "Use hash map", "O(n)", "dsa_python:arrays:001")
	if err != nil {
		t.Fatal(err)
	}

	if q1.ID != q2.ID {
		t.Errorf("expected dedup: q1.ID=%d q2.ID=%d", q1.ID, q2.ID)
	}

	// Different source should create a new row.
	q3, err := s.CreateQuizQuestion(topic.ID, "hard", "Three sum?", "Sort + two pointers", "O(n^2)", "dsa_python:arrays:002")
	if err != nil {
		t.Fatal(err)
	}
	if q3.ID == q1.ID {
		t.Error("expected different question for different source")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -run TestQuizQuestionSourceDedup ./internal/tutor/store/`
Expected: FAIL — `CreateQuizQuestion` does plain INSERT, no dedup logic.

- [ ] **Step 3: Add unique index to migrate()**

In `internal/tutor/store/store.go`, add to the end of the `schema` string in `migrate()`, after the existing index declarations:

```go
CREATE UNIQUE INDEX IF NOT EXISTS idx_quiz_questions_source_dedup ON quiz_questions(topic_id, source) WHERE source != '';
```

- [ ] **Step 4: Update CreateQuizQuestion to handle conflict**

In `internal/tutor/store/store.go`, replace the `CreateQuizQuestion` method:

```go
func (s *Store) CreateQuizQuestion(topicID int64, difficulty, questionText, answerText, explanation, source string) (*QuizQuestion, error) {
	// If source is non-empty, check for existing question with same source for this topic.
	if source != "" {
		var existingID int64
		err := s.db.QueryRow(
			"SELECT id FROM quiz_questions WHERE topic_id = ? AND source = ?",
			topicID, source,
		).Scan(&existingID)
		if err == nil {
			return s.GetQuizQuestion(existingID)
		}
	}

	res, err := s.db.Exec(
		"INSERT INTO quiz_questions (topic_id, difficulty, question_text, answer_text, explanation, source) VALUES (?, ?, ?, ?, ?, ?)",
		topicID, difficulty, questionText, answerText, explanation, source,
	)
	if err != nil {
		return nil, fmt.Errorf("tutor: create quiz question: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetQuizQuestion(id)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -race -run TestQuizQuestionSourceDedup ./internal/tutor/store/`
Expected: PASS

- [ ] **Step 6: Run full store tests**

Run: `go test -race ./internal/tutor/store/`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add internal/tutor/store/store.go internal/tutor/store/store_dedup_test.go
git commit -m "feat(tutor): add quiz question dedup by source field"
```

---

## Task 2: Question Loader (go:embed)

**Files:**
- Create: `internal/tutor/questions/loader.go`
- Create: `internal/tutor/questions/loader_test.go`
- Create: `internal/tutor/questions/dsa_python.json` (start with 5 sample questions, full 50 later)
- Create: `internal/tutor/questions/ai_llm.json` (start with 5 sample)
- Create: `internal/tutor/questions/system_design.json` (start with 5 sample)

- [ ] **Step 1: Create sample JSON files**

Create `internal/tutor/questions/dsa_python.json` with 5 seed questions:
```json
[
  {
    "module": "dsa",
    "category": "arrays",
    "topic": "Two Pointers",
    "difficulty": "medium",
    "question": "Given a sorted array of integers and a target sum, find two numbers that add up to the target. What is the optimal Python approach and its complexity?",
    "answer": "Use two pointers: left=0, right=len(arr)-1. If arr[left]+arr[right] == target, return. If sum < target, move left right. If sum > target, move right left. O(n) time, O(1) space. Python: `while l < r:` with `arr[l] + arr[r]` comparison.",
    "explanation": "Two pointers exploit the sorted property to avoid O(n^2) brute force or O(n) space hash map. This is the standard pattern for sorted array pair-sum problems.",
    "source": "dsa_python:arrays:001"
  },
  {
    "module": "dsa",
    "category": "arrays",
    "topic": "Sliding Window",
    "difficulty": "medium",
    "question": "Find the maximum sum subarray of size k in a given array. Implement in Python using the sliding window technique. What is the time and space complexity?",
    "answer": "Initialize window_sum = sum(arr[:k]). Slide: for i in range(k, len(arr)): window_sum += arr[i] - arr[i-k]; max_sum = max(max_sum, window_sum). O(n) time, O(1) space.",
    "explanation": "Sliding window avoids recalculating the entire sum each time. Adding the new element and removing the old one is O(1) per step.",
    "source": "dsa_python:arrays:002"
  },
  {
    "module": "dsa",
    "category": "hash_maps",
    "topic": "Frequency Counting",
    "difficulty": "easy",
    "question": "Given a list of strings, group anagrams together. What Python data structures would you use and why?",
    "answer": "Use collections.defaultdict(list) with sorted tuple of characters as key: `d[tuple(sorted(s))].append(s)`. Alternative: use Counter as key via `tuple(sorted(Counter(s).items()))`. O(n * k log k) where k is max string length.",
    "explanation": "defaultdict avoids KeyError checks. Sorting characters creates a canonical form — all anagrams share the same sorted form. Counter approach works but sorting is simpler.",
    "source": "dsa_python:hash_maps:001"
  },
  {
    "module": "dsa",
    "category": "trees",
    "topic": "BFS Level Order",
    "difficulty": "medium",
    "question": "Implement level-order traversal of a binary tree in Python. Return a list of lists where each inner list contains values at that level.",
    "answer": "Use collections.deque: `q = deque([root])`. While q: level_size = len(q), iterate level_size times popping left, append val to level list, enqueue children. Append level list to result. O(n) time, O(n) space.",
    "explanation": "deque gives O(1) popleft vs list's O(n). Processing exactly level_size nodes per iteration naturally groups by level without needing depth tracking.",
    "source": "dsa_python:trees:001"
  },
  {
    "module": "dsa",
    "category": "python_specific",
    "topic": "Generators and Itertools",
    "difficulty": "medium",
    "question": "Explain Python generators and when you'd use them over lists in data processing. Give an example using itertools for a combinatorial problem.",
    "answer": "Generators yield values lazily — O(1) memory vs O(n) for lists. Use for large datasets, infinite sequences, or pipeline processing. Example: `itertools.combinations(range(n), k)` generates all k-size subsets without storing them. Chain with `itertools.chain.from_iterable()` to flatten. `itertools.product()` for cartesian products.",
    "explanation": "Generators are Python's primary tool for memory-efficient iteration. In interviews, mentioning generators for large-scale data processing shows Python maturity beyond basic list comprehensions.",
    "source": "dsa_python:python_specific:001"
  }
]
```

Create `internal/tutor/questions/ai_llm.json` with 5 seed questions:
```json
[
  {
    "module": "ai",
    "category": "transformers",
    "topic": "Self-Attention Mechanism",
    "difficulty": "hard",
    "question": "Explain the self-attention mechanism in transformers. How are Q, K, V matrices computed? What is the purpose of the scaling factor? Describe in terms of Python/PyTorch operations.",
    "answer": "Q, K, V = input @ W_q, input @ W_k, input @ W_v where W matrices are learned. Attention = softmax(Q @ K.T / sqrt(d_k)) @ V. The sqrt(d_k) scaling prevents softmax saturation when d_k is large (dot products grow with dimension). In PyTorch: `torch.matmul(Q, K.transpose(-2, -1)) / math.sqrt(d_k)`, then `F.softmax(scores, dim=-1)`, then `torch.matmul(attn_weights, V)`.",
    "explanation": "Without scaling, large dot products push softmax into regions with tiny gradients, slowing training. Multi-head attention runs h parallel attention functions on d_k = d_model/h dimensional projections.",
    "source": "ai_llm:transformers:001"
  },
  {
    "module": "ai",
    "category": "rag",
    "topic": "Chunking Strategies",
    "difficulty": "medium",
    "question": "You're building a RAG pipeline in Python using LangChain. What chunking strategies would you consider and how do you choose chunk size? What are the trade-offs?",
    "answer": "Strategies: (1) Fixed-size with overlap (RecursiveCharacterTextSplitter), (2) Semantic chunking (split on topic boundaries), (3) Document-structure-aware (headers, paragraphs). Chunk size: 500-1000 tokens typical. Larger chunks = more context but less precise retrieval. Smaller = precise but may lose context. Overlap (10-20%) prevents information loss at boundaries. In LangChain: `RecursiveCharacterTextSplitter(chunk_size=1000, chunk_overlap=200)`.",
    "explanation": "Chunk size is the most impactful RAG parameter after embedding model choice. Too large and retrieval returns irrelevant content; too small and answers lack context. Semantic chunking is best but most expensive.",
    "source": "ai_llm:rag:001"
  },
  {
    "module": "ai",
    "category": "llm_fundamentals",
    "topic": "Quantization",
    "difficulty": "medium",
    "question": "Explain model quantization for LLM deployment. What are the common bit-width options and their trade-offs? How would you serve a quantized model in production using Python?",
    "answer": "Quantization reduces model precision: FP16 (half memory, minimal quality loss), INT8 (4x compression, slight quality loss), INT4/GPTQ/AWQ (8x compression, noticeable quality loss on complex tasks). For serving: vLLM with `--quantization awq` flag, or `transformers` with `BitsAndBytesConfig(load_in_4bit=True)`. Trade-off: memory/speed vs quality. AWQ generally outperforms GPTQ at same bit-width.",
    "explanation": "Quantization is essential for deploying large models on limited hardware. Understanding the quality-efficiency trade-off is critical for production ML engineering roles.",
    "source": "ai_llm:llm_fundamentals:001"
  },
  {
    "module": "ai",
    "category": "multi_agent",
    "topic": "LangGraph Orchestration",
    "difficulty": "hard",
    "question": "Design a multi-agent system using LangGraph in Python for a research assistant that can search the web, analyze documents, and synthesize answers. What are the key abstractions?",
    "answer": "Key abstractions: (1) StateGraph with TypedDict state, (2) Nodes = agent functions that take state and return updates, (3) Edges = conditional routing via `add_conditional_edges()`. Design: SearchAgent node → AnalyzeAgent node → SynthesizeAgent node. State carries `messages`, `documents`, `analysis`. Use `add_conditional_edges` from router node to branch based on query type. `graph.compile()` creates runnable. Checkpointing via `MemorySaver()` for conversation persistence.",
    "explanation": "LangGraph's state machine model is more controllable than pure agent loops. The graph structure makes debugging and testing individual nodes possible, unlike opaque agent chains.",
    "source": "ai_llm:multi_agent:001"
  },
  {
    "module": "ai",
    "category": "mlops",
    "topic": "Model Serving with vLLM",
    "difficulty": "medium",
    "question": "How does vLLM achieve high throughput for LLM inference? Explain PagedAttention and continuous batching. How would you deploy vLLM in production?",
    "answer": "vLLM uses PagedAttention: KV cache stored in non-contiguous pages (like OS virtual memory), eliminating memory waste from pre-allocation. Continuous batching: new requests join the batch as soon as a slot opens, vs static batching which waits for all to finish. Deploy: `python -m vllm.entrypoints.openai.api_server --model <model> --tensor-parallel-size <gpus>`. Use `--max-model-len` to cap context. Front with nginx for load balancing.",
    "explanation": "PagedAttention reduces KV cache memory waste from ~60-80% to ~4%. Combined with continuous batching, vLLM achieves 2-4x throughput over HuggingFace's default serving.",
    "source": "ai_llm:mlops:001"
  }
]
```

Create `internal/tutor/questions/system_design.json` with 5 seed questions:
```json
[
  {
    "module": "sysdesign",
    "category": "ml_system",
    "topic": "RAG Pipeline at Scale",
    "difficulty": "hard",
    "question": "Design a RAG (Retrieval-Augmented Generation) pipeline that handles 10M documents and serves 1000 QPS. Cover: ingestion, embedding, retrieval, generation, and monitoring.",
    "answer": "Ingestion: async workers consume from Kafka, chunk documents (1000 tokens, 200 overlap), store raw in S3. Embedding: batch inference via vLLM/TEI, store in vector DB (Pinecone/Qdrant). Retrieval: hybrid search (dense + BM25 via Elasticsearch), reranker (cross-encoder) on top-50 results, return top-5. Generation: LLM with retrieved context, stream response. Caching: Redis for repeated queries (semantic hash). Monitoring: retrieval recall metrics, answer relevance scoring, latency p50/p99. Scale: shard vector DB by document category, replicate for read throughput.",
    "explanation": "Key design decisions: hybrid search outperforms pure dense retrieval, reranking improves precision dramatically, caching handles the Zipf distribution of queries. Monitoring retrieval quality is more important than generation quality — garbage in, garbage out.",
    "source": "system_design:ml_system:001"
  },
  {
    "module": "sysdesign",
    "category": "ml_system",
    "topic": "Feature Store Architecture",
    "difficulty": "hard",
    "question": "Design a feature store that serves both batch training and real-time inference for a recommendation system with 100M users. Cover: offline/online stores, feature computation, consistency, and serving.",
    "answer": "Offline store: Spark jobs compute features hourly/daily, write to Parquet on S3, catalog in Hive metastore. Online store: Redis/DynamoDB for low-latency serving (<5ms). Sync: batch pipeline materializes latest features to online store. Real-time features: Kafka Streams/Flink for session-based features (last-N actions), dual-write to online store. Feature registry: metadata service with schema versioning, lineage tracking. Serving: feature server with batched lookups (get features for 100 candidates in 1 call). Consistency: eventual for batch features (hourly lag acceptable), strong for real-time features.",
    "explanation": "The dual-store pattern (offline + online) is standard. The key trade-off is freshness vs cost: real-time features are expensive but essential for session-based recommendations.",
    "source": "system_design:ml_system:002"
  },
  {
    "module": "sysdesign",
    "category": "data_system",
    "topic": "Data Lake Architecture",
    "difficulty": "hard",
    "question": "Design a data lake architecture for an enterprise ingesting data from 50+ sources (APIs, databases, file uploads) with 100GB daily volume. Cover: ingestion, storage layers, processing, governance, and cost optimization.",
    "answer": "Ingestion: CDC via Debezium for databases, API pollers with Airflow scheduling, S3 upload for files. Storage layers: Raw (Bronze) → Cleaned (Silver) → Aggregated (Gold) in S3 with Delta Lake/Iceberg format for ACID transactions. Processing: Spark on EMR for batch, Flink for streaming. Schema registry (Confluent or Glue) for evolution. Governance: Unity Catalog or AWS Lake Formation for access control, column-level tagging for PII. Data quality: Great Expectations checks at each layer boundary. Cost: lifecycle policies (IA after 30d, Glacier after 90d), partition pruning, Z-ordering for query optimization.",
    "explanation": "The medallion architecture (Bronze/Silver/Gold) is the industry standard. Delta Lake/Iceberg solve the 'small files problem' and provide ACID guarantees. Cost optimization through lifecycle policies is often overlooked but critical at scale.",
    "source": "system_design:data_system:001"
  },
  {
    "module": "sysdesign",
    "category": "data_system",
    "topic": "Streaming vs Batch Pipeline",
    "difficulty": "medium",
    "question": "You need to build an analytics pipeline that processes user click events. The business wants both real-time dashboards and daily aggregate reports. Design the pipeline covering both requirements.",
    "answer": "Lambda architecture variant: (1) Stream path: Kafka → Flink/Spark Streaming → real-time aggregations → Redis/Druid for dashboards. (2) Batch path: Kafka → S3 (raw events) → Spark batch (hourly/daily) → data warehouse (Redshift/Snowflake) for reports. Dedup: event-time windows + idempotent writes. Schema: Avro with schema registry for backward compatibility. Key decisions: Kappa (stream-only) is simpler but batch reprocessing is harder. Lambda adds complexity but gives you the 'recompute everything' safety net. For click events: Kappa with compacted Kafka topics + materialized views is usually sufficient.",
    "explanation": "The Lambda vs Kappa debate is a classic interview topic. Showing you understand both and can reason about when each applies demonstrates architectural maturity.",
    "source": "system_design:data_system:002"
  },
  {
    "module": "sysdesign",
    "category": "data_system",
    "topic": "Data Quality Framework",
    "difficulty": "medium",
    "question": "Design a data quality framework for an enterprise data platform. You've built one before at IBM-TWC — walk through the architecture, validation types, alerting, and how you'd make it self-service.",
    "answer": "Architecture: validation rules stored in config (Google Sheets/YAML → API), AWS Lambda validators triggered by Airflow after each pipeline stage. Validation types: (1) Schema validation (column types, nullability), (2) Statistical checks (z-score anomaly on row counts, value distributions), (3) Business rules (referential integrity, valid ranges), (4) Freshness checks (SLA monitoring). Alerting: severity-based (critical → PagerDuty, warning → Slack, info → dashboard). Self-service: rule builder UI/sheets where analysts define rules without code, AI-powered rule suggestions from data profiling. Reporting: daily quality scorecard per dataset, trend analysis, SLA compliance dashboard.",
    "explanation": "Data quality is the most impactful but least glamorous part of data engineering. A good framework prevents the 'garbage in, garbage out' problem that undermines all downstream analytics.",
    "source": "system_design:data_system:003"
  }
]
```

- [ ] **Step 2: Write loader.go**

Create `internal/tutor/questions/loader.go`:
```go
package questions

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

//go:embed dsa_python.json ai_llm.json system_design.json
var questionFS embed.FS

// Question represents a single question in the embedded JSON files.
type Question struct {
	Module      string `json:"module"`
	Category    string `json:"category"`
	Topic       string `json:"topic"`
	Difficulty  string `json:"difficulty"`
	QuestionTxt string `json:"question"`
	Answer      string `json:"answer"`
	Explanation string `json:"explanation"`
	Source      string `json:"source"`
}

// LoadStats reports how many items were seeded.
type LoadStats struct {
	TopicsCreated    int `json:"topicsCreated"`
	QuestionsCreated int `json:"questionsCreated"`
	QuestionsSkipped int `json:"questionsSkipped"`
}

// Load reads all embedded question JSON files and seeds them into the store.
// Idempotent — uses source field for dedup.
func Load(s *store.Store) (*LoadStats, error) {
	files := []string{"dsa_python.json", "ai_llm.json", "system_design.json"}
	stats := &LoadStats{}

	for _, file := range files {
		data, err := questionFS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("questions: read %s: %w", file, err)
		}

		var questions []Question
		if err := json.Unmarshal(data, &questions); err != nil {
			return nil, fmt.Errorf("questions: parse %s: %w", file, err)
		}

		for _, q := range questions {
			// Create or get topic (idempotent via UNIQUE constraint).
			topic, err := s.CreateTopic(q.Module, q.Category, q.Topic, q.Difficulty, "")
			if err != nil {
				log.Printf("questions: skip topic %s/%s/%s: %v", q.Module, q.Category, q.Topic, err)
				continue
			}
			stats.TopicsCreated++ // May count existing topics — that's fine for logging.

			// Create question (dedup by source field).
			_, err = s.CreateQuizQuestion(topic.ID, q.Difficulty, q.QuestionTxt, q.Answer, q.Explanation, q.Source)
			if err != nil {
				log.Printf("questions: skip question %s: %v", q.Source, err)
				continue
			}
			stats.QuestionsCreated++
		}
	}

	return stats, nil
}
```

- [ ] **Step 3: Write loader test**

Create `internal/tutor/questions/loader_test.go`:
```go
package questions

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	stats, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	if stats.QuestionsCreated == 0 {
		t.Error("expected questions to be created")
	}

	// Verify idempotency — second load should not create duplicates.
	stats2, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	// All questions should be skipped on second run (dedup by source).
	// QuestionsCreated will still count because CreateQuizQuestion returns existing.
	// Verify by checking total question count hasn't doubled.
	topics, _ := s.ListTopics("", "")
	totalQuestions := 0
	for _, topic := range topics {
		qs, _ := s.ListQuestions(topic.ID)
		totalQuestions += len(qs)
	}

	if totalQuestions != stats.QuestionsCreated {
		t.Errorf("expected %d total questions after 2 loads, got %d", stats.QuestionsCreated, totalQuestions)
	}

	_ = stats2
}

func TestLoadJSONValid(t *testing.T) {
	// Verify all embedded JSON files parse correctly.
	files := []string{"dsa_python.json", "ai_llm.json", "system_design.json"}
	for _, file := range files {
		data, err := questionFS.ReadFile(file)
		if err != nil {
			t.Errorf("cannot read %s: %v", file, err)
			continue
		}
		var questions []Question
		if err := json.Unmarshal(data, &questions); err != nil {
			t.Errorf("invalid JSON in %s: %v", file, err)
			continue
		}
		if len(questions) == 0 {
			t.Errorf("empty question bank: %s", file)
		}
		// Verify every question has required fields.
		for i, q := range questions {
			if q.Module == "" || q.Category == "" || q.Topic == "" || q.Source == "" {
				t.Errorf("%s[%d]: missing required field (module=%q, category=%q, topic=%q, source=%q)",
					file, i, q.Module, q.Category, q.Topic, q.Source)
			}
			if q.QuestionTxt == "" || q.Answer == "" {
				t.Errorf("%s[%d]: missing question or answer text", file, i)
			}
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/tutor/questions/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tutor/questions/
git commit -m "feat(tutor): add question loader with 15 seed questions (5 per module)"
```

---

## Task 3: Claude Semantic Evaluator

**Files:**
- Create: `internal/tutor/eval/eval.go`
- Create: `internal/tutor/eval/eval_test.go`

- [ ] **Step 1: Write eval.go**

Create `internal/tutor/eval/eval.go`:
```go
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// Sender sends a non-streaming request to Claude and returns the response.
type Sender interface {
	Send(ctx context.Context, req *stream.Request) (*stream.Response, error)
}

// Result holds the evaluation outcome.
type Result struct {
	Correct   bool     `json:"correct"`
	Score     float64  `json:"score"`
	Quality   int      `json:"quality"`
	Feedback  string   `json:"feedback"`
	KeyMissed []string `json:"keyMissed"`
	KeyHit    []string `json:"keyHit"`
}

// Evaluator evaluates answers using Claude, with word-overlap fallback.
type Evaluator struct {
	sender Sender
}

// New creates a new Evaluator. sender may be nil (uses fallback only).
func New(sender Sender) *Evaluator {
	return &Evaluator{sender: sender}
}

const evalSystemPrompt = `You are an expert technical interviewer evaluating a candidate's answer.
Given the reference answer and candidate's response, evaluate on:
1. Correctness of core concepts
2. Completeness (key points covered)
3. Technical accuracy of details
4. For Python questions: idiomatic Python usage

Return ONLY valid JSON with this exact schema:
{"correct": bool, "score": 0-100, "quality": 0-5, "feedback": "2-3 sentences", "keyMissed": ["concept1"], "keyHit": ["concept1"]}

Quality mapping: 5=perfect, 4=correct with gaps, 3=barely correct, 2=incorrect but close, 1=completely wrong, 0=no attempt/blank.
Score is granular 0-100: partial credit for partial answers.`

// Evaluate assesses a user's answer against a reference answer.
// Falls back to word overlap if Claude is unavailable.
func (e *Evaluator) Evaluate(ctx context.Context, questionText, referenceAnswer, userAnswer string) (*Result, error) {
	// Handle blank/skipped answers.
	trimmed := strings.TrimSpace(userAnswer)
	if trimmed == "" || strings.EqualFold(trimmed, "skip") || strings.EqualFold(trimmed, "idk") || strings.EqualFold(trimmed, "i don't know") {
		return &Result{
			Correct:   false,
			Score:     0,
			Quality:   0,
			Feedback:  "No answer provided.",
			KeyMissed: []string{"entire answer"},
			KeyHit:    []string{},
		}, nil
	}

	// Try Claude evaluation.
	if e.sender != nil {
		result, err := e.evaluateWithClaude(ctx, questionText, referenceAnswer, userAnswer)
		if err == nil {
			return result, nil
		}
		// Log and fall through to fallback.
		_ = err
	}

	// Fallback: word overlap.
	return e.evaluateWordOverlap(referenceAnswer, userAnswer), nil
}

func (e *Evaluator) evaluateWithClaude(ctx context.Context, questionText, referenceAnswer, userAnswer string) (*Result, error) {
	userMsg := fmt.Sprintf("Question:\n%s\n\nReference Answer:\n%s\n\nCandidate's Answer:\n%s", questionText, referenceAnswer, userAnswer)

	req := &stream.Request{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 1024,
		System:    evalSystemPrompt,
		Messages: []stream.Message{
			{Role: "user", Content: []stream.ContentBlock{{Type: "text", Text: userMsg}}},
		},
	}

	resp, err := e.sender.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("eval: claude send: %w", err)
	}

	// Extract text from response.
	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("eval: no text in response")
	}

	// Parse JSON — handle markdown code fences.
	text = extractJSON(text)

	var result Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("eval: parse response: %w", err)
	}

	// Clamp values.
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	if result.Quality < 0 {
		result.Quality = 0
	}
	if result.Quality > 5 {
		result.Quality = 5
	}

	return &result, nil
}

func (e *Evaluator) evaluateWordOverlap(referenceAnswer, userAnswer string) *Result {
	refWords := toWordSet(strings.ToLower(referenceAnswer))
	userWords := toWordSet(strings.ToLower(userAnswer))

	if len(refWords) == 0 {
		return &Result{Correct: true, Score: 100, Quality: 5, Feedback: "No reference answer to compare against."}
	}

	overlap := 0
	var keyHit, keyMissed []string
	for w := range refWords {
		if userWords[w] {
			overlap++
			keyHit = append(keyHit, w)
		} else {
			keyMissed = append(keyMissed, w)
		}
	}

	ratio := float64(overlap) / float64(len(refWords))
	score := ratio * 100
	correct := ratio >= 0.5

	quality := scoreToQuality(score)

	feedback := "Evaluated using word overlap (Claude unavailable)."
	if correct {
		feedback = fmt.Sprintf("%.0f%% keyword match. %s", score, feedback)
	} else {
		feedback = fmt.Sprintf("Only %.0f%% keyword match. %s", score, feedback)
	}

	return &Result{
		Correct:   correct,
		Score:     score,
		Quality:   quality,
		Feedback:  feedback,
		KeyMissed: keyMissed,
		KeyHit:    keyHit,
	}
}

func scoreToQuality(score float64) int {
	switch {
	case score >= 90:
		return 5
	case score >= 70:
		return 4
	case score >= 50:
		return 3
	case score >= 30:
		return 2
	case score > 0:
		return 1
	default:
		return 0
	}
}

func toWordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		if w != "" {
			set[w] = true
		}
	}
	return set
}

func extractJSON(text string) string {
	if start := strings.Index(text, "```json"); start >= 0 {
		text = text[start+7:]
		if end := strings.Index(text, "```"); end >= 0 {
			text = text[:end]
		}
	} else if start := strings.Index(text, "```"); start >= 0 {
		text = text[start+3:]
		if end := strings.Index(text, "```"); end >= 0 {
			text = text[:end]
		}
	}
	return strings.TrimSpace(text)
}
```

- [ ] **Step 2: Write eval_test.go**

Create `internal/tutor/eval/eval_test.go`:
```go
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rishav1305/soul-v2/internal/chat/stream"
)

// mockSender returns a canned response.
type mockSender struct {
	response *stream.Response
	err      error
}

func (m *mockSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	return m.response, m.err
}

func TestEvaluateBlankAnswer(t *testing.T) {
	e := New(nil) // no sender, fallback only
	result, err := e.Evaluate(context.Background(), "What is X?", "X is Y", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Quality != 0 {
		t.Errorf("expected quality=0 for blank, got %d", result.Quality)
	}
	if result.Score != 0 {
		t.Errorf("expected score=0 for blank, got %.0f", result.Score)
	}
}

func TestEvaluateSkipAnswer(t *testing.T) {
	e := New(nil)
	for _, skip := range []string{"skip", "idk", "I don't know", "  SKIP  "} {
		result, err := e.Evaluate(context.Background(), "Q?", "A", skip)
		if err != nil {
			t.Fatal(err)
		}
		if result.Quality != 0 {
			t.Errorf("expected quality=0 for %q, got %d", skip, result.Quality)
		}
	}
}

func TestEvaluateWordOverlapFallback(t *testing.T) {
	e := New(nil) // no sender → word overlap
	result, err := e.Evaluate(context.Background(),
		"What is a hash map?",
		"A hash map is a data structure that maps keys to values using a hash function for O(1) average lookup",
		"A hash map uses a hash function to map keys to values with constant time lookup",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score == 0 {
		t.Error("expected non-zero score for overlapping answer")
	}
	if result.Feedback == "" {
		t.Error("expected feedback")
	}
}

func TestEvaluateWithClaude(t *testing.T) {
	expected := Result{
		Correct:   true,
		Score:     85,
		Quality:   4,
		Feedback:  "Good answer covering key concepts.",
		KeyMissed: []string{"hash function"},
		KeyHit:    []string{"O(1) lookup", "key-value pairs"},
	}
	respJSON, _ := json.Marshal(expected)

	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{
				{Type: "text", Text: string(respJSON)},
			},
		},
	}

	e := New(sender)
	result, err := e.Evaluate(context.Background(),
		"What is a hash map?",
		"A hash map is a data structure...",
		"A hash map stores key-value pairs...",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score != 85 {
		t.Errorf("expected score=85, got %.0f", result.Score)
	}
	if result.Quality != 4 {
		t.Errorf("expected quality=4, got %d", result.Quality)
	}
}

func TestEvaluateClaudeFailsFallsBack(t *testing.T) {
	sender := &mockSender{err: fmt.Errorf("network error")}
	e := New(sender)

	result, err := e.Evaluate(context.Background(),
		"What is X?",
		"X is a data structure for efficient lookup",
		"X provides efficient lookup using hashing",
	)
	if err != nil {
		t.Fatal(err)
	}
	// Should have fallen back to word overlap.
	if result.Feedback == "" {
		t.Error("expected fallback feedback")
	}
}

func TestEvaluateClaudeMalformedJSON(t *testing.T) {
	sender := &mockSender{
		response: &stream.Response{
			Content: []stream.ContentBlock{
				{Type: "text", Text: "not valid json at all"},
			},
		},
	}
	e := New(sender)

	result, err := e.Evaluate(context.Background(), "Q?", "A", "my answer")
	if err != nil {
		t.Fatal(err)
	}
	// Should fall back to word overlap.
	if result == nil {
		t.Error("expected fallback result, got nil")
	}
}

func TestScoreToQuality(t *testing.T) {
	tests := []struct {
		score   float64
		quality int
	}{
		{100, 5}, {90, 5}, {85, 4}, {70, 4},
		{60, 3}, {50, 3}, {40, 2}, {30, 2},
		{20, 1}, {1, 1}, {0, 0},
	}
	for _, tt := range tests {
		got := scoreToQuality(tt.score)
		if got != tt.quality {
			t.Errorf("scoreToQuality(%.0f) = %d, want %d", tt.score, got, tt.quality)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"score": 85}`, `{"score": 85}`},
		{"```json\n{\"score\": 85}\n```", `{"score": 85}`},
		{"Here is the result:\n```\n{\"score\": 85}\n```\nDone.", `{"score": 85}`},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.expected {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/tutor/eval/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/tutor/eval/
git commit -m "feat(tutor): add Claude semantic evaluator with word-overlap fallback"
```

---

## Task 4: System Design Module

**Files:**
- Create: `internal/tutor/modules/sysdesign.go`
- Create: `internal/tutor/modules/sysdesign_test.go`

- [ ] **Step 1: Write sysdesign.go**

Create `internal/tutor/modules/sysdesign.go`:
```go
package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// SystemDesignModule handles system design interview prep.
type SystemDesignModule struct {
	store     *store.Store
	evaluator *eval.Evaluator
}

// Learn retrieves a system design topic and returns a structured framework.
func (m *SystemDesignModule) Learn(input map[string]interface{}) (*ToolResult, error) {
	topic, err := m.resolveTopic(input)
	if err != nil {
		return nil, err
	}

	m.store.UpdateTopicStatus(topic.ID, "learning")

	framework := generateSDFramework(topic.Name)

	return &ToolResult{
		Summary: fmt.Sprintf("System Design: %s (%s)", topic.Name, topic.Category),
		Data: map[string]interface{}{
			"topic":     topic,
			"framework": framework,
		},
	}, nil
}

// Drill handles quiz drilling for system design topics.
func (m *SystemDesignModule) Drill(input map[string]interface{}) (*ToolResult, error) {
	if qID, ok := getInt64(input, "question_id"); ok {
		return m.evaluateAnswer(qID, input)
	}

	topicID, ok := getInt64(input, "topic_id")
	if !ok {
		return nil, fmt.Errorf("sysdesign: drill requires 'topic_id' or 'question_id'")
	}

	question, err := m.store.PickNextQuestion(topicID)
	if err != nil {
		return nil, fmt.Errorf("sysdesign: pick question: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Question: %s", truncate(question.QuestionText, 80)),
		Data: map[string]interface{}{
			"question": question,
			"mode":     "question",
		},
	}, nil
}

func (m *SystemDesignModule) evaluateAnswer(questionID int64, input map[string]interface{}) (*ToolResult, error) {
	answer, _ := input["answer"].(string)
	if answer == "" {
		return nil, fmt.Errorf("sysdesign: drill answer requires 'answer' field")
	}

	question, err := m.store.GetQuizQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("sysdesign: get question: %w", err)
	}

	// Use Claude evaluation.
	var evalResult *eval.Result
	if m.evaluator != nil {
		evalResult, err = m.evaluator.Evaluate(context.Background(), question.QuestionText, question.AnswerText, answer)
		if err != nil {
			evalResult = nil
		}
	}

	// Fallback if evaluator is nil or errored.
	if evalResult == nil {
		correct := evaluateWordOverlap(answer, question.AnswerText, 0.5)
		score := 0.0
		if correct {
			score = 100.0
		}
		evalResult = &eval.Result{
			Correct:  correct,
			Score:    score,
			Quality:  2,
			Feedback: "Evaluated using word overlap (evaluator unavailable).",
		}
		if correct {
			evalResult.Quality = 4
		}
	}

	// Record progress.
	prog, err := m.store.RecordProgress(question.TopicID, evalResult.Score, 1, boolToInt(evalResult.Correct), 0, "")
	if err != nil {
		return nil, fmt.Errorf("sysdesign: record progress: %w", err)
	}

	if _, err := m.store.RecordAttempt(questionID, prog.ID, evalResult.Correct, 0, answer); err != nil {
		return nil, fmt.Errorf("sysdesign: record attempt: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	m.store.UpsertDailyActivity(today, "sysdesign", 0, 1, 1, evalResult.Score)

	// Update spaced repetition.
	sr, _ := m.store.GetSpacedRep(question.TopicID)
	currentInterval := 1.0
	currentEF := 2.5
	currentReps := 0
	if sr != nil {
		currentInterval = float64(sr.IntervalDays)
		currentEF = sr.EaseFactor
		currentReps = sr.RepetitionCount
	}

	sm2 := SM2Update(evalResult.Quality, currentInterval, currentEF, currentReps)
	m.store.UpsertSpacedRep(question.TopicID, sm2.NextReview, int(sm2.IntervalDays), sm2.EaseFactor, sm2.RepetitionCount)

	return &ToolResult{
		Summary: fmt.Sprintf("Score: %.0f/100 — %s", evalResult.Score, truncate(evalResult.Feedback, 80)),
		Data: map[string]interface{}{
			"score":      evalResult.Score,
			"quality":    evalResult.Quality,
			"correct":    evalResult.Correct,
			"feedback":   evalResult.Feedback,
			"keyMissed":  evalResult.KeyMissed,
			"keyHit":     evalResult.KeyHit,
			"answer":     question.AnswerText,
			"nextReview": sm2.NextReview.Format("2006-01-02"),
			"mode":       "result",
		},
	}, nil
}

// GenerateContent creates a new system design topic.
func (m *SystemDesignModule) GenerateContent(input map[string]interface{}) (*ToolResult, error) {
	category, _ := input["category"].(string)
	name, _ := input["name"].(string)
	if category == "" || name == "" {
		return nil, fmt.Errorf("sysdesign: generate requires 'category' and 'name' fields")
	}
	difficulty, _ := input["difficulty"].(string)
	if difficulty == "" {
		difficulty = "hard"
	}

	topic, err := m.store.CreateTopic("sysdesign", category, name, difficulty, "")
	if err != nil {
		return nil, fmt.Errorf("sysdesign: create topic: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Created system design topic: %s/%s", category, name),
		Data: map[string]interface{}{
			"topic": topic,
		},
	}, nil
}

func (m *SystemDesignModule) resolveTopic(input map[string]interface{}) (*store.Topic, error) {
	if topicID, ok := getInt64(input, "topic_id"); ok {
		return m.store.GetTopic(topicID)
	}

	topicName, _ := input["topic"].(string)
	category, _ := input["category"].(string)
	if topicName == "" {
		return nil, fmt.Errorf("sysdesign: requires 'topic_id' or 'topic' + 'category'")
	}

	if category != "" {
		return m.store.GetTopicByName("sysdesign", category, topicName)
	}

	topics, err := m.store.ListTopics("sysdesign", "")
	if err != nil {
		return nil, err
	}
	for i := range topics {
		if strings.EqualFold(topics[i].Name, topicName) {
			return &topics[i], nil
		}
	}
	return nil, fmt.Errorf("sysdesign: topic not found: %s", topicName)
}

func generateSDFramework(topic string) string {
	return fmt.Sprintf(`# System Design: %s

## Step 1: Requirements & Constraints
- Functional requirements: What must the system do?
- Non-functional: latency, throughput, availability, consistency
- Scale: users, data volume, QPS
- Clarify assumptions with the interviewer

## Step 2: Capacity Estimation
- Storage: how much data per day/year?
- Bandwidth: read/write ratio, peak QPS
- Memory: what needs caching?

## Step 3: High-Level Design
- Draw the major components
- Client → Load Balancer → API Gateway → Services → Storage
- Identify read vs write paths

## Step 4: Deep Dive
- Database choice and schema
- Caching strategy (what, where, invalidation)
- Scaling strategy (horizontal, sharding, replication)
- Data consistency model

## Step 5: Trade-offs & Extensions
- CAP theorem implications
- Cost optimization
- Monitoring and alerting
- Future scalability`, topic)
}
```

- [ ] **Step 2: Write sysdesign_test.go**

Create `internal/tutor/modules/sysdesign_test.go`:
```go
package modules

import (
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

func TestSystemDesignLearn(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	topic, err := s.CreateTopic("sysdesign", "ml_system", "RAG Pipeline", "hard", "")
	if err != nil {
		t.Fatal(err)
	}

	mod := &SystemDesignModule{store: s, evaluator: eval.New(nil)}
	result, err := mod.Learn(map[string]interface{}{"topic_id": float64(topic.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}

	// Verify topic status updated.
	updated, _ := s.GetTopic(topic.ID)
	if updated.Status != "learning" {
		t.Errorf("expected status=learning, got %s", updated.Status)
	}
}

func TestSystemDesignDrill(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	topic, _ := s.CreateTopic("sysdesign", "ml_system", "Feature Store", "hard", "")
	s.CreateQuizQuestion(topic.ID, "hard", "Design a feature store", "Use offline + online stores", "Dual store pattern", "test:001")

	mod := &SystemDesignModule{store: s, evaluator: eval.New(nil)}

	// Start mode.
	result, err := mod.Drill(map[string]interface{}{"topic_id": float64(topic.ID)})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]interface{})
	if data["mode"] != "question" {
		t.Error("expected question mode")
	}

	// Answer mode.
	q := data["question"].(*store.QuizQuestion)
	result, err = mod.Drill(map[string]interface{}{
		"question_id": float64(q.ID),
		"answer":      "Use offline and online stores with batch sync",
	})
	if err != nil {
		t.Fatal(err)
	}
	ansData := result.Data.(map[string]interface{})
	if ansData["mode"] != "result" {
		t.Error("expected result mode")
	}
}

func TestSystemDesignGenerate(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	mod := &SystemDesignModule{store: s, evaluator: eval.New(nil)}
	result, err := mod.GenerateContent(map[string]interface{}{
		"category": "data_system",
		"name":     "Event-Driven Pipeline",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]interface{})
	topic := data["topic"].(*store.Topic)
	if topic.Module != "sysdesign" {
		t.Errorf("expected module=sysdesign, got %s", topic.Module)
	}
	if topic.Difficulty != "hard" {
		t.Errorf("expected default difficulty=hard, got %s", topic.Difficulty)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/tutor/modules/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/tutor/modules/sysdesign.go internal/tutor/modules/sysdesign_test.go
git commit -m "feat(tutor): add System Design module with Claude evaluation"
```

---

## Task 5: Wire Evaluator into DSA + AI Modules

**Files:**
- Modify: `internal/tutor/modules/registry.go`
- Modify: `internal/tutor/modules/dsa.go`
- Modify: `internal/tutor/modules/ai.go`

- [ ] **Step 1: Update registry.go**

Replace the `Registry` struct and `NewRegistry` function in `internal/tutor/modules/registry.go`:

```go
package modules

import (
	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// ToolResult is the standard return type for all module methods.
type ToolResult struct {
	Summary string      `json:"summary"`
	Data    interface{} `json:"data"`
}

// Registry holds references to all tutor modules.
type Registry struct {
	Store        *store.Store
	ContentDir   string
	DSA          *DSAModule
	AI           *AIModule
	Behavioral   *BehavioralModule
	Mock         *MockModule
	Planner      *PlannerModule
	Progress     *ProgressModule
	SystemDesign *SystemDesignModule
}

// NewRegistry creates a Registry with all modules initialized.
// evaluator may be nil (modules fall back to word overlap).
func NewRegistry(s *store.Store, contentDir string, evaluator *eval.Evaluator) *Registry {
	return &Registry{
		Store:        s,
		ContentDir:   contentDir,
		DSA:          &DSAModule{store: s, contentDir: contentDir, evaluator: evaluator},
		AI:           &AIModule{store: s, contentDir: contentDir, evaluator: evaluator},
		Behavioral:   &BehavioralModule{store: s},
		Mock:         &MockModule{store: s},
		Planner:      &PlannerModule{store: s},
		Progress:     &ProgressModule{store: s},
		SystemDesign: &SystemDesignModule{store: s, evaluator: evaluator},
	}
}
```

- [ ] **Step 2: Add evaluator to DSAModule and update evaluateAnswer**

In `internal/tutor/modules/dsa.go`, add `evaluator` field to struct:

```go
type DSAModule struct {
	store      *store.Store
	contentDir string
	evaluator  *eval.Evaluator
}
```

Replace the `evaluateAnswer` method to use Claude evaluation:

```go
func (m *DSAModule) evaluateAnswer(questionID int64, input map[string]interface{}) (*ToolResult, error) {
	answer, _ := input["answer"].(string)
	if answer == "" {
		return nil, fmt.Errorf("dsa: drill answer requires 'answer' field")
	}

	question, err := m.store.GetQuizQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("dsa: get question: %w", err)
	}

	// Use Claude evaluation if available, fallback to word overlap.
	var evalResult *eval.Result
	if m.evaluator != nil {
		evalResult, err = m.evaluator.Evaluate(context.Background(), question.QuestionText, question.AnswerText, answer)
		if err != nil {
			evalResult = nil
		}
	}
	if evalResult == nil {
		correct := evaluateWordOverlap(answer, question.AnswerText, 0.5)
		score := 0.0
		quality := 2
		if correct {
			score = 100.0
			quality = 4
		}
		evalResult = &eval.Result{Correct: correct, Score: score, Quality: quality, Feedback: "Word overlap evaluation."}
	}

	prog, err := m.store.RecordProgress(question.TopicID, evalResult.Score, 1, boolToInt(evalResult.Correct), 0, "")
	if err != nil {
		return nil, fmt.Errorf("dsa: record progress: %w", err)
	}

	if _, err := m.store.RecordAttempt(questionID, prog.ID, evalResult.Correct, 0, answer); err != nil {
		return nil, fmt.Errorf("dsa: record attempt: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	topic, _ := m.store.GetTopic(question.TopicID)
	moduleName := "dsa"
	if topic != nil {
		moduleName = topic.Module
	}
	m.store.UpsertDailyActivity(today, moduleName, 0, 1, 1, evalResult.Score)

	sr, _ := m.store.GetSpacedRep(question.TopicID)
	currentInterval := 1.0
	currentEF := 2.5
	currentReps := 0
	if sr != nil {
		currentInterval = float64(sr.IntervalDays)
		currentEF = sr.EaseFactor
		currentReps = sr.RepetitionCount
	}

	sm2 := SM2Update(evalResult.Quality, currentInterval, currentEF, currentReps)
	m.store.UpsertSpacedRep(question.TopicID, sm2.NextReview, int(sm2.IntervalDays), sm2.EaseFactor, sm2.RepetitionCount)

	return &ToolResult{
		Summary: fmt.Sprintf("Score: %.0f/100 — %s", evalResult.Score, truncate(evalResult.Feedback, 80)),
		Data: map[string]interface{}{
			"score":       evalResult.Score,
			"quality":     evalResult.Quality,
			"correct":     evalResult.Correct,
			"feedback":    evalResult.Feedback,
			"keyMissed":   evalResult.KeyMissed,
			"keyHit":      evalResult.KeyHit,
			"answer":      question.AnswerText,
			"explanation": question.Explanation,
			"nextReview":  sm2.NextReview.Format("2006-01-02"),
			"mode":        "result",
		},
	}, nil
}
```

Add `"context"` and `"github.com/rishav1305/soul-v2/internal/tutor/eval"` to imports.

- [ ] **Step 3: Mirror the same changes in ai.go**

In `internal/tutor/modules/ai.go`:
- Add `evaluator *eval.Evaluator` field to `AIModule` struct
- Replace `evaluateAnswer` with the same Claude-first pattern (identical logic, just `"ai"` prefix in error messages)
- Add `"context"` and eval import

- [ ] **Step 4: Update server.go NewRegistry call**

In `internal/tutor/server/server.go`, the `New` function calls `modules.NewRegistry(s.store, s.contentDir)`. Update to pass evaluator:

```go
s.modules = modules.NewRegistry(s.store, s.contentDir, s.evaluator)
```

Add `evaluator *eval.Evaluator` field to `Server` struct and `WithEvaluator` option:

```go
func WithEvaluator(e *eval.Evaluator) Option { return func(srv *Server) { srv.evaluator = e } }
```

- [ ] **Step 5: Run all module tests**

Run: `go test -race ./internal/tutor/modules/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tutor/modules/ internal/tutor/server/server.go
git commit -m "feat(tutor): wire Claude evaluator into DSA, AI, SystemDesign modules"
```

---

## Task 6: Server — Evaluate Endpoint + SystemDesign Routes

**Files:**
- Modify: `internal/tutor/server/server.go`

- [ ] **Step 1: Add evaluate endpoint route**

In `server.go`, add to the route registration block:

```go
s.mux.HandleFunc("POST /api/tutor/evaluate", s.handleEvaluate)
```

- [ ] **Step 2: Implement handleEvaluate**

```go
func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		QuestionID int64  `json:"question_id"`
		Answer     string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if input.QuestionID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question_id is required"})
		return
	}
	if input.Answer == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "answer is required"})
		return
	}

	// Look up the question to determine which module to use.
	question, err := s.store.GetQuizQuestion(input.QuestionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "question not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Determine module from topic.
	topic, err := s.store.GetTopic(question.TopicID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Dispatch to the right module's drill evaluator.
	drillInput := map[string]interface{}{
		"question_id": float64(input.QuestionID),
		"answer":      input.Answer,
	}

	var result *modules.ToolResult
	switch topic.Module {
	case "dsa":
		result, err = s.modules.DSA.Drill(drillInput)
	case "ai":
		result, err = s.modules.AI.DrillTheory(drillInput)
	case "sysdesign":
		result, err = s.modules.SystemDesign.Drill(drillInput)
	default:
		result, err = s.modules.DSA.Drill(drillInput)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result.Data)
}
```

- [ ] **Step 3: Add sysdesign cases to handleToolExecute**

In the `switch toolName` block in `handleToolExecute`, add:

```go
// System Design tools
case "sysdesign_learn":
	result, err = s.modules.SystemDesign.Learn(input)
case "sysdesign_drill":
	result, err = s.modules.SystemDesign.Drill(input)
case "sysdesign_generate":
	result, err = s.modules.SystemDesign.GenerateContent(input)
```

- [ ] **Step 4: Write test for handleEvaluate endpoint**

Add to an existing server test file or create `internal/tutor/server/server_evaluate_test.go`:
```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

func TestHandleEvaluate(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	srv := New(WithStore(s), WithEvaluator(eval.New(nil)))

	// Create a topic + question.
	topic, _ := s.CreateTopic("dsa", "arrays", "test-topic", "medium", "")
	q, _ := s.CreateQuizQuestion(topic.ID, "medium", "What is a hash map?",
		"A hash map maps keys to values using a hash function", "O(1) lookup", "test:001")

	// Test valid evaluation.
	body, _ := json.Marshal(map[string]interface{}{
		"question_id": q.ID,
		"answer":      "A hash map uses a hash function to map keys to values",
	})
	req := httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Test missing question_id.
	body, _ = json.Marshal(map[string]interface{}{"answer": "test"})
	req = httptest.NewRequest("POST", "/api/tutor/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 5: Run server tests + vet**

Run: `go vet ./internal/tutor/... && go test -race ./internal/tutor/...`
Expected: PASS, no vet issues

- [ ] **Step 6: Commit**

```bash
git add internal/tutor/server/
git commit -m "feat(tutor): add /api/tutor/evaluate endpoint + sysdesign tool routes"
```

---

## Task 7: Wire Everything in cmd/tutor/main.go

**Files:**
- Modify: `cmd/tutor/main.go`

- [ ] **Step 1: Update main.go**

Replace the `runServe()` function to add stream client init, evaluator, and question loading:

```go
import (
	// ... existing imports ...
	"github.com/rishav1305/soul-v2/internal/chat/stream"
	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/questions"
	"github.com/rishav1305/soul-v2/pkg/auth"
)
```

After the store opening block (after `defer tutorStore.Close()`), add:

```go
	// Claude API client for semantic evaluation.
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	authSource := auth.NewOAuthTokenSource(credPath, nil)
	streamClient := stream.NewClient(authSource)

	// Create evaluator (uses stream client for Claude, falls back to word overlap).
	evaluator := eval.New(streamClient)
	log.Println("tutor: Claude evaluation enabled")
```

Replace the auto-import block with question loader:

```go
	// Seed embedded questions on boot (idempotent).
	loadStats, err := questions.Load(tutorStore)
	if err != nil {
		log.Printf("tutor: question loading error: %v", err)
	} else {
		log.Printf("tutor: questions loaded — %d created", loadStats.QuestionsCreated)
	}
```

Add evaluator to server options:

```go
	opts := []server.Option{
		server.WithStore(tutorStore),
		server.WithHost(host),
		server.WithPort(port),
		server.WithContentDir(contentDir),
		server.WithMetrics(metricsLogger),
		server.WithEvaluator(evaluator),
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/tutor/`
Expected: Build succeeds

- [ ] **Step 3: Run full test suite**

Run: `go vet ./internal/tutor/... ./cmd/tutor/ && go test -race ./internal/tutor/...`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add cmd/tutor/main.go
git commit -m "feat(tutor): wire Claude evaluator + question loader in main.go"
```

---

## Task 8: Claude Code Skills

**Files:**
- Create: `.claude/skills/drill/SKILL.md`
- Create: `.claude/skills/mock/SKILL.md`

- [ ] **Step 1: Write /drill skill**

Create `.claude/skills/drill/SKILL.md`:
````markdown
---
name: drill
description: Interactive interview prep drill using SM-2 spaced repetition. Presents questions, evaluates answers with Claude, tracks progress.
---

# Interview Drill

You are running an interactive interview drill session. The Tutor server must be running on port 3006.

## Flow

1. **Determine mode from args:**
   - No args → all modules, SM-2 picks next due
   - `dsa` → DSA questions only
   - `ai` → AI/LLM questions only
   - `sysdesign` → System Design only
   - `hard` → hard difficulty only
   - `status` → show progress summary and exit

2. **Get due reviews:**
   ```bash
   curl -s http://127.0.0.1:3006/api/tutor/drill/due
   ```
   Filter by module if specified. Pick the most overdue topic.

3. **If no due reviews, pick an unseen topic:**
   ```bash
   curl -s "http://127.0.0.1:3006/api/tutor/topics?module=MODULE"
   ```
   Pick a topic with status `not_started`.

4. **Start drill — get a question:**
   ```bash
   curl -s -X POST http://127.0.0.1:3006/api/tutor/drill/start -H 'Content-Type: application/json' -d '{"topic_id": TOPIC_ID}'
   ```

5. **Present the question conversationally:**
   - Show module, category, difficulty as context
   - Frame it as an interviewer would: "Let's talk about [topic]. [question]"
   - Do NOT show the reference answer

6. **Wait for the user's answer.** They type naturally in the conversation.

7. **Evaluate the answer:**
   ```bash
   curl -s -X POST http://127.0.0.1:3006/api/tutor/evaluate -H 'Content-Type: application/json' -d '{"question_id": QUESTION_ID, "answer": "USER_ANSWER"}'
   ```

8. **Present feedback:**
   - Score (0-100) with a visual indicator
   - Key concepts hit and missed
   - The reference answer for comparison
   - Claude's feedback
   - Next review date (from SM-2)

9. **Ask:** "Next question? Or type 'done' to finish."

10. **If continuing, go to step 2.** If done, show session summary:
    ```bash
    curl -s http://127.0.0.1:3006/api/tutor/dashboard
    ```
    Display: questions answered, average score, streak, module breakdown.

## Status Mode

When args contain `status`:
```bash
curl -s http://127.0.0.1:3006/api/tutor/dashboard
curl -s http://127.0.0.1:3006/api/tutor/drill/due
```
Show: total topics per module, completion %, due reviews count, streak, average scores.

## Style

- Be conversational, not robotic
- Frame questions as an interviewer would
- Give encouraging but honest feedback
- Reference the user's profile context when relevant (they work at Gartner/IBM-TWC, built GOAT, Data Quality Framework)
- For system design: ask follow-up probes like "What about failure modes?" or "How would you scale this 10x?"
````

- [ ] **Step 2: Write /mock skill**

Create `.claude/skills/mock/SKILL.md`:
````markdown
---
name: mock
description: Full mock interview session driven by a job description. Generates targeted questions, evaluates answers, provides structured feedback.
---

# Mock Interview

You are conducting a mock interview session. The Tutor server must be running on port 3006.

## Flow

1. **Get the job description:**
   - If user pastes a JD, use it directly
   - If user says "use lead N" or "use Turing", fetch from Scout API:
     ```bash
     curl -s http://127.0.0.1:3020/api/scout/leads
     ```
     Find the matching lead and use its description field.

2. **Optionally fetch profile for personalization:**
   ```bash
   curl -s http://127.0.0.1:3020/api/tools/resume_match/execute -X POST -H 'Content-Type: application/json' -d '{"lead_id": LEAD_ID}'
   ```
   Use profile context to personalize questions and identify gaps.

3. **Create mock session:**
   ```bash
   curl -s -X POST http://127.0.0.1:3006/api/tutor/mocks -H 'Content-Type: application/json' -d '{"type": "TYPE", "job_description": "JD_TEXT"}'
   ```
   Type: `technical`, `behavioral`, or `full_loop`.

4. **Generate 5-7 targeted questions** based on the JD:
   - Analyze the JD for key skills (Python, LangGraph, RAG, system design, AWS, etc.)
   - Cross-reference with the user's profile strengths and gaps
   - Generate questions that test both strengths (to build confidence) and gaps (to identify prep needs)
   - For technical: mix coding + system design
   - For behavioral: use STAR format targeting competencies in the JD
   - For full_loop: 2 technical + 2 behavioral + 1 HR

5. **Run each question:**
   - Present as an interviewer: "For this next question, imagine you're in a 45-minute technical screen..."
   - Wait for user's answer
   - Evaluate using the evaluate API:
     ```bash
     curl -s -X POST http://127.0.0.1:3006/api/tutor/evaluate -H 'Content-Type: application/json' -d '{"question_id": QID, "answer": "ANSWER"}'
     ```
   - Provide interview-style feedback: "Strong answer. You covered X well. Consider also mentioning Y because interviewers at this level typically look for..."
   - Track dimension scores: technical_depth, communication, structured_thinking

6. **Session complete — save results:**
   ```bash
   curl -s -X POST http://127.0.0.1:3006/api/tutor/mocks/SESSION_ID/answer -H 'Content-Type: application/json' -d '{
     "overall_score": SCORE,
     "feedback_json": "FEEDBACK_JSON",
     "scores": [
       {"dimension": "technical_depth", "score": N},
       {"dimension": "communication", "score": N},
       {"dimension": "structured_thinking", "score": N}
     ]
   }'
   ```

7. **Present final report:**
   - Overall score and assessment
   - Dimension breakdown with ratings
   - Top 3 areas to improve with specific recommendations
   - Suggested topics to drill next (link to `/drill` skill)

## Interaction Style

- Act as a professional interviewer, not a quiz machine
- Use the user's real experience for context: "Given your work on GOAT at Gartner, how would you..."
- For behavioral: probe deeper with follow-ups ("What was the impact?" "What would you do differently?")
- For system design: challenge assumptions ("What if the load doubles?" "What's your fallback?")
- Be encouraging but calibrate feedback to senior engineer level — don't sugar-coat gaps
````

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/drill/ .claude/skills/mock/
git commit -m "feat(tutor): add /drill and /mock Claude Code skills"
```

---

## Task 9: Populate Full Question Banks (130 questions)

**Files:**
- Modify: `internal/tutor/questions/dsa_python.json` (expand 5 → 50)
- Modify: `internal/tutor/questions/ai_llm.json` (expand 5 → 50)
- Modify: `internal/tutor/questions/system_design.json` (expand 5 → 30)

- [ ] **Step 1: Expand DSA Python to 50 questions**

Follow the distribution from the spec:
- Arrays/Strings: 10 questions (source: `dsa_python:arrays:001` through `dsa_python:arrays:010`)
- Hash Maps/Sets: 6 questions (source: `dsa_python:hash_maps:001` through `006`)
- Linked Lists: 4 questions (source: `dsa_python:linked_lists:001` through `004`)
- Trees/Graphs: 8 questions (source: `dsa_python:trees:001` through `008`)
- Dynamic Programming: 8 questions (source: `dsa_python:dp:001` through `008`)
- Stacks/Queues: 4 questions (source: `dsa_python:stacks:001` through `004`)
- Sorting/Searching: 4 questions (source: `dsa_python:sorting:001` through `004`)
- Python-specific: 6 questions (source: `dsa_python:python_specific:001` through `006`)

All questions must have Python-idiomatic answers with code snippets.

- [ ] **Step 2: Expand AI/LLM to 50 questions**

- Transformers: 8 (source: `ai_llm:transformers:001-008`)
- RAG: 6 (source: `ai_llm:rag:001-006`)
- LLM Fundamentals: 8 (source: `ai_llm:llm_fundamentals:001-008`)
- Multi-Agent: 6 (source: `ai_llm:multi_agent:001-006`)
- MLOps: 6 (source: `ai_llm:mlops:001-006`)
- Classical ML: 8 (source: `ai_llm:classical_ml:001-008`)
- Data Engineering for ML: 4 (source: `ai_llm:data_eng_ml:001-004`)
- Prompt Engineering: 4 (source: `ai_llm:prompt_eng:001-004`)

- [ ] **Step 3: Expand System Design to 30 questions**

- ML System Design: 15 (source: `system_design:ml_system:001-015`)
- Data System Design: 15 (source: `system_design:data_system:001-015`)

- [ ] **Step 4: Validate all JSON**

Run: `go test -race -run TestLoadJSONValid ./internal/tutor/questions/`
Expected: PASS — all 130 questions parse, all have required fields

- [ ] **Step 5: Run full loader test**

Run: `go test -race ./internal/tutor/questions/`
Expected: PASS — 130 questions loaded, idempotency verified

- [ ] **Step 6: Commit**

```bash
git add internal/tutor/questions/*.json
git commit -m "feat(tutor): populate full question banks — 50 DSA, 50 AI, 30 System Design"
```

---

## Task 10: Integration Test + Final Verification

**Files:**
- No new files — verification only

- [ ] **Step 1: Build everything**

Run: `go build ./cmd/tutor/`
Expected: Build succeeds

- [ ] **Step 2: Run full test suite with race detector**

Run: `go test -race ./internal/tutor/...`
Expected: All pass

- [ ] **Step 3: Run vet**

Run: `go vet ./internal/tutor/... ./cmd/tutor/`
Expected: No issues

- [ ] **Step 4: Smoke test the server**

```bash
# Start server in background
./bin/soul-tutor serve &
sleep 2

# Check health
curl -s http://127.0.0.1:3006/api/health | jq .

# Check questions loaded
curl -s http://127.0.0.1:3006/api/tutor/topics | jq '.[] | .module' | sort | uniq -c

# Check due reviews
curl -s http://127.0.0.1:3006/api/tutor/drill/due | jq .

# Stop server
kill %1
```

Expected: health OK, ~130 topics across dsa/ai/sysdesign modules, 0 due reviews (all new).

- [ ] **Step 5: Final commit**

```bash
git commit -m "test(tutor): verify full upgrade — 130 questions, Claude evaluation, 3 modules"
```
