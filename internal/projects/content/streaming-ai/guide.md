# Streaming AI: Real-Time Inference with SSE and WebSocket

## Overview

Streaming transforms the AI user experience. Instead of waiting 5-30 seconds for a complete response, users see tokens appear in real-time, reducing perceived latency to near-zero. Streaming is the default for every major LLM interface (ChatGPT, Claude, Gemini) and is essential for production AI applications.

This project builds a real-time AI inference system with three streaming protocols: Server-Sent Events (SSE) for unidirectional streaming, WebSocket for bidirectional communication, and async generators for efficient token-by-token processing. You'll handle backpressure, connection management, error recovery, and concurrent streams. These skills apply directly to building chat interfaces, real-time dashboards, and any system that surfaces AI responses incrementally.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                  Streaming AI Architecture                       │
│                                                                  │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │ Client   │◀─▶│ WebSocket    │◀──│ Connection   │            │
│  │ (Browser)│   │ Handler      │   │ Manager      │            │
│  └──────────┘   └──────┬───────┘   └──────────────┘            │
│                         │                                        │
│  ┌──────────┐   ┌──────▼───────┐   ┌──────────────┐            │
│  │ Client   │◀──│ SSE          │◀──│ Async        │            │
│  │ (HTTP)   │   │ Handler      │   │ Generator    │            │
│  └──────────┘   └──────┬───────┘   └──────────────┘            │
│                         │                                        │
│                  ┌──────▼───────┐                                │
│                  │ LLM Provider │                                │
│                  │ (Streaming)  │                                │
│                  │ - OpenAI     │                                │
│                  │ - Anthropic  │                                │
│                  │ - Local      │                                │
│                  └──────────────┘                                │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **WebSocket Handler** — Bidirectional communication for chat interfaces. Handles multiple concurrent conversations, heartbeats, and reconnection.
- **SSE Handler** — Unidirectional server-to-client streaming over HTTP. Simpler than WebSocket, works through proxies, supports automatic reconnection.
- **Connection Manager** — Tracks active connections, handles cleanup on disconnect, enforces connection limits.
- **Async Generator** — Converts LLM streaming responses into async iterables that can feed either SSE or WebSocket outputs.
- **LLM Provider** — Abstraction over streaming APIs (OpenAI, Anthropic, local models) with unified streaming interface.

## Key Concepts

### Server-Sent Events (SSE)

SSE is a standard HTTP-based protocol for server-to-client streaming. The server sends events over a long-lived HTTP connection with a specific format:

```
data: {"token": "Hello"}

data: {"token": " world"}

data: [DONE]
```

Each event is prefixed with `data: ` and separated by double newlines. SSE has built-in reconnection — if the connection drops, the browser automatically reconnects and can resume from where it left off using the `Last-Event-ID` header.

**Advantages over WebSocket**: Works with standard HTTP infrastructure (proxies, CDNs, load balancers), simpler to implement, automatic reconnection. **Disadvantages**: Unidirectional only (server to client), limited to text data, connection limits per domain in browsers (6 in HTTP/1.1).

### WebSocket

WebSocket provides full-duplex communication over a single TCP connection. After an initial HTTP upgrade handshake, both client and server can send messages at any time. This enables real-time chat where the user can send new messages while still receiving a streaming response.

Key considerations: WebSocket connections are stateful, which complicates horizontal scaling. You need sticky sessions or a pub/sub layer (Redis) to route messages to the correct server instance. WebSocket also doesn't have built-in reconnection — you must implement it client-side.

### Backpressure

When the LLM generates tokens faster than the client can consume them (slow network, overwhelmed browser), tokens queue up in server memory. Without backpressure handling, this leads to memory exhaustion.

Solutions: (1) **Bounded buffer**: Use an async queue with a max size. When full, pause token generation until the client catches up. (2) **Drop policy**: Drop intermediate tokens and send the latest one (acceptable for progress indicators, not for text). (3) **Flow control**: WebSocket has built-in flow control via TCP — when the client's receive buffer is full, TCP backpressure slows the sender.

### Async Generators in Python

Python's `async for` protocol is ideal for streaming. An async generator `yield`s tokens as they become available, and the consumer processes them at its own pace:

```python
async def stream_tokens():
    async for chunk in llm_stream():
        yield chunk.text

async for token in stream_tokens():
    await send_to_client(token)
```

This naturally handles backpressure — if `send_to_client` is slow, the generator pauses at the `yield` until the consumer is ready.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
fastapi==0.115.0
uvicorn[standard]==0.30.6
websockets==13.0.1
sse-starlette==2.1.3
anthropic==0.34.1
openai==1.46.0
redis==5.0.8
asyncio==3.4.3
```

### Step 2: LLM Streaming Provider

```python
# provider.py
import anthropic
import openai
from typing import AsyncIterator
from dataclasses import dataclass

@dataclass
class StreamChunk:
    text: str
    finish_reason: str = None  # None, "stop", "max_tokens"
    usage: dict = None  # token counts on final chunk

class LLMProvider:
    """Unified streaming interface for multiple LLM providers."""

    def __init__(self, provider: str = "anthropic"):
        self.provider = provider
        if provider == "anthropic":
            self.client = anthropic.AsyncAnthropic()
        elif provider == "openai":
            self.client = openai.AsyncOpenAI()

    async def stream(self, messages: list[dict],
                     model: str = None,
                     max_tokens: int = 4096,
                     temperature: float = 0.7) -> AsyncIterator[StreamChunk]:
        """Stream tokens from the LLM provider."""
        if self.provider == "anthropic":
            async for chunk in self._stream_anthropic(
                messages, model or "claude-sonnet-4-20250514",
                max_tokens, temperature
            ):
                yield chunk
        elif self.provider == "openai":
            async for chunk in self._stream_openai(
                messages, model or "gpt-4o-mini",
                max_tokens, temperature
            ):
                yield chunk

    async def _stream_anthropic(
        self, messages, model, max_tokens, temperature
    ) -> AsyncIterator[StreamChunk]:
        async with self.client.messages.stream(
            model=model,
            max_tokens=max_tokens,
            temperature=temperature,
            messages=messages,
        ) as stream:
            async for text in stream.text_stream:
                yield StreamChunk(text=text)

            # Final message with usage
            message = await stream.get_final_message()
            yield StreamChunk(
                text="",
                finish_reason=message.stop_reason,
                usage={
                    "input_tokens": message.usage.input_tokens,
                    "output_tokens": message.usage.output_tokens,
                },
            )

    async def _stream_openai(
        self, messages, model, max_tokens, temperature
    ) -> AsyncIterator[StreamChunk]:
        stream = await self.client.chat.completions.create(
            model=model,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
            stream=True,
            stream_options={"include_usage": True},
        )
        async for chunk in stream:
            if chunk.choices and chunk.choices[0].delta.content:
                yield StreamChunk(text=chunk.choices[0].delta.content)
            if chunk.choices and chunk.choices[0].finish_reason:
                yield StreamChunk(
                    text="",
                    finish_reason=chunk.choices[0].finish_reason,
                )
            if chunk.usage:
                yield StreamChunk(
                    text="",
                    usage={
                        "input_tokens": chunk.usage.prompt_tokens,
                        "output_tokens": chunk.usage.completion_tokens,
                    },
                )
```

### Step 3: SSE Endpoint

```python
# sse_handler.py
import json
import asyncio
from fastapi import FastAPI, Request, HTTPException
from sse_starlette.sse import EventSourceResponse
from pydantic import BaseModel

app = FastAPI()
provider = LLMProvider("anthropic")

class ChatRequest(BaseModel):
    messages: list[dict]
    model: str = "claude-sonnet-4-20250514"
    max_tokens: int = 4096
    temperature: float = 0.7

@app.post("/v1/chat/stream")
async def stream_chat(request: ChatRequest, http_request: Request):
    """SSE endpoint for streaming chat responses."""

    async def event_generator():
        try:
            full_text = ""
            async for chunk in provider.stream(
                messages=request.messages,
                model=request.model,
                max_tokens=request.max_tokens,
                temperature=request.temperature,
            ):
                # Check if client disconnected
                if await http_request.is_disconnected():
                    break

                if chunk.text:
                    full_text += chunk.text
                    yield {
                        "event": "token",
                        "data": json.dumps({
                            "text": chunk.text,
                            "full_text": full_text,
                        }),
                    }

                if chunk.finish_reason:
                    yield {
                        "event": "done",
                        "data": json.dumps({
                            "finish_reason": chunk.finish_reason,
                            "usage": chunk.usage,
                            "full_text": full_text,
                        }),
                    }

        except Exception as e:
            yield {
                "event": "error",
                "data": json.dumps({"error": str(e)}),
            }

    return EventSourceResponse(event_generator())
```

### Step 4: WebSocket Handler

```python
# ws_handler.py
import json
import asyncio
from fastapi import WebSocket, WebSocketDisconnect
from dataclasses import dataclass, field
from datetime import datetime

@dataclass
class Connection:
    websocket: WebSocket
    connected_at: datetime = field(default_factory=datetime.now)
    conversation_id: str = ""
    messages: list[dict] = field(default_factory=list)

class ConnectionManager:
    def __init__(self, max_connections: int = 100):
        self.active: dict[str, Connection] = {}
        self.max_connections = max_connections

    async def connect(self, websocket: WebSocket,
                      connection_id: str) -> Connection:
        if len(self.active) >= self.max_connections:
            await websocket.close(code=1013, reason="Max connections reached")
            raise ConnectionError("Max connections reached")

        await websocket.accept()
        conn = Connection(
            websocket=websocket, conversation_id=connection_id
        )
        self.active[connection_id] = conn
        return conn

    def disconnect(self, connection_id: str):
        self.active.pop(connection_id, None)

    async def send_json(self, connection_id: str, data: dict):
        conn = self.active.get(connection_id)
        if conn:
            await conn.websocket.send_json(data)

manager = ConnectionManager()

@app.websocket("/v1/chat/ws/{conversation_id}")
async def websocket_chat(websocket: WebSocket, conversation_id: str):
    """WebSocket endpoint for bidirectional chat streaming."""
    try:
        conn = await manager.connect(websocket, conversation_id)
    except ConnectionError:
        return

    try:
        # Send heartbeats in background
        heartbeat_task = asyncio.create_task(
            _heartbeat(websocket, conversation_id)
        )

        while True:
            # Receive message from client
            data = await websocket.receive_json()

            if data.get("type") == "message":
                conn.messages.append({
                    "role": "user",
                    "content": data["content"],
                })

                # Stream response
                full_text = ""
                async for chunk in provider.stream(
                    messages=conn.messages,
                    model=data.get("model", "claude-sonnet-4-20250514"),
                ):
                    if chunk.text:
                        full_text += chunk.text
                        await websocket.send_json({
                            "type": "token",
                            "text": chunk.text,
                        })

                    if chunk.finish_reason:
                        await websocket.send_json({
                            "type": "done",
                            "finish_reason": chunk.finish_reason,
                            "usage": chunk.usage,
                            "full_text": full_text,
                        })

                # Add assistant response to conversation
                conn.messages.append({
                    "role": "assistant",
                    "content": full_text,
                })

            elif data.get("type") == "ping":
                await websocket.send_json({"type": "pong"})

    except WebSocketDisconnect:
        pass
    finally:
        heartbeat_task.cancel()
        manager.disconnect(conversation_id)

async def _heartbeat(websocket: WebSocket, connection_id: str,
                     interval: int = 30):
    """Send periodic heartbeats to detect dead connections."""
    while True:
        await asyncio.sleep(interval)
        try:
            await websocket.send_json({"type": "heartbeat"})
        except Exception:
            manager.disconnect(connection_id)
            break
```

### Step 5: Client-Side SSE Consumer (JavaScript)

```javascript
// client.js — Browser-side SSE consumer
class StreamingChat {
    constructor(baseUrl) {
        this.baseUrl = baseUrl;
    }

    async streamMessage(messages, onToken, onDone, onError) {
        const response = await fetch(`${this.baseUrl}/v1/chat/stream`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ messages }),
        });

        if (!response.ok) {
            onError(new Error(`HTTP ${response.status}`));
            return;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop(); // Keep incomplete line in buffer

            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const data = JSON.parse(line.slice(6));
                    if (line.includes('"event":"token"') || data.text) {
                        onToken(data.text);
                    }
                }
                if (line.startsWith('event: done')) {
                    // Next data line has the final info
                }
            }
        }

        onDone();
    }
}
```

### Step 6: WebSocket Client with Reconnection

```javascript
// ws_client.js — Browser-side WebSocket with auto-reconnect
class WebSocketChat {
    constructor(url, conversationId) {
        this.url = `${url}/v1/chat/ws/${conversationId}`;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.handlers = { token: null, done: null, error: null };
        this.connect();
    }

    connect() {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.reconnectAttempts = 0;
        };

        this.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            switch (data.type) {
                case 'token':
                    this.handlers.token?.(data.text);
                    break;
                case 'done':
                    this.handlers.done?.(data);
                    break;
                case 'heartbeat':
                    // Server is alive, no action needed
                    break;
            }
        };

        this.ws.onclose = (event) => {
            if (!event.wasClean && this.reconnectAttempts < this.maxReconnectAttempts) {
                const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
                this.reconnectAttempts++;
                console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
                setTimeout(() => this.connect(), delay);
            }
        };

        this.ws.onerror = (error) => {
            this.handlers.error?.(error);
        };
    }

    send(content) {
        if (this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type: 'message', content }));
        }
    }

    on(event, handler) {
        this.handlers[event] = handler;
    }
}
```

## Testing & Measurement

### Performance Metrics

- **Time to First Token (TTFT)**: Latency from request to first streamed token. Should be <500ms for good UX.
- **Inter-Token Latency (ITL)**: Time between consecutive tokens. Should be <50ms for smooth text rendering.
- **Throughput**: Tokens per second per connection and total across all connections.
- **Connection stability**: Percentage of streams that complete without disconnection.

### Load Testing

```python
# test_streaming.py
import asyncio
import time
import httpx

async def measure_streaming_performance(url: str, n_requests: int = 10):
    """Measure TTFT and ITL for streaming endpoint."""
    results = []

    async with httpx.AsyncClient(timeout=60.0) as client:
        for _ in range(n_requests):
            start = time.time()
            first_token_time = None
            token_times = []

            async with client.stream(
                "POST", f"{url}/v1/chat/stream",
                json={"messages": [{"role": "user", "content": "Count to 20."}]},
            ) as response:
                async for line in response.aiter_lines():
                    if line.startswith("data:"):
                        now = time.time()
                        if first_token_time is None:
                            first_token_time = now - start
                        else:
                            token_times.append(now)

            if first_token_time:
                itl_values = [
                    token_times[i] - token_times[i-1]
                    for i in range(1, len(token_times))
                ] if len(token_times) > 1 else []

                results.append({
                    "ttft_ms": first_token_time * 1000,
                    "avg_itl_ms": (sum(itl_values) / len(itl_values) * 1000)
                        if itl_values else 0,
                    "total_tokens": len(token_times) + 1,
                })

    return results
```

### Testing Checklist

- Client disconnection mid-stream: server should stop generating and clean up resources.
- Concurrent streams: 50 simultaneous connections should not degrade individual stream quality.
- Network interruption: WebSocket should reconnect and resume conversation context.
- Malformed input: server should return proper error events, not crash.
- Memory stability: run 1000 streams sequentially and verify no memory leak.

## Interview Angles

### Q1: When would you choose SSE over WebSocket for streaming AI responses?

**Sample Answer:** SSE is the right default for most AI streaming use cases. It works over standard HTTP, goes through proxies and CDNs without special configuration, has built-in reconnection with Last-Event-ID for resumption, and is simpler to implement and debug. WebSocket is better when you need bidirectional streaming — for example, if the user can send new messages while a response is still streaming, or if you're building a real-time collaborative editor with AI suggestions. The tradeoff is that WebSocket requires more infrastructure (sticky sessions or pub/sub for scaling, explicit reconnection logic, health checks) but gives you full-duplex communication. In practice, most chat interfaces use SSE for the streaming response and regular HTTP POST for sending messages — the user rarely sends a new message before the current response finishes.

### Q2: How do you handle backpressure in a streaming system?

**Sample Answer:** Backpressure occurs when the producer (LLM) generates tokens faster than the consumer (client network) can handle. Three strategies: (1) Async generators with bounded queues — the generator pauses at `yield` when the consumer isn't ready, which is Python's natural flow control. This is my default approach. (2) Explicit buffering — use an `asyncio.Queue(maxsize=100)` between the LLM stream and the client writer. When the queue is full, the producer blocks, slowing down token consumption from the LLM API. (3) TCP-level flow control — for WebSocket, TCP backpressure naturally slows the sender when the receiver's buffer is full. For SSE, the HTTP response buffer serves the same purpose. The key insight is that most LLM APIs charge per output token regardless of consumption speed, so slowing down token generation doesn't waste money — it just increases wall-clock time. The real risk is per-connection memory: if you buffer 10,000 tokens per connection across 1,000 connections, that's significant memory.

### Q3: How do you scale a streaming AI service horizontally?

**Sample Answer:** SSE scales naturally with standard HTTP load balancing — each request is independent, and any server can handle any stream. WebSocket is harder because connections are stateful. I use three approaches: (1) Sticky sessions — the load balancer routes a client to the same server for the duration of the connection. Simple but creates hot spots if some connections are long-lived. (2) Redis pub/sub — any server can receive a client message, publish it to Redis, and the server holding the LLM stream picks it up. This decouples client connections from inference. (3) Shared-nothing with client state — the client sends its full conversation history with each reconnection, so any server can resume. This is the simplest and most resilient approach, at the cost of more bandwidth. For most AI applications, option 3 works best because conversations are small relative to inference cost, and stateless servers are much easier to operate.

### Q4: How do you handle errors and retries in streaming responses?

**Sample Answer:** Streaming errors are trickier than regular request errors because the response has already started — you can't return a different HTTP status code. I use structured error events: `{"event": "error", "data": {"code": "rate_limit", "message": "...", "retry_after": 5}}`. The client-side handler detects these error events and decides whether to retry. For transient errors (rate limits, timeouts), I implement server-side retry with exponential backoff before surfacing the error to the client. For permanent errors (invalid input, authentication failure), I surface immediately. A critical pattern is partial response recovery: if the LLM stream fails at token 500 of 1000, I save the partial response and surface it to the user with a "response incomplete" indicator, rather than discarding everything. For SSE, the `Last-Event-ID` header enables resumption — the server can pick up where it left off if the client reconnects within a timeout window.
