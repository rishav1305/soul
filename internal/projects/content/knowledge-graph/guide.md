# Knowledge Graph: Extracting Structure from Unstructured Text

## Overview

Knowledge graphs transform unstructured text into structured, queryable networks of entities and relationships. They power question answering, recommendation systems, fraud detection, drug discovery, and enterprise search. Unlike vector search which finds similar text, knowledge graphs capture explicit relationships — "Company X acquired Company Y in 2024 for $2B" becomes a queryable fact.

This project builds a complete knowledge graph pipeline: named entity recognition (NER) to extract entities, relation extraction to identify connections, graph storage in Neo4j for querying, and embedding-based graph completion for discovering implicit relationships. These skills combine NLP, graph theory, and database engineering — a powerful combination for roles in data science, NLP engineering, and knowledge management.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                   Knowledge Graph Pipeline                       │
│                                                                  │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐ │
│  │ Document  │──▶│ NER      │──▶│ Relation │──▶│ Entity       │ │
│  │ Loader   │   │ (spaCy)  │   │ Extract  │   │ Resolution   │ │
│  └──────────┘   └──────────┘   └──────────┘   └──────┬───────┘ │
│                                                        │         │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐          │         │
│  │ Graph    │◀──│ Query    │◀──│ Neo4j    │◀─────────┘         │
│  │ Complete │   │ Engine   │   │ Storage  │                     │
│  └──────────┘   └──────────┘   └──────────┘                     │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Document Loader** — Ingests text from multiple sources (articles, PDFs, web pages). Handles sentence segmentation and preprocessing.
- **NER (spaCy)** — Extracts named entities: people, organizations, locations, dates, products. Uses pre-trained models with optional fine-tuning.
- **Relation Extraction** — Identifies relationships between entity pairs in the same sentence or passage. Uses transformer models or LLM prompting.
- **Entity Resolution** — Deduplicates entities that refer to the same real-world thing ("Google", "Alphabet Inc.", "GOOGL" all map to one node).
- **Neo4j Storage** — Graph database storing entities as nodes and relationships as edges. Supports Cypher queries for pattern matching.
- **Query Engine** — Natural language to Cypher translation for user-friendly graph querying.
- **Graph Completion** — Uses embeddings to predict missing relationships based on graph structure.

## Key Concepts

### Named Entity Recognition

NER identifies spans of text that refer to real-world entities and classifies them into categories. SpaCy's pre-trained models handle common entity types (PERSON, ORG, GPE, DATE, MONEY) with ~90% F1 on news text.

For domain-specific entities (drug names, gene symbols, financial instruments), you need to either fine-tune spaCy's NER or use a combination of rules (regex patterns, gazetteers) and ML. SpaCy's `EntityRuler` can add rule-based entities alongside ML predictions, which is effective for well-defined entity formats.

Key challenge: **nested entities**. "The New York Times" is an ORG, but "New York" within it is a GPE. Most NER systems extract only the outermost entity, which is usually correct for knowledge graph construction.

### Relation Extraction

Relation extraction identifies how entities are connected. Two main approaches:

**Pattern-based**: Define syntactic patterns using dependency parses. "X acquired Y" -> (X, ACQUIRED, Y). Fast and precise but low recall — misses paraphrases like "Y was purchased by X."

**Model-based**: Train a classifier on (entity1, entity2, context) triples to predict relationship type. Transformers fine-tuned on relation extraction datasets (TACRED, DocRED) achieve 70-80% F1. For custom domains, LLM prompting is an effective zero-shot alternative: "Given this text, identify the relationship between Entity A and Entity B."

**Open Information Extraction**: Instead of predicting from a fixed set of relation types, extract (subject, predicate, object) triples directly from text. More flexible but harder to normalize ("acquired", "bought", "purchased" should map to the same relation).

### Entity Resolution

The same entity appears in many forms across documents. Entity resolution (also called entity linking or deduplication) maps these to a single canonical form.

Approaches: (1) **String similarity** — Levenshtein distance, Jaro-Winkler, token overlap. Catches spelling variations. (2) **Embedding similarity** — Encode entity mentions with context using sentence-transformers. Semantically similar mentions cluster together. (3) **Knowledge base linking** — Link to Wikidata or a domain-specific knowledge base. If both "Jeff Bezos" and "Amazon's founder" link to Q312, they're the same entity.

### Graph Queries with Cypher

Neo4j's Cypher query language enables pattern matching on graphs:

```cypher
// Find all companies acquired by a specific company
MATCH (acquirer:Company {name: "Google"})-[:ACQUIRED]->(target:Company)
RETURN target.name, acquirer.name

// Find shortest path between two entities
MATCH path = shortestPath(
    (a:Person {name: "Elon Musk"})-[*]-(b:Company {name: "OpenAI"})
)
RETURN path

// Find common connections between two people
MATCH (a:Person {name: "Person A"})-[:WORKS_AT]->(org)<-[:WORKS_AT]-(b:Person {name: "Person B"})
RETURN org.name
```

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
spacy==3.7.6
networkx==3.3
neo4j==5.24.0
sentence-transformers==3.0.1
pandas==2.2.2
```

Install spaCy model:
```bash
python -m spacy download en_core_web_trf
```

### Step 2: Entity Extraction

```python
# entities.py
import spacy
from dataclasses import dataclass, field
from collections import defaultdict

@dataclass
class Entity:
    text: str
    label: str  # PERSON, ORG, GPE, DATE, etc.
    start: int
    end: int
    source: str = ""
    canonical: str = ""  # Resolved canonical form

    def __hash__(self):
        return hash((self.canonical or self.text, self.label))

class EntityExtractor:
    def __init__(self, model: str = "en_core_web_trf"):
        self.nlp = spacy.load(model)
        # Add custom entity patterns for domain-specific terms
        self.ruler = self.nlp.add_pipe(
            "entity_ruler", before="ner", config={"overwrite_ents": False}
        )

    def add_patterns(self, patterns: list[dict]):
        """Add custom entity patterns.

        Example: [{"label": "PRODUCT", "pattern": "iPhone 15"}]
        """
        self.ruler.add_patterns(patterns)

    def extract(self, text: str, source: str = "") -> list[Entity]:
        """Extract named entities from text."""
        doc = self.nlp(text)
        entities = []
        for ent in doc.ents:
            entities.append(Entity(
                text=ent.text,
                label=ent.label_,
                start=ent.start_char,
                end=ent.end_char,
                source=source,
            ))
        return entities

    def extract_batch(self, texts: list[str],
                      sources: list[str] = None) -> list[Entity]:
        """Extract entities from multiple texts efficiently."""
        sources = sources or [""] * len(texts)
        all_entities = []
        for doc, source in zip(self.nlp.pipe(texts, batch_size=32), sources):
            for ent in doc.ents:
                all_entities.append(Entity(
                    text=ent.text,
                    label=ent.label_,
                    start=ent.start_char,
                    end=ent.end_char,
                    source=source,
                ))
        return all_entities
```

### Step 3: Relation Extraction

```python
# relations.py
from dataclasses import dataclass
from typing import Optional
import spacy

@dataclass
class Relation:
    subject: Entity
    predicate: str
    obj: Entity
    confidence: float = 1.0
    source_text: str = ""

class RelationExtractor:
    def __init__(self):
        self.nlp = spacy.load("en_core_web_trf")

    def extract_dependency_relations(self, text: str,
                                     entities: list[Entity]) -> list[Relation]:
        """Extract relations using dependency parsing patterns."""
        doc = self.nlp(text)
        relations = []

        # Build entity span lookup
        entity_spans = {}
        for ent in entities:
            for i in range(ent.start, ent.end):
                entity_spans[i] = ent

        for token in doc:
            # Pattern: Subject -[nsubj]-> Verb -[dobj]-> Object
            if token.dep_ == "ROOT" and token.pos_ == "VERB":
                subject = None
                obj = None

                for child in token.children:
                    if child.dep_ in ("nsubj", "nsubjpass"):
                        # Check if this token is part of an entity
                        if child.idx in entity_spans:
                            subject = entity_spans[child.idx]
                        elif child.head.idx in entity_spans:
                            subject = entity_spans[child.head.idx]

                    if child.dep_ in ("dobj", "pobj", "attr"):
                        if child.idx in entity_spans:
                            obj = entity_spans[child.idx]

                if subject and obj:
                    relations.append(Relation(
                        subject=subject,
                        predicate=token.lemma_.upper(),
                        obj=obj,
                        source_text=text[:200],
                    ))

        return relations

    def extract_with_llm(self, text: str, entities: list[Entity],
                         client) -> list[Relation]:
        """Use an LLM to extract relations between entities."""
        entity_list = ", ".join(
            f"{e.text} ({e.label})" for e in entities
        )
        prompt = f"""Extract relationships between the following entities from this text.

Text: {text}

Entities: {entity_list}

For each relationship, output a JSON object:
{{"subject": "entity name", "predicate": "RELATIONSHIP_TYPE", "object": "entity name"}}

Output only valid JSON objects, one per line. Common relationship types:
WORKS_AT, FOUNDED, ACQUIRED, LOCATED_IN, PARTNER_OF, INVESTED_IN, CEO_OF
"""
        response = client.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        import json
        relations = []
        for line in response.content[0].text.strip().split("\n"):
            line = line.strip()
            if line.startswith("{"):
                try:
                    data = json.loads(line)
                    # Match to actual entities
                    subj = next(
                        (e for e in entities if e.text == data["subject"]), None
                    )
                    obj = next(
                        (e for e in entities if e.text == data["object"]), None
                    )
                    if subj and obj:
                        relations.append(Relation(
                            subject=subj,
                            predicate=data["predicate"],
                            obj=obj,
                            source_text=text[:200],
                        ))
                except (json.JSONDecodeError, KeyError):
                    continue

        return relations
```

### Step 4: Entity Resolution

```python
# resolution.py
from sentence_transformers import SentenceTransformer
from sklearn.cluster import AgglomerativeClustering
import numpy as np

class EntityResolver:
    def __init__(self, similarity_threshold: float = 0.85):
        self.model = SentenceTransformer("all-MiniLM-L6-v2")
        self.threshold = similarity_threshold
        self.canonical_map: dict[str, str] = {}  # mention -> canonical

    def resolve(self, entities: list[Entity]) -> list[Entity]:
        """Resolve entity mentions to canonical forms."""
        # Group by entity type
        by_type: dict[str, list[Entity]] = {}
        for ent in entities:
            by_type.setdefault(ent.label, []).append(ent)

        for label, type_entities in by_type.items():
            if len(type_entities) < 2:
                for e in type_entities:
                    e.canonical = e.text
                continue

            # Encode all mentions
            texts = [e.text for e in type_entities]
            embeddings = self.model.encode(texts)

            # Cluster similar mentions
            clustering = AgglomerativeClustering(
                n_clusters=None,
                distance_threshold=1 - self.threshold,
                metric="cosine",
                linkage="average",
            )
            labels = clustering.fit_predict(embeddings)

            # Assign canonical form (most frequent mention in cluster)
            clusters: dict[int, list[Entity]] = {}
            for ent, cluster_id in zip(type_entities, labels):
                clusters.setdefault(cluster_id, []).append(ent)

            for cluster_entities in clusters.values():
                # Most frequent mention becomes canonical
                from collections import Counter
                mention_counts = Counter(e.text for e in cluster_entities)
                canonical = mention_counts.most_common(1)[0][0]
                for e in cluster_entities:
                    e.canonical = canonical
                    self.canonical_map[e.text] = canonical

        return entities
```

### Step 5: Neo4j Graph Storage

```python
# graph_store.py
from neo4j import GraphDatabase

class KnowledgeGraphStore:
    def __init__(self, uri: str = "bolt://localhost:7687",
                 user: str = "neo4j", password: str = "password"):
        self.driver = GraphDatabase.driver(uri, auth=(user, password))

    def close(self):
        self.driver.close()

    def create_entity(self, entity: Entity):
        """Create or update an entity node."""
        with self.driver.session() as session:
            session.run(
                f"MERGE (e:{entity.label} {{name: $name}}) "
                f"SET e.canonical = $canonical, e.updated = datetime()",
                name=entity.canonical or entity.text,
                canonical=entity.canonical or entity.text,
            )

    def create_relation(self, relation: Relation):
        """Create a relationship between two entities."""
        with self.driver.session() as session:
            query = (
                f"MATCH (a:{relation.subject.label} {{name: $subj_name}}) "
                f"MATCH (b:{relation.obj.label} {{name: $obj_name}}) "
                f"MERGE (a)-[r:{relation.predicate}]->(b) "
                f"SET r.confidence = $confidence, "
                f"r.source = $source"
            )
            session.run(
                query,
                subj_name=relation.subject.canonical or relation.subject.text,
                obj_name=relation.obj.canonical or relation.obj.text,
                confidence=relation.confidence,
                source=relation.source_text[:100],
            )

    def ingest(self, entities: list[Entity], relations: list[Relation]):
        """Bulk ingest entities and relations."""
        for entity in entities:
            self.create_entity(entity)
        for relation in relations:
            self.create_relation(relation)

    def query(self, cypher: str, params: dict = None) -> list[dict]:
        """Execute a Cypher query and return results."""
        with self.driver.session() as session:
            result = session.run(cypher, params or {})
            return [record.data() for record in result]

    def find_connections(self, entity_name: str, max_depth: int = 3
                         ) -> list[dict]:
        """Find all entities connected to a given entity."""
        cypher = (
            "MATCH path = (e {name: $name})-[*1.." + str(max_depth) + "]-(connected) "
            "RETURN connected.name AS name, labels(connected) AS labels, "
            "length(path) AS distance "
            "ORDER BY distance"
        )
        return self.query(cypher, {"name": entity_name})

    def get_statistics(self) -> dict:
        """Get graph statistics."""
        node_count = self.query("MATCH (n) RETURN count(n) AS count")[0]["count"]
        edge_count = self.query("MATCH ()-[r]->() RETURN count(r) AS count")[0]["count"]
        labels = self.query(
            "MATCH (n) RETURN DISTINCT labels(n) AS label, count(n) AS count"
        )
        return {
            "total_nodes": node_count,
            "total_edges": edge_count,
            "node_types": {str(l["label"]): l["count"] for l in labels},
        }
```

### Step 6: NetworkX for Analysis

```python
# analysis.py
import networkx as nx

def build_networkx_graph(store: KnowledgeGraphStore) -> nx.DiGraph:
    """Export Neo4j graph to NetworkX for analysis."""
    G = nx.DiGraph()

    # Get all nodes
    nodes = store.query("MATCH (n) RETURN n.name AS name, labels(n) AS labels")
    for node in nodes:
        G.add_node(node["name"], labels=node["labels"])

    # Get all edges
    edges = store.query(
        "MATCH (a)-[r]->(b) "
        "RETURN a.name AS source, b.name AS target, type(r) AS relation"
    )
    for edge in edges:
        G.add_edge(edge["source"], edge["target"], relation=edge["relation"])

    return G

def analyze_graph(G: nx.DiGraph) -> dict:
    """Compute graph analytics."""
    return {
        "nodes": G.number_of_nodes(),
        "edges": G.number_of_edges(),
        "density": nx.density(G),
        "components": nx.number_weakly_connected_components(G),
        "most_connected": sorted(
            G.degree(), key=lambda x: x[1], reverse=True
        )[:10],
        "pagerank_top": sorted(
            nx.pagerank(G).items(), key=lambda x: x[1], reverse=True
        )[:10],
    }
```

## Testing & Measurement

### Pipeline Quality Metrics

- **NER F1**: Measure precision, recall, and F1 on a manually annotated sample. Target >85% F1 for common entity types.
- **Relation extraction accuracy**: Manually verify 100 extracted relations. Track precision (how many extracted relations are correct) and recall (how many true relations were found).
- **Entity resolution accuracy**: Measure cluster purity — what percentage of entities in each cluster actually refer to the same real-world entity.
- **Graph completeness**: Compare extracted triples against a gold-standard knowledge base for your domain.

### Testing Strategy

```python
def test_ner_quality():
    """Test NER on known examples."""
    extractor = EntityExtractor()
    test_cases = [
        ("Tim Cook is the CEO of Apple Inc.", [("Tim Cook", "PERSON"), ("Apple Inc.", "ORG")]),
        ("The deal was worth $2.5 billion.", [("$2.5 billion", "MONEY")]),
    ]
    for text, expected in test_cases:
        entities = extractor.extract(text)
        extracted = [(e.text, e.label) for e in entities]
        for exp in expected:
            assert exp in extracted, f"Missing: {exp} in {extracted}"
```

## Interview Angles

### Q1: How do you handle entity resolution at scale?

**Sample Answer:** At small scale (<10K entities), embedding-based clustering with agglomerative clustering works well — encode each mention with a sentence transformer, cluster by cosine similarity with a threshold of 0.85. At medium scale (10K-1M), I switch to blocking: first group entities by type and first character, then compare within blocks. This reduces O(n^2) comparisons to O(n*k) where k is the average block size. At large scale (>1M), I use locality-sensitive hashing (LSH) to find candidate pairs efficiently, then verify with embedding similarity. For production systems, I also maintain a curated alias table (manual mappings for known entities) that overrides automated resolution. The tradeoff is precision vs recall — strict thresholds merge fewer incorrect pairs but miss legitimate matches. I tune the threshold on a validation set targeting 95% precision.

### Q2: When would you use a knowledge graph vs a vector database?

**Sample Answer:** Knowledge graphs excel when relationships are the primary information — "who reports to whom," "which drugs interact," "what companies compete." They support multi-hop reasoning (A is connected to B which is connected to C) and structured queries that vector search can't handle. Vector databases excel at semantic similarity — finding passages or entities that are "about" a topic, even if the exact relationship isn't defined. In practice, I often use both: vector search for initial retrieval (find relevant entities and documents) and knowledge graph for structured reasoning over the retrieved entities. The tradeoff is construction cost — knowledge graphs require entity extraction, relation extraction, and entity resolution, which is expensive and error-prone. Vector databases just need embeddings. If you only need "find similar things," use vectors. If you need "find connections between things," use a graph.

### Q3: How do you evaluate the quality of a knowledge graph?

**Sample Answer:** I measure at three levels. Triple level: precision (what percentage of extracted triples are factually correct) and recall (what percentage of true triples are in the graph). I manually verify a random sample of 200 triples for precision, and compare against a reference knowledge base (e.g., Wikidata subset) for recall. Graph level: completeness (are there isolated nodes that should be connected?), consistency (are there contradictory triples like "X founded Y" and "X never worked at Y"?), and freshness (are dates current?). Application level: does the graph improve downstream task performance? If the graph powers a QA system, measure QA accuracy with and without the graph. The tradeoff is that manual verification is expensive but essential — automated metrics (node count, edge count, density) don't tell you if the content is correct.

### Q4: How do you keep a knowledge graph up to date as new information arrives?

**Sample Answer:** I implement an incremental ingestion pipeline. New documents are processed through the same NER + relation extraction pipeline, but with an additional step: before adding a new triple, check if it conflicts with or duplicates an existing triple. For conflicts (e.g., "CEO of Company X changed from Person A to Person B"), I use temporal annotations — each triple has a valid_from and valid_to timestamp. The old triple gets a valid_to date, and the new triple gets a valid_from date. For duplicates, I increment a confidence score (more sources confirming a triple = higher confidence). I run the ingestion pipeline nightly on new documents, with a monthly full reprocessing pass to catch entities that were missed by earlier NER models. The tradeoff is storage and compute cost — maintaining temporal annotations and running regular reprocessing is expensive, but without it, the graph becomes stale and contradictory.
