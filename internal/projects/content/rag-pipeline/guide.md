# RAG Pipeline: Production-Grade Retrieval-Augmented Generation

## Overview

Retrieval-Augmented Generation (RAG) is the most widely deployed pattern for grounding LLM responses in factual, domain-specific data. Rather than relying solely on a model's parametric knowledge, RAG retrieves relevant documents at query time and injects them into the prompt context. This eliminates hallucinations on known data, keeps answers current without retraining, and provides citation traceability.

Building a production RAG pipeline teaches you the full lifecycle: document ingestion, chunking strategies, embedding generation, vector storage, retrieval ranking, and answer synthesis. These skills are directly applicable to enterprise search, customer support automation, internal knowledge bases, and compliance systems — the most common LLM use cases in industry today.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     RAG Pipeline                            │
│                                                             │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌─────────┐ │
│  │ Document  │──▶│ Chunking │──▶│ Embedding│──▶│ Vector  │ │
│  │ Loader   │   │ Engine   │   │ Model    │   │ Store   │ │
│  └──────────┘   └──────────┘   └──────────┘   └────┬────┘ │
│                                                      │      │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐        │      │
│  │ Response  │◀──│ LLM      │◀──│ Retriever│◀───────┘      │
│  │ + Cites  │   │ (Synth.) │   │ + Ranker │               │
│  └──────────┘   └──────────┘   └──────────┘               │
└─────────────────────────────────────────────────────────────┘
```

**Components:**

- **Document Loader** — Ingests PDFs, HTML, Markdown, DOCX. Handles encoding, metadata extraction, and deduplication.
- **Chunking Engine** — Splits documents into semantically coherent chunks. Supports recursive character splitting, sentence-based splitting, and semantic chunking.
- **Embedding Model** — Converts text chunks into dense vector representations. Uses sentence-transformers for local inference or OpenAI/Cohere APIs.
- **Vector Store** — Indexes embeddings for fast approximate nearest neighbor (ANN) search. ChromaDB for development, FAISS or Pinecone for production scale.
- **Retriever + Ranker** — Fetches top-k candidates via ANN search, then re-ranks with a cross-encoder for precision.
- **LLM Synthesis** — Generates answers grounded in retrieved context, with source citations.

**Data Flow:**

1. Documents are loaded, cleaned, and split into chunks (typically 256-512 tokens).
2. Each chunk is embedded and stored with metadata (source, page, section).
3. At query time, the user question is embedded and used to retrieve top-k chunks.
4. Retrieved chunks are optionally re-ranked by a cross-encoder.
5. The top chunks are formatted into a prompt and sent to the LLM with the user question.
6. The LLM generates an answer with citations referencing the source chunks.

## Key Concepts

### Chunking Strategies

Chunking quality is the single biggest lever for RAG performance. Poor chunks lead to irrelevant retrieval and hallucinated answers.

**Recursive Character Splitting** splits text by progressively smaller separators (`\n\n`, `\n`, `.`, ` `). It's simple and works well for structured documents. Set `chunk_size=512` and `chunk_overlap=64` as starting defaults.

**Semantic Chunking** uses embedding similarity between sentences to find natural breakpoints. Sentences with high cosine similarity stay together; drops in similarity indicate topic boundaries. This produces more coherent chunks but is slower to compute.

**Parent-Child Chunking** stores small chunks for retrieval precision but returns the parent (larger) chunk for context completeness. This gives you the best of both worlds: precise retrieval with sufficient context for generation.

### Embedding Models

The embedding model determines retrieval quality. Key considerations:

- **Dimensionality**: Higher dimensions (768-1024) capture more nuance but cost more storage and compute. 384 dimensions is often sufficient.
- **Max Sequence Length**: Most models truncate at 256-512 tokens. Chunks must fit within this limit.
- **Domain Adaptation**: General-purpose models (all-MiniLM-L6-v2) work for most cases. Domain-specific fine-tuning improves recall by 10-20% on specialized corpora.
- **Asymmetric vs Symmetric**: Some models are trained for query-document asymmetry (short query, long passage). Use these for search; use symmetric models for document-document similarity.

### Vector Search

FAISS uses HNSW (Hierarchical Navigable Small World) graphs for ANN search, achieving sub-millisecond latency on millions of vectors. ChromaDB wraps HNSW with a developer-friendly API and metadata filtering. For production, consider:

- **Index Type**: `IndexFlatL2` for exact search (small datasets), `IndexIVFFlat` for approximate search (medium), `IndexIVFPQ` for compressed search (large).
- **Metadata Filtering**: Pre-filter by source, date, or category before vector search to reduce the search space and improve relevance.
- **Hybrid Search**: Combine dense vector search with sparse keyword search (BM25) for better recall. Dense search handles semantic similarity; sparse search handles exact term matching.

### Re-Ranking

Initial retrieval casts a wide net (top-20 candidates). A cross-encoder re-ranker then scores each (query, document) pair jointly, which is more accurate than independent embedding comparison but too slow for first-stage retrieval. This two-stage approach gives you both speed and precision.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
langchain==0.2.16
langchain-community==0.2.16
langchain-openai==0.1.25
chromadb==0.5.5
sentence-transformers==3.0.1
faiss-cpu==1.8.0.post1
unstructured==0.15.7
pypdf==4.3.1
```

### Step 2: Document Ingestion

```python
from langchain_community.document_loaders import (
    PyPDFLoader, UnstructuredHTMLLoader, TextLoader
)
from langchain.text_splitter import RecursiveCharacterTextSplitter
from pathlib import Path

def load_documents(directory: str) -> list:
    """Load documents from a directory, handling multiple formats."""
    loaders = {
        ".pdf": PyPDFLoader,
        ".html": UnstructuredHTMLLoader,
        ".txt": TextLoader,
        ".md": TextLoader,
    }
    documents = []
    for path in Path(directory).rglob("*"):
        if path.suffix in loaders:
            loader = loaders[path.suffix](str(path))
            docs = loader.load()
            for doc in docs:
                doc.metadata["source"] = str(path)
                doc.metadata["filename"] = path.name
            documents.extend(docs)
    return documents

def chunk_documents(documents: list, chunk_size: int = 512,
                    chunk_overlap: int = 64) -> list:
    """Split documents into chunks with overlap for context continuity."""
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        separators=["\n\n", "\n", ". ", " ", ""],
        length_function=len,
    )
    chunks = splitter.split_documents(documents)
    # Add chunk index to metadata for citation tracking
    for i, chunk in enumerate(chunks):
        chunk.metadata["chunk_id"] = i
    return chunks
```

### Step 3: Embedding and Vector Store

```python
from langchain_community.embeddings import HuggingFaceEmbeddings
from langchain_community.vectorstores import Chroma
import chromadb

def create_vector_store(chunks: list, persist_dir: str = "./chroma_db"):
    """Create and persist a ChromaDB vector store from document chunks."""
    embeddings = HuggingFaceEmbeddings(
        model_name="sentence-transformers/all-MiniLM-L6-v2",
        model_kwargs={"device": "cpu"},
        encode_kwargs={"normalize_embeddings": True, "batch_size": 64},
    )
    vector_store = Chroma.from_documents(
        documents=chunks,
        embedding=embeddings,
        persist_directory=persist_dir,
        collection_metadata={"hnsw:space": "cosine"},
    )
    return vector_store

def load_vector_store(persist_dir: str = "./chroma_db"):
    """Load an existing vector store from disk."""
    embeddings = HuggingFaceEmbeddings(
        model_name="sentence-transformers/all-MiniLM-L6-v2",
        model_kwargs={"device": "cpu"},
        encode_kwargs={"normalize_embeddings": True},
    )
    return Chroma(
        persist_directory=persist_dir,
        embedding_function=embeddings,
    )
```

### Step 4: Retrieval with Re-Ranking

```python
from sentence_transformers import CrossEncoder

class ReRankingRetriever:
    def __init__(self, vector_store, top_k: int = 20, final_k: int = 5):
        self.vector_store = vector_store
        self.top_k = top_k
        self.final_k = final_k
        self.cross_encoder = CrossEncoder(
            "cross-encoder/ms-marco-MiniLM-L-6-v2"
        )

    def retrieve(self, query: str) -> list:
        # Stage 1: Broad retrieval via embedding similarity
        candidates = self.vector_store.similarity_search(
            query, k=self.top_k
        )
        if not candidates:
            return []

        # Stage 2: Re-rank with cross-encoder
        pairs = [(query, doc.page_content) for doc in candidates]
        scores = self.cross_encoder.predict(pairs)

        scored_docs = list(zip(candidates, scores))
        scored_docs.sort(key=lambda x: x[1], reverse=True)
        return [doc for doc, _ in scored_docs[:self.final_k]]
```

### Step 5: Answer Synthesis with Citations

```python
from langchain_openai import ChatOpenAI
from langchain.prompts import ChatPromptTemplate

SYSTEM_PROMPT = """You are a helpful assistant that answers questions based on
the provided context. Always cite your sources using [Source: filename] notation.
If the context doesn't contain enough information, say so explicitly.

Context:
{context}
"""

def build_rag_chain(vector_store):
    retriever = ReRankingRetriever(vector_store)
    llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)

    def answer(question: str) -> dict:
        docs = retriever.retrieve(question)
        context = "\n\n---\n\n".join(
            f"[Source: {d.metadata.get('filename', 'unknown')}]\n{d.page_content}"
            for d in docs
        )
        prompt = ChatPromptTemplate.from_messages([
            ("system", SYSTEM_PROMPT),
            ("human", "{question}"),
        ])
        chain = prompt | llm
        response = chain.invoke({"context": context, "question": question})
        return {
            "answer": response.content,
            "sources": [d.metadata for d in docs],
        }

    return answer
```

## Testing & Evaluation

### Retrieval Quality Metrics

- **Recall@k**: Fraction of relevant documents in top-k results. Aim for >0.85 at k=5.
- **MRR (Mean Reciprocal Rank)**: Average of 1/rank for the first relevant result. Measures how quickly you surface the right answer.
- **NDCG (Normalized Discounted Cumulative Gain)**: Accounts for position-weighted relevance. Best metric when you have graded relevance labels.

### End-to-End Evaluation

```python
from datasets import Dataset

def evaluate_rag(rag_chain, test_set: list[dict]) -> dict:
    """Evaluate RAG pipeline on a test set of question/answer pairs.

    test_set: [{"question": str, "expected_answer": str, "relevant_sources": list}]
    """
    correct_retrieval = 0
    total = len(test_set)

    for item in test_set:
        result = rag_chain(item["question"])
        retrieved_sources = {m.get("filename") for m in result["sources"]}
        expected_sources = set(item["relevant_sources"])
        if expected_sources & retrieved_sources:
            correct_retrieval += 1

    return {
        "retrieval_recall": correct_retrieval / total,
        "total_queries": total,
    }
```

### LLM-as-Judge for Answer Quality

Use a strong LLM to evaluate faithfulness (does the answer match the retrieved context?) and relevance (does the answer address the question?). Score on a 1-5 scale with rubrics. This is more scalable than human evaluation and correlates well with human judgments.

## Interview Angles

### Q1: How do you handle documents that are too long for the embedding model's context window?

**Sample Answer:** Chunking is essential. I use recursive character splitting with a chunk size of 512 tokens and 64-token overlap. The overlap ensures that information at chunk boundaries isn't lost. For more coherent chunks, semantic chunking groups sentences by embedding similarity — when similarity drops below a threshold, a new chunk starts. I also use parent-child chunking for retrieval: small chunks (256 tokens) are used as retrieval targets for precision, but the retrieval returns the parent chunk (1024 tokens) for generation context. The tradeoff is that smaller chunks give more precise retrieval but less context for generation; larger chunks provide more context but dilute relevance scores.

### Q2: When would you choose FAISS over a managed vector database like Pinecone?

**Sample Answer:** FAISS is ideal when you need low-latency search on a single machine, want to avoid external dependencies, or need fine-grained control over index parameters. It's free, runs in-process, and supports GPU acceleration. The tradeoff is that you manage persistence, scaling, and updates yourself. Pinecone or Weaviate make sense when you need managed infrastructure, multi-tenant isolation, automatic scaling, or when your team doesn't want to manage vector index operations. For a corpus under 10M vectors on a dedicated server, FAISS with IVF+PQ indexing often outperforms managed solutions on latency while being significantly cheaper.

### Q3: How do you evaluate RAG quality in production when you don't have labeled test sets?

**Sample Answer:** I use three approaches. First, LLM-as-judge: a strong model (GPT-4) scores each response on faithfulness (grounded in retrieved context) and relevance (addresses the question) using defined rubrics. Second, implicit feedback signals: click-through rates on cited sources, thumbs-up/down from users, and follow-up question rates. Third, retrieval diagnostics: I log the similarity scores of retrieved chunks and flag queries where the top score is below a threshold — these indicate retrieval failures. For faithfulness specifically, I check that every claim in the answer can be traced to a retrieved chunk, which catches hallucinations even without ground truth labels.

### Q4: What's the difference between naive RAG and advanced RAG, and when do you need the advanced version?

**Sample Answer:** Naive RAG does embed-retrieve-generate in a single pass. Advanced RAG adds query transformation (rewriting ambiguous queries, decomposing multi-part questions), hybrid retrieval (combining dense and sparse search), re-ranking (cross-encoder scoring), and iterative retrieval (using the LLM's partial answer to retrieve more context). You need advanced RAG when naive RAG fails on complex queries — for example, comparative questions ("How does X compare to Y?") benefit from query decomposition, while domain-specific jargon benefits from hybrid search where BM25 catches exact terms that embeddings miss. The tradeoff is latency: each additional stage adds 100-500ms. I typically start with naive RAG and add stages based on error analysis.

### Q5: How do you handle updates when source documents change?

**Sample Answer:** I implement incremental indexing with document-level checksums. Each document gets a hash stored in metadata. On re-ingestion, I compare hashes — unchanged documents are skipped, modified documents have their old chunks deleted and new chunks inserted, and deleted documents have their chunks removed. For ChromaDB, this means filtering by source metadata and deleting before reinserting. For FAISS, which doesn't support deletion natively, I use an ID mapping layer that marks deleted vectors and periodically rebuilds the index. In production, I run ingestion as a scheduled pipeline (e.g., nightly) with a change detection step. The tradeoff between real-time and batch updates depends on how stale your data can be — for most knowledge bases, nightly is sufficient.
