# Model Serving: Production ML APIs with FastAPI

## Overview

Deploying ML models as reliable, scalable APIs is the bridge between training and value. A model sitting in a notebook is worth nothing — it needs to handle concurrent requests, respond within latency SLAs, batch efficiently, cache repeated predictions, and scale under load. This project builds a production-grade model serving system from scratch.

You'll learn request handling with FastAPI, inference optimization with batching, response caching with Redis, containerization with Docker, and load testing with Locust. These are core ML engineering skills — every deployed model needs this infrastructure, whether it's a classification model, an embedding service, or an LLM endpoint.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Model Serving Stack                          │
│                                                                 │
│  Client                                                        │
│    │                                                           │
│    ▼                                                           │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────┐ │
│  │ Load      │──▶│ FastAPI  │──▶│ Request  │──▶│ Inference  │ │
│  │ Balancer │   │ Server   │   │ Queue    │   │ Worker     │ │
│  └──────────┘   └────┬─────┘   └──────────┘   │ (Batched)  │ │
│                      │                          └──────┬─────┘ │
│                      ▼                                 │       │
│                 ┌──────────┐                           │       │
│                 │ Redis    │◀──────────────────────────┘       │
│                 │ Cache    │                                    │
│                 └──────────┘                                    │
│                                                                 │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                   │
│  │ Health   │   │ Metrics  │   │ Locust   │                   │
│  │ Check    │   │ (Prom.)  │   │ LoadTest │                   │
│  └──────────┘   └──────────┘   └──────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

**Components:**

- **FastAPI Server** — Async HTTP server handling prediction requests with input validation via Pydantic models.
- **Request Queue** — Collects individual requests and groups them into batches for efficient GPU utilization.
- **Inference Worker** — Runs model inference on batched inputs. Manages model loading, warm-up, and memory.
- **Redis Cache** — Caches predictions for repeated inputs. Reduces latency and compute cost for common queries.
- **Health Check** — Liveness and readiness probes for Kubernetes/Docker orchestration.
- **Metrics** — Prometheus-compatible metrics: request latency, throughput, queue depth, cache hit rate.
- **Load Balancer** — Distributes traffic across multiple server instances for horizontal scaling.

## Key Concepts

### Dynamic Batching

GPUs are massively parallel — processing 1 input vs 32 inputs takes nearly the same time. Dynamic batching collects individual requests arriving within a time window (e.g., 50ms) and processes them as a single batch. This dramatically improves throughput (10-50x) with minimal latency increase.

The key parameters are **max batch size** (limited by GPU memory) and **max wait time** (how long to collect requests before processing). Too short a wait time means small batches and wasted GPU cycles. Too long means high latency. Start with max_batch_size=32 and max_wait_ms=50, then tune based on your traffic pattern.

### Caching Strategy

ML predictions are deterministic for the same input (at temperature=0). Caching avoids redundant computation. Use a hash of the model version + input as the cache key. Set TTL based on how frequently the model changes — if you deploy weekly, a 7-day TTL is appropriate.

For embedding models, caching is especially valuable because the same documents are often re-embedded. For classification models, cache popular inputs (e.g., common customer queries). Monitor cache hit rate — below 10% means caching adds overhead without benefit.

### Graceful Degradation

Production services must handle failures gracefully. Key patterns:

- **Circuit breaker**: If the model fails repeatedly, stop sending requests and return a fallback response. This prevents cascade failures.
- **Timeout**: Set strict inference timeouts (e.g., 5 seconds). Kill requests that exceed the timeout rather than letting them queue up.
- **Graceful shutdown**: On SIGTERM, stop accepting new requests but finish in-flight ones. This prevents dropped requests during deployments.
- **Model fallback**: Keep a simpler model (or rule-based system) as a fallback when the primary model is unavailable.

### Model Loading

Loading a model into GPU memory takes 5-30 seconds depending on size. Do this at startup, not per-request. Use the FastAPI `lifespan` event to load models before the server accepts traffic. For multiple models, use a model registry pattern with lazy loading and LRU eviction.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
fastapi==0.115.0
uvicorn[standard]==0.30.6
redis==5.0.8
pydantic==2.9.1
torch==2.4.1
transformers==4.44.2
prometheus-client==0.20.0
locust==2.31.3
python-multipart==0.0.9
```

### Step 2: Core Server with Model Loading

```python
# server.py
import asyncio
import hashlib
import json
import time
from contextlib import asynccontextmanager
from typing import Optional

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from transformers import AutoTokenizer, AutoModelForSequenceClassification

# --- Models ---
class PredictionRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=10000)
    model_name: str = Field(default="default")

class PredictionResponse(BaseModel):
    label: str
    confidence: float
    model_version: str
    latency_ms: float
    cached: bool = False

class HealthResponse(BaseModel):
    status: str
    model_loaded: bool
    uptime_seconds: float

# --- Global State ---
class ModelState:
    def __init__(self):
        self.model = None
        self.tokenizer = None
        self.model_version = "v1.0.0"
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        self.start_time = time.time()
        self.request_count = 0

state = ModelState()

# --- Lifespan ---
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup: load model
    model_name = "distilbert-base-uncased-finetuned-sst-2-english"
    state.tokenizer = AutoTokenizer.from_pretrained(model_name)
    state.model = AutoModelForSequenceClassification.from_pretrained(model_name)
    state.model.to(state.device)
    state.model.set_grad_enabled(False)

    # Warm up with a dummy inference
    dummy = state.tokenizer("warmup", return_tensors="pt").to(state.device)
    with torch.no_grad():
        state.model(**dummy)
    print(f"Model loaded on {state.device}")

    yield

    # Shutdown: cleanup
    del state.model
    if torch.cuda.is_available():
        torch.cuda.empty_cache()

app = FastAPI(title="ML Model Server", lifespan=lifespan)
```

### Step 3: Prediction Endpoint with Caching

```python
# cache.py
import hashlib
import json
from typing import Optional
import redis.asyncio as redis

class PredictionCache:
    def __init__(self, redis_url: str = "redis://localhost:6379",
                 ttl_seconds: int = 86400):
        self.client = redis.from_url(redis_url)
        self.ttl = ttl_seconds
        self.hits = 0
        self.misses = 0

    def _make_key(self, text: str, model_version: str) -> str:
        content = json.dumps({"text": text, "v": model_version},
                            sort_keys=True)
        return f"pred:{hashlib.sha256(content.encode()).hexdigest()}"

    async def get(self, text: str, model_version: str) -> Optional[dict]:
        key = self._make_key(text, model_version)
        cached = await self.client.get(key)
        if cached:
            self.hits += 1
            return json.loads(cached)
        self.misses += 1
        return None

    async def set(self, text: str, model_version: str, result: dict):
        key = self._make_key(text, model_version)
        await self.client.setex(key, self.ttl, json.dumps(result))

    @property
    def hit_rate(self) -> float:
        total = self.hits + self.misses
        return self.hits / total if total > 0 else 0.0

cache = PredictionCache()

# Add to server.py
@app.post("/predict", response_model=PredictionResponse)
async def predict(request: PredictionRequest):
    start = time.time()
    state.request_count += 1

    # Check cache
    cached = await cache.get(request.text, state.model_version)
    if cached:
        return PredictionResponse(
            **cached, latency_ms=(time.time() - start) * 1000, cached=True
        )

    # Run inference
    inputs = state.tokenizer(
        request.text, return_tensors="pt", truncation=True,
        max_length=512, padding=True
    ).to(state.device)

    with torch.no_grad():
        outputs = state.model(**inputs)
        probs = torch.softmax(outputs.logits, dim=-1)
        confidence, predicted = torch.max(probs, dim=-1)

    labels = state.model.config.id2label
    result = {
        "label": labels[predicted.item()],
        "confidence": round(confidence.item(), 4),
        "model_version": state.model_version,
    }

    # Cache result
    await cache.set(request.text, state.model_version, result)

    return PredictionResponse(
        **result, latency_ms=(time.time() - start) * 1000, cached=False
    )
```

### Step 4: Dynamic Batching

```python
# batcher.py
import asyncio
import time
from dataclasses import dataclass
from typing import Any

@dataclass
class BatchItem:
    text: str
    future: asyncio.Future
    arrived_at: float

class DynamicBatcher:
    def __init__(self, model_state: ModelState, max_batch_size: int = 32,
                 max_wait_ms: float = 50.0):
        self.state = model_state
        self.max_batch_size = max_batch_size
        self.max_wait_ms = max_wait_ms
        self.queue: asyncio.Queue = asyncio.Queue()
        self._task = None

    async def start(self):
        self._task = asyncio.create_task(self._process_loop())

    async def stop(self):
        if self._task:
            self._task.cancel()

    async def predict(self, text: str) -> dict:
        """Submit a prediction request and wait for the result."""
        loop = asyncio.get_event_loop()
        future = loop.create_future()
        item = BatchItem(text=text, future=future, arrived_at=time.time())
        await self.queue.put(item)
        return await future

    async def _process_loop(self):
        while True:
            batch = []
            # Wait for first item
            item = await self.queue.get()
            batch.append(item)
            deadline = time.time() + self.max_wait_ms / 1000

            # Collect more items until batch full or timeout
            while len(batch) < self.max_batch_size:
                remaining = deadline - time.time()
                if remaining <= 0:
                    break
                try:
                    item = await asyncio.wait_for(
                        self.queue.get(), timeout=remaining
                    )
                    batch.append(item)
                except asyncio.TimeoutError:
                    break

            # Process batch
            await self._process_batch(batch)

    async def _process_batch(self, batch: list[BatchItem]):
        texts = [item.text for item in batch]
        try:
            inputs = self.state.tokenizer(
                texts, return_tensors="pt", truncation=True,
                max_length=512, padding=True
            ).to(self.state.device)

            with torch.no_grad():
                outputs = self.state.model(**inputs)
                probs = torch.softmax(outputs.logits, dim=-1)

            labels = self.state.model.config.id2label
            for i, item in enumerate(batch):
                confidence, predicted = torch.max(probs[i], dim=-1)
                result = {
                    "label": labels[predicted.item()],
                    "confidence": round(confidence.item(), 4),
                    "model_version": self.state.model_version,
                }
                item.future.set_result(result)
        except Exception as e:
            for item in batch:
                if not item.future.done():
                    item.future.set_exception(e)
```

### Step 5: Health Checks and Metrics

```python
from prometheus_client import Counter, Histogram, Gauge, generate_latest
from fastapi.responses import PlainTextResponse

REQUEST_COUNT = Counter("predictions_total", "Total predictions", ["status"])
REQUEST_LATENCY = Histogram("prediction_latency_seconds", "Prediction latency")
BATCH_SIZE = Histogram("batch_size", "Inference batch size",
                        buckets=[1, 2, 4, 8, 16, 32])
CACHE_HIT_RATE = Gauge("cache_hit_rate", "Cache hit rate percentage")

@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(
        status="healthy" if state.model is not None else "unhealthy",
        model_loaded=state.model is not None,
        uptime_seconds=time.time() - state.start_time,
    )

@app.get("/ready")
async def ready():
    if state.model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")
    return {"status": "ready"}

@app.get("/metrics")
async def metrics():
    CACHE_HIT_RATE.set(cache.hit_rate * 100)
    return PlainTextResponse(
        generate_latest(), media_type="text/plain; version=0.0.4"
    )
```

### Step 6: Load Testing

```python
# locustfile.py
from locust import HttpUser, task, between
import random

SAMPLE_TEXTS = [
    "This movie was absolutely fantastic, I loved every minute of it!",
    "Terrible experience, would not recommend to anyone.",
    "The product is okay, nothing special but it works.",
    "Best purchase I've ever made, exceeded all expectations!",
    "Disappointing quality for the price, expected much better.",
]

class PredictionUser(HttpUser):
    wait_time = between(0.1, 0.5)

    @task(10)
    def predict(self):
        self.client.post("/predict", json={
            "text": random.choice(SAMPLE_TEXTS)
        })

    @task(1)
    def health_check(self):
        self.client.get("/health")

# Run: locust -f locustfile.py --host http://localhost:8000
```

### Step 7: Dockerfile

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Download model at build time for faster starts
RUN python -c "from transformers import AutoTokenizer, AutoModelForSequenceClassification; \
    AutoTokenizer.from_pretrained('distilbert-base-uncased-finetuned-sst-2-english'); \
    AutoModelForSequenceClassification.from_pretrained('distilbert-base-uncased-finetuned-sst-2-english')"

EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s CMD curl -f http://localhost:8000/health || exit 1

CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]
```

## Testing & Measurement

### Performance Benchmarks

Run Locust with increasing concurrency to find:
- **P50/P95/P99 latency**: P99 should be under your SLA (e.g., <200ms). P50 is your typical experience.
- **Max throughput**: Requests per second at acceptable latency. With batching, expect 100-500 RPS per GPU.
- **Saturation point**: The concurrency level where latency starts increasing non-linearly.

### Correctness Tests

```python
def test_prediction_deterministic():
    """Same input should always produce same output."""
    response1 = client.post("/predict", json={"text": "Great product!"})
    response2 = client.post("/predict", json={"text": "Great product!"})
    assert response1.json()["label"] == response2.json()["label"]
    assert response1.json()["confidence"] == response2.json()["confidence"]

def test_empty_input_rejected():
    response = client.post("/predict", json={"text": ""})
    assert response.status_code == 422
```

### Reliability Targets

- **Availability**: 99.9% uptime (8.7 hours downtime per year)
- **Latency**: P99 < 200ms for cached, P99 < 500ms for uncached
- **Error rate**: < 0.1% of requests return 5xx errors

## Interview Angles

### Q1: How do you handle model updates without downtime?

**Sample Answer:** I use blue-green deployment. Two identical environments run behind a load balancer — "blue" serves production traffic, "green" is idle. To deploy a new model, I load it on the green environment, run health checks and a canary test suite, then switch the load balancer to route traffic to green. If anything fails, I switch back to blue instantly. The tradeoff vs rolling updates is that blue-green requires 2x infrastructure but provides instant rollback. For ML specifically, I also do a shadow deployment phase where both models receive traffic but only the old model's responses are returned — the new model's predictions are logged for offline comparison. This catches quality regressions before they affect users.

### Q2: When should you use dynamic batching vs processing requests individually?

**Sample Answer:** Dynamic batching is valuable when GPU utilization is the bottleneck — processing 1 input vs 32 inputs takes nearly the same time on a GPU due to parallelism. If your service handles >10 RPS and uses GPU inference, batching typically improves throughput by 10-30x. However, batching adds latency (the wait time to collect a batch, typically 20-100ms), so it's a throughput-latency tradeoff. For CPU inference, batching helps less because CPUs don't parallelize the same way. For very latency-sensitive applications (<10ms SLA), skip batching and process individually. The sweet spot is max_wait=50ms with max_batch=32 — this adds at most 50ms of latency but can 20x your throughput.

### Q3: How do you monitor ML models in production?

**Sample Answer:** I monitor at three levels. Infrastructure: CPU/GPU utilization, memory, request latency, error rates, and queue depth — standard service metrics. Model quality: prediction distribution drift (if the model suddenly predicts class A 90% of the time vs the training distribution of 60%, something changed), confidence score distribution (declining confidence suggests distribution shift), and feature drift on inputs. Business metrics: the downstream KPIs the model affects — conversion rate, resolution rate, etc. I use Prometheus + Grafana for infrastructure, custom drift detection that runs hourly comparing production distributions to training distributions, and A/B test dashboards for business impact. The key is setting up alerts on each level so you catch issues early.

### Q4: How do you decide between serving a model via API vs embedding it in the application?

**Sample Answer:** API serving (model-as-a-service) is better when: multiple applications consume the model, the model is large and needs GPU, you want to update the model independently of applications, or you need centralized monitoring. Embedded serving (model in-process) is better when: latency is critical (eliminates network hop), the model is small (scikit-learn, small ONNX), or the application is offline/edge. The tradeoff is operational complexity vs latency. API serving adds a network dependency and requires separate infrastructure, but gives you centralized management and resource efficiency. For LLMs and deep learning models, API serving is almost always the right choice due to GPU requirements. For lightweight models (<100MB), embedding is simpler and faster.
