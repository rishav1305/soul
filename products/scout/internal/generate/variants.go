// Package generate provides resume and cover letter PDF generation using
// variant-based templates and headless Chrome (Rod) for HTML-to-PDF conversion.
package generate

// Variant defines a resume variant targeting a specific role archetype.
// Each variant tailors the headline, summary, skill emphasis, and cover
// letter template to match what recruiters expect for that role family.
type Variant struct {
	ID              string
	TargetRole      string
	Headline        string
	Summary         string
	LeadBullets     []string
	SkillEmphasis   []string
	ProjectEmphasis []string
	CoverTemplate   string
}

// Variants maps variant IDs (A-G) to their full definitions.
var Variants = map[string]Variant{
	"A": {
		ID:         "A",
		TargetRole: "AI Platform Architect / Solutions Architect",
		Headline:   "AI Platform Architect | Enterprise Agentic Systems | Cloud-Native ML Infrastructure",
		Summary: "Architect and technical leader with 8+ years building production AI platforms at scale. " +
			"Led the design and delivery of GOAT, Gartner's enterprise agentic AI platform serving 5,000+ analysts, " +
			"orchestrating multi-model pipelines across Azure and GCP. Proven track record of translating complex AI " +
			"research into resilient, observable, cloud-native systems.",
		LeadBullets: []string{
			"Architected GOAT agentic platform end-to-end: model orchestration, tool framework, evaluation harness, serving 5,000+ users at Gartner",
			"Designed multi-cloud ML infrastructure across Azure and GCP with auto-scaling, A/B model routing, and sub-second latency SLAs",
			"Built enterprise RAG pipelines processing 200K+ research documents with hybrid vector search and reranking",
			"Led platform migration from monolith to event-driven microservices, reducing deployment time by 70%",
		},
		SkillEmphasis:   []string{"System Architecture", "Azure/GCP", "Kubernetes", "LLM Orchestration", "RAG Pipelines", "Event-Driven Design"},
		ProjectEmphasis: []string{"Soul Platform", "GOAT Agentic Platform"},
		CoverTemplate: "I am writing to express my strong interest in the [ROLE] position at [COMPANY]. " +
			"As the architect behind GOAT, Gartner's enterprise agentic AI platform serving 5,000+ analysts, " +
			"I bring deep experience designing production AI systems that operate at scale. " +
			"[SPECIFIC THING] resonates with my work building multi-cloud ML infrastructure with " +
			"auto-scaling, model routing, and sub-second latency guarantees. I would welcome the opportunity " +
			"to bring this architectural expertise to [COMPANY].",
	},
	"B": {
		ID:         "B",
		TargetRole: "GenAI Engineer / LLM Engineer",
		Headline:   "GenAI Engineer | LLM Systems & Agentic Workflows | Production AI at Scale",
		Summary: "GenAI engineer specialising in large language model integration, agentic workflows, and " +
			"production-grade AI systems. Built GOAT at Gartner — an enterprise agentic platform with multi-model " +
			"orchestration, tool-use frameworks, and evaluation harnesses used by 5,000+ analysts. " +
			"Deep hands-on experience with prompt engineering, RAG, fine-tuning, and LLM evaluation.",
		LeadBullets: []string{
			"Built multi-model LLM orchestration layer for GOAT: dynamic model selection, prompt chaining, and tool-use across GPT-4, Claude, and Gemini",
			"Developed enterprise RAG system processing 200K+ documents with hybrid search, reranking, and citation grounding",
			"Created automated LLM evaluation harness with custom metrics for faithfulness, relevance, and safety",
			"Implemented agentic workflows with function calling, memory management, and human-in-the-loop controls",
		},
		SkillEmphasis:   []string{"LLM Integration", "Prompt Engineering", "RAG Systems", "Agentic Workflows", "Model Evaluation", "Python"},
		ProjectEmphasis: []string{"Soul Platform", "GOAT Agentic Platform"},
		CoverTemplate: "I am excited to apply for the [ROLE] role at [COMPANY]. " +
			"Having built GOAT, Gartner's enterprise agentic AI platform, I have deep production experience " +
			"with LLM orchestration, RAG systems, and agentic workflows at scale. " +
			"[SPECIFIC THING] aligns closely with my work on multi-model orchestration and tool-use frameworks. " +
			"I am eager to bring my GenAI expertise to [COMPANY] and contribute to your AI initiatives.",
	},
	"C": {
		ID:         "C",
		TargetRole: "Senior AI Engineer",
		Headline:   "Senior AI Engineer | End-to-End ML Systems | Enterprise AI Platforms",
		Summary: "Senior AI engineer with 8+ years delivering end-to-end machine learning systems across " +
			"enterprise, automotive, and pharmaceutical domains. Currently leading AI engineering at Gartner, " +
			"building the GOAT agentic platform that powers 5,000+ analysts. Previously built ML pipelines at " +
			"IBM-TWC, NLP systems at Novartis, and computer vision solutions at Polestar.",
		LeadBullets: []string{
			"Lead AI engineer on GOAT platform at Gartner: agentic orchestration, RAG pipelines, and LLM evaluation for 5,000+ users",
			"Built real-time weather ML pipelines at IBM-TWC processing 10B+ daily observations with sub-minute latency",
			"Developed NLP document processing system at Novartis reducing clinical trial document review time by 60%",
			"Created computer vision quality inspection system at Polestar achieving 98% defect detection accuracy",
		},
		SkillEmphasis:   []string{"Python", "PyTorch", "MLOps", "NLP", "Computer Vision", "Data Pipelines"},
		ProjectEmphasis: []string{"Soul Platform", "GOAT Agentic Platform", "IBM-TWC Weather ML"},
		CoverTemplate: "I am applying for the [ROLE] position at [COMPANY]. " +
			"With 8+ years building production AI systems — from enterprise agentic platforms at Gartner to " +
			"real-time ML pipelines at IBM-TWC — I bring broad and deep AI engineering experience. " +
			"[SPECIFIC THING] aligns with my track record of delivering reliable, scalable AI systems. " +
			"I would be thrilled to contribute my expertise to [COMPANY].",
	},
	"D": {
		ID:         "D",
		TargetRole: "AI Manager / AI Lead",
		Headline:   "AI Engineering Lead | Team Builder | Enterprise AI Strategy & Delivery",
		Summary: "AI engineering leader with experience building and managing cross-functional teams " +
			"delivering production AI platforms. At Gartner, led a team of engineers building GOAT, " +
			"the enterprise agentic AI platform serving 5,000+ analysts. Strong combination of hands-on " +
			"technical depth with stakeholder management, roadmap ownership, and talent development.",
		LeadBullets: []string{
			"Led AI engineering team at Gartner delivering GOAT platform: sprint planning, architecture reviews, and stakeholder alignment",
			"Drove AI strategy and roadmap for enterprise agentic capabilities, securing executive buy-in and budget approval",
			"Mentored junior engineers and established engineering practices: code review, testing standards, and documentation",
			"Managed cross-functional collaboration with product, research, and infrastructure teams across US and India offices",
		},
		SkillEmphasis:   []string{"Team Leadership", "AI Strategy", "Stakeholder Management", "Agile/Scrum", "Technical Architecture", "Hiring"},
		ProjectEmphasis: []string{"GOAT Agentic Platform", "Soul Platform"},
		CoverTemplate: "I am writing to express my interest in the [ROLE] position at [COMPANY]. " +
			"As the AI engineering lead at Gartner, I built and managed the team behind GOAT, our enterprise " +
			"agentic AI platform serving 5,000+ analysts. I combine deep technical expertise with proven " +
			"leadership in roadmap ownership, stakeholder alignment, and team development. " +
			"[SPECIFIC THING] resonates with my experience, and I am excited about the opportunity at [COMPANY].",
	},
	"E": {
		ID:         "E",
		TargetRole: "AI Consultant / Freelance",
		Headline:   "AI Consultant | Agentic Systems & LLM Strategy | Enterprise AI Delivery",
		Summary: "Independent AI consultant with 8+ years of enterprise AI delivery experience. " +
			"Most recently architected GOAT, Gartner's agentic AI platform serving 5,000+ analysts. " +
			"I help organisations design and implement production AI systems — from LLM strategy and " +
			"RAG architecture to agentic workflows and MLOps pipelines.",
		LeadBullets: []string{
			"Delivered GOAT agentic platform at Gartner: architecture, implementation, and production launch for 5,000+ users",
			"Consulted on LLM integration strategy for enterprise clients across research, automotive, and pharmaceutical sectors",
			"Designed and built RAG systems, evaluation frameworks, and agentic workflows for production deployment",
			"Built Soul — an open-source AI development platform with autonomous task execution and git worktree isolation",
		},
		SkillEmphasis:   []string{"AI Strategy", "LLM Architecture", "RAG Systems", "Agentic Workflows", "Technical Advisory", "Rapid Prototyping"},
		ProjectEmphasis: []string{"Soul Platform", "GOAT Agentic Platform", "Portfolio Website"},
		CoverTemplate: "I am reaching out regarding the [ROLE] opportunity at [COMPANY]. " +
			"As an AI consultant who architected GOAT, Gartner's enterprise agentic platform, " +
			"I specialise in helping organisations build production AI systems that deliver real value. " +
			"[SPECIFIC THING] is an area where I can make an immediate impact, drawing on my experience " +
			"with LLM orchestration, RAG pipelines, and agentic workflows. I would love to discuss how " +
			"I can help [COMPANY] achieve its AI objectives.",
	},
	"F": {
		ID:         "F",
		TargetRole: "AI Researcher",
		Headline:   "Applied AI Researcher | Agentic Systems | LLM Evaluation & Alignment",
		Summary: "Applied AI researcher with production experience bridging research and engineering. " +
			"Built evaluation frameworks and agentic architectures for GOAT at Gartner, exploring " +
			"multi-model orchestration, tool-use reliability, and faithfulness metrics at enterprise scale. " +
			"Interested in agentic systems, LLM evaluation, and human-AI interaction patterns.",
		LeadBullets: []string{
			"Designed LLM evaluation framework measuring faithfulness, relevance, safety, and tool-use reliability across multiple models",
			"Researched and implemented multi-model orchestration strategies: dynamic routing, fallback chains, and ensemble approaches",
			"Built agentic workflow architectures exploring planning, memory, and self-correction patterns in production",
			"Developed hybrid retrieval systems combining dense embeddings, sparse search, and learned reranking models",
		},
		SkillEmphasis:   []string{"LLM Evaluation", "Agentic Systems", "Retrieval Methods", "Experiment Design", "Python", "PyTorch"},
		ProjectEmphasis: []string{"GOAT Agentic Platform", "Soul Platform"},
		CoverTemplate: "I am excited about the [ROLE] opportunity at [COMPANY]. " +
			"My work on GOAT at Gartner sits at the intersection of research and production — designing " +
			"evaluation frameworks, agentic architectures, and retrieval systems that work reliably at scale. " +
			"[SPECIFIC THING] aligns with my research interests in agentic systems and LLM evaluation. " +
			"I would welcome the chance to contribute to [COMPANY]'s research efforts.",
	},
	"G": {
		ID:         "G",
		TargetRole: "Senior Data Engineer (AI-focused)",
		Headline:   "Senior Data Engineer | AI/ML Data Platforms | Real-Time & Batch Pipelines",
		Summary: "Senior data engineer with 8+ years building data platforms that power AI and ML systems. " +
			"Built real-time data pipelines at IBM-TWC processing 10B+ daily weather observations, and " +
			"designed the data infrastructure behind GOAT at Gartner — ingesting, transforming, and serving " +
			"200K+ research documents for enterprise RAG and analytics.",
		LeadBullets: []string{
			"Built real-time data pipelines at IBM-TWC ingesting 10B+ daily observations with Apache Kafka, Spark, and Cassandra",
			"Designed data platform for GOAT at Gartner: document ingestion, embedding pipelines, vector stores, and analytics",
			"Created ETL frameworks processing pharmaceutical clinical trial data at Novartis with data quality validation",
			"Implemented data versioning, lineage tracking, and automated quality checks across multiple production systems",
		},
		SkillEmphasis:   []string{"Apache Spark", "Kafka", "SQL", "Data Modeling", "ETL/ELT", "Cloud Data Services"},
		ProjectEmphasis: []string{"GOAT Agentic Platform", "IBM-TWC Weather ML"},
		CoverTemplate: "I am applying for the [ROLE] position at [COMPANY]. " +
			"With experience building data platforms that power AI systems — from 10B+ daily observation " +
			"pipelines at IBM-TWC to the document infrastructure behind Gartner's GOAT platform — " +
			"I bring strong data engineering skills with a focus on AI/ML workloads. " +
			"[SPECIFIC THING] aligns with my expertise in building reliable, scalable data systems. " +
			"I am keen to bring this experience to [COMPANY].",
	},
}
