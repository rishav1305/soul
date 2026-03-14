package store

import (
	"fmt"
	"log"
)

type seedMilestone struct {
	Name string
	Desc string
	AC   string
}

type seedProject struct {
	Name        string
	Description string
	Phase       int
	Week        int
	Hours       float64
	Milestones  []seedMilestone
	Keywords    []string
}

// Seed populates the database with 11 projects, milestones, keywords, and syncs.
// Idempotent: skips if any projects exist.
func (s *Store) Seed() error {
	count, err := s.ProjectCount()
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("[projects] seed: %d projects exist, skipping", count)
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("projects: seed begin tx: %w", err)
	}
	defer tx.Rollback()

	projects := []seedProject{
		{
			Name:        "rag-pipeline",
			Description: "Build a production-grade Retrieval-Augmented Generation pipeline with document ingestion, embedding, vector storage, and retrieval chain.",
			Phase:       1, Week: 1, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "Document ingestion pipeline", Desc: "Parse PDF/markdown into chunks with metadata", AC: "Process 100+ documents without error"},
				{Name: "Embedding pipeline", Desc: "Generate embeddings using sentence-transformers", AC: "Batch embed 1000 chunks in <60s"},
				{Name: "Vector store integration", Desc: "Store/query embeddings with FAISS or ChromaDB", AC: "recall@10 > 0.80 on test queries"},
				{Name: "Retrieval chain", Desc: "Build query→retrieve→augment→generate chain", AC: "End-to-end query returns relevant, grounded answer"},
				{Name: "Evaluation framework", Desc: "Measure retrieval quality (recall, precision, MRR)", AC: "Automated eval script with baseline metrics"},
				{Name: "Reranking layer", Desc: "Add cross-encoder reranking to improve precision", AC: "precision@5 improves by >10% over baseline"},
			},
			Keywords: []string{"RAG", "Vector Database", "Embeddings", "FAISS", "ChromaDB", "Semantic Search", "Document Chunking"},
		},
		{
			Name:        "fine-tuning",
			Description: "Fine-tune open-source LLMs (LoRA/QLoRA) on custom datasets with evaluation and deployment.",
			Phase:       1, Week: 2, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "Dataset preparation", Desc: "Curate and format training data (instruction/response pairs)", AC: "1000+ clean examples in JSONL"},
				{Name: "LoRA adapter training", Desc: "Fine-tune base model with LoRA on single GPU", AC: "Training completes, loss decreases"},
				{Name: "QLoRA 4-bit training", Desc: "Quantized fine-tuning for memory efficiency", AC: "Train 7B model on 16GB GPU"},
				{Name: "Evaluation pipeline", Desc: "Compare fine-tuned vs base on benchmark tasks", AC: "Measurable improvement on target task"},
				{Name: "Merge and export", Desc: "Merge adapter weights, export to GGUF/HF format", AC: "Merged model loads and runs inference"},
				{Name: "Hyperparameter sweep", Desc: "Experiment with rank, alpha, learning rate", AC: "Document best config with metrics"},
			},
			Keywords: []string{"Fine-tuning", "LoRA", "QLoRA", "PEFT", "Instruction Tuning", "Model Merging"},
		},
		{
			Name:        "llm-evaluation",
			Description: "Build systematic LLM evaluation framework — automated benchmarks, human eval pipelines, and quality metrics.",
			Phase:       2, Week: 3, Hours: 17.5,
			Milestones: []seedMilestone{
				{Name: "Benchmark harness", Desc: "Run models against standard benchmarks (MMLU, HellaSwag)", AC: "Reproducible scores for 2+ models"},
				{Name: "Custom eval tasks", Desc: "Design domain-specific evaluation prompts", AC: "50+ eval prompts with rubrics"},
				{Name: "Automated scoring", Desc: "LLM-as-judge scoring pipeline", AC: "Judge agreement >80% with human labels"},
				{Name: "Human eval interface", Desc: "Side-by-side comparison tool", AC: "Rate 100 pairs, compute inter-annotator agreement"},
				{Name: "Metrics dashboard", Desc: "Aggregate scores by model, task, dimension", AC: "Single view showing model comparison"},
			},
			Keywords: []string{"LLM Evaluation", "Benchmarking", "MMLU", "LLM-as-Judge", "Human Evaluation"},
		},
		{
			Name:        "mlops-pipeline",
			Description: "End-to-end ML pipeline with experiment tracking, model registry, CI/CD, and monitoring.",
			Phase:       2, Week: 4, Hours: 17.5,
			Milestones: []seedMilestone{
				{Name: "Experiment tracking", Desc: "Set up MLflow/W&B for logging params, metrics, artifacts", AC: "10+ runs tracked with comparison view"},
				{Name: "Training pipeline", Desc: "Reproducible training with config-driven runs", AC: "Same config → same results (seed fixed)"},
				{Name: "Model registry", Desc: "Version, stage, and promote models", AC: "Model flows through staging→production"},
				{Name: "CI/CD pipeline", Desc: "Automated test→train→evaluate→deploy", AC: "Git push triggers full pipeline"},
				{Name: "Model monitoring", Desc: "Track drift, latency, error rates in production", AC: "Alert fires on synthetic drift injection"},
				{Name: "Data versioning", Desc: "Track dataset versions alongside model versions", AC: "Reproduce any past training run"},
			},
			Keywords: []string{"MLOps", "Experiment Tracking", "MLflow", "Model Registry", "CI/CD", "Data Versioning", "Model Monitoring"},
		},
		{
			Name:        "model-serving",
			Description: "Deploy ML models as production APIs with batching, caching, autoscaling, and observability.",
			Phase:       2, Week: 5, Hours: 17.5,
			Milestones: []seedMilestone{
				{Name: "REST API server", Desc: "Serve model inference via FastAPI/Flask", AC: "<100ms p95 latency on test input"},
				{Name: "Request batching", Desc: "Batch concurrent requests for GPU efficiency", AC: "2x+ throughput vs unbatched"},
				{Name: "Response caching", Desc: "Cache frequent queries (Redis/in-memory)", AC: "Cache hit rate >50% on repeated queries"},
				{Name: "Load testing", Desc: "Benchmark with concurrent users", AC: "Sustain 100 RPS without degradation"},
				{Name: "Container deployment", Desc: "Docker image with model baked in", AC: "docker run → serving in <30s"},
				{Name: "Autoscaling config", Desc: "K8s HPA or similar", AC: "Scale 1→3 replicas under load, back down at idle"},
			},
			Keywords: []string{"Model Serving", "FastAPI", "Inference Optimization", "Batching", "Docker", "Kubernetes", "Autoscaling"},
		},
		{
			Name:        "data-quality",
			Description: "Build data quality framework — validation, profiling, anomaly detection, and automated cleaning pipelines.",
			Phase:       3, Week: 6, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "Data profiling", Desc: "Generate statistical profiles of datasets", AC: "Profile report for 1M+ row dataset in <60s"},
				{Name: "Validation rules", Desc: "Schema + semantic validation with Great Expectations or custom", AC: "20+ rules catching known bad data"},
				{Name: "Anomaly detection", Desc: "Statistical outlier detection on numeric/categorical columns", AC: "Flag injected anomalies with >90% recall"},
				{Name: "Cleaning pipeline", Desc: "Automated fix/flag/reject workflow", AC: "Pipeline cleans test dataset, logs all actions"},
				{Name: "Quality dashboard", Desc: "Track quality metrics over time", AC: "Trend view showing improvement after fixes"},
				{Name: "Integration tests", Desc: "Validate pipeline on real-world messy data", AC: "Process 5 real datasets without crash"},
			},
			Keywords: []string{"Data Quality", "Data Validation", "Great Expectations", "Anomaly Detection", "Data Profiling", "Data Cleaning"},
		},
		{
			Name:        "agent-framework",
			Description: "Build multi-agent framework with tool use, planning, memory, and orchestration.",
			Phase:       3, Week: 7, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "Single agent loop", Desc: "Tool-calling agent with ReAct pattern", AC: "Agent solves 3-step task using 2+ tools"},
				{Name: "Tool registry", Desc: "Dynamic tool registration with schema validation", AC: "Add tool at runtime, agent discovers and uses it"},
				{Name: "Memory system", Desc: "Short-term (conversation) + long-term (vector) memory", AC: "Agent recalls facts from 10+ turns ago"},
				{Name: "Multi-agent orchestration", Desc: "Coordinator dispatches to specialist agents", AC: "2+ agents collaborate on a task"},
				{Name: "Planning module", Desc: "Agent decomposes complex task into subtasks", AC: "Plan + execute 5-step task autonomously"},
				{Name: "Guardrails", Desc: "Token limits, tool call limits, output validation", AC: "Agent gracefully stops at limits, no runaway loops"},
				{Name: "Evaluation", Desc: "Benchmark on agent tasks (SWE-bench lite, tool-use)", AC: "Documented scores with failure analysis"},
			},
			Keywords: []string{"AI Agents", "Multi-Agent Systems", "ReAct", "Tool Use", "Agent Memory", "Orchestration", "Guardrails"},
		},
		{
			Name:        "knowledge-graph",
			Description: "Build knowledge graph from unstructured text — entity extraction, relation mapping, graph storage, and querying.",
			Phase:       4, Week: 8, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "Entity extraction", Desc: "NER pipeline for domain entities", AC: "F1 >0.80 on test corpus"},
				{Name: "Relation extraction", Desc: "Extract entity relationships from text", AC: "Identify 5+ relation types with >0.70 precision"},
				{Name: "Graph storage", Desc: "Store in Neo4j or NetworkX", AC: "1000+ nodes, 5000+ edges loaded and queryable"},
				{Name: "Graph querying", Desc: "Natural language to graph query translation", AC: "Answer 10 test questions from graph data"},
				{Name: "Visualization", Desc: "Interactive graph explorer", AC: "Render subgraph with zoom/filter/search"},
				{Name: "GraphRAG integration", Desc: "Combine graph context with LLM retrieval", AC: "Better answers than pure vector RAG on relationship questions"},
			},
			Keywords: []string{"Knowledge Graph", "NER", "Relation Extraction", "Neo4j", "GraphRAG", "Entity Resolution"},
		},
		{
			Name:        "multimodal-ai",
			Description: "Build multimodal pipeline processing text + images — captioning, VQA, document understanding.",
			Phase:       4, Week: 9, Hours: 17.5,
			Milestones: []seedMilestone{
				{Name: "Image captioning", Desc: "Generate captions from images using vision model", AC: "Captions for 100 images, >70% human-rated \"good\""},
				{Name: "Visual QA", Desc: "Answer questions about image content", AC: ">60% accuracy on VQA test set"},
				{Name: "Document understanding", Desc: "Extract structured data from document images", AC: "Parse invoices/receipts with >80% field accuracy"},
				{Name: "Multimodal embeddings", Desc: "Joint text-image embedding space", AC: "Cross-modal retrieval works"},
				{Name: "Pipeline integration", Desc: "End-to-end: image in → structured output", AC: "Process 50 documents, output JSON"},
			},
			Keywords: []string{"Multimodal AI", "Vision-Language Models", "VQA", "Document Understanding", "Image Captioning", "CLIP"},
		},
		{
			Name:        "streaming-ai",
			Description: "Real-time AI inference with streaming — SSE, WebSocket delivery, token-by-token generation, and backpressure.",
			Phase:       4, Week: 9, Hours: 22.5,
			Milestones: []seedMilestone{
				{Name: "SSE streaming endpoint", Desc: "Stream LLM tokens via Server-Sent Events", AC: "First token <500ms, smooth delivery"},
				{Name: "WebSocket streaming", Desc: "Bidirectional streaming with cancellation", AC: "Client can cancel mid-generation"},
				{Name: "Backpressure handling", Desc: "Slow client doesn't block server", AC: "Slow consumer doesn't increase server memory"},
				{Name: "Multi-model routing", Desc: "Route requests to different models by task", AC: "3+ models behind single endpoint"},
				{Name: "Stream processing", Desc: "Transform/filter token stream (PII redaction, formatting)", AC: "Redact emails in real-time stream"},
				{Name: "Load testing", Desc: "Concurrent streaming sessions", AC: "50 concurrent streams without degradation"},
			},
			Keywords: []string{"Streaming AI", "SSE", "WebSocket", "Real-time Inference", "Backpressure", "Token Streaming"},
		},
		{
			Name:        "ai-safety",
			Description: "AI safety toolkit — content filtering, prompt injection detection, output validation, and red-teaming.",
			Phase:       4, Week: 10, Hours: 17.5,
			Milestones: []seedMilestone{
				{Name: "Content classifier", Desc: "Detect harmful/toxic content in inputs and outputs", AC: ">90% recall on standard toxicity benchmark"},
				{Name: "Prompt injection detector", Desc: "Identify injection attempts", AC: "Detect 80%+ of known injection patterns"},
				{Name: "Output validator", Desc: "Verify LLM output meets format/safety constraints", AC: "Reject malformed or unsafe outputs"},
				{Name: "Red-team harness", Desc: "Automated adversarial testing framework", AC: "Generate 100+ attack prompts, measure model robustness"},
				{Name: "Guardrail pipeline", Desc: "Chain: input filter → model → output filter", AC: "End-to-end pipeline blocks unsafe content, passes safe"},
			},
			Keywords: []string{"AI Safety", "Content Filtering", "Prompt Injection", "Red Teaming", "Guardrails", "Output Validation"},
		},
	}

	platforms := []string{"linkedin", "naukri", "indeed", "wellfound", "instahyre", "portfolio", "github"}

	for _, p := range projects {
		// Insert project.
		res, err := tx.Exec(
			"INSERT INTO projects (name, description, phase, week_planned, hours_estimated) VALUES (?, ?, ?, ?, ?)",
			p.Name, p.Description, p.Phase, p.Week, p.Hours,
		)
		if err != nil {
			return fmt.Errorf("projects: seed insert project %s: %w", p.Name, err)
		}
		projectID, _ := res.LastInsertId()

		// Insert milestones with sequential sort_order.
		for i, m := range p.Milestones {
			_, err := tx.Exec(
				"INSERT INTO milestones (project_id, name, description, acceptance_criteria, sort_order) VALUES (?, ?, ?, ?, ?)",
				projectID, m.Name, m.Desc, m.AC, i+1,
			)
			if err != nil {
				return fmt.Errorf("projects: seed insert milestone %s/%s: %w", p.Name, m.Name, err)
			}
		}

		// Insert keywords with project_id.
		for _, kw := range p.Keywords {
			_, err := tx.Exec(
				"INSERT OR IGNORE INTO keywords (project_id, keyword) VALUES (?, ?)",
				projectID, kw,
			)
			if err != nil {
				return fmt.Errorf("projects: seed insert keyword %s/%s: %w", p.Name, kw, err)
			}
		}

		// Insert 7 platform syncs.
		for _, platform := range platforms {
			_, err := tx.Exec(
				"INSERT INTO profile_syncs (project_id, platform) VALUES (?, ?)",
				projectID, platform,
			)
			if err != nil {
				return fmt.Errorf("projects: seed insert sync %s/%s: %w", p.Name, platform, err)
			}
		}
	}

	// 5 pre-shipped standalone keywords (no project_id).
	preShipped := []string{"Prompt Engineering", "Multi-Agent Systems", "Agentic AI", "LLM Orchestration", "Production AI"}
	for _, kw := range preShipped {
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO keywords (keyword, status, shipped_at) VALUES (?, 'shipped', datetime('now'))",
			kw,
		)
		if err != nil {
			return fmt.Errorf("projects: seed insert pre-shipped keyword %s: %w", kw, err)
		}
	}

	log.Printf("[projects] seed: inserted 11 projects with milestones, keywords, and syncs")
	return tx.Commit()
}
