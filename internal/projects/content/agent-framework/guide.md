# Agent Framework: Multi-Agent Systems with Tool Use and Memory

## Overview

AI agents go beyond simple prompt-response patterns by combining LLMs with tool use, planning, memory, and autonomous decision-making. An agent framework orchestrates multiple specialized agents, each with their own capabilities and roles, to solve complex tasks that require multi-step reasoning, information gathering, and action execution.

This project builds a multi-agent framework from scratch: individual agents with tool use and memory, an orchestrator for agent coordination, and a planning system for task decomposition. These skills map directly to the fastest-growing area of applied AI — agent-based systems are being deployed for customer support, code generation, research automation, and workflow orchestration across every major tech company.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                     Agent Framework                              │
│                                                                  │
│  ┌─────────────────────────────────┐                            │
│  │         Orchestrator            │                            │
│  │  - Task decomposition           │                            │
│  │  - Agent routing                │                            │
│  │  - Result aggregation           │                            │
│  └───────┬──────────┬──────────┬───┘                            │
│          │          │          │                                  │
│    ┌─────▼───┐ ┌────▼────┐ ┌──▼──────┐                         │
│    │Research │ │Code     │ │Analysis │                          │
│    │Agent    │ │Agent    │ │Agent    │                          │
│    └────┬────┘ └────┬────┘ └────┬────┘                          │
│         │           │           │                                │
│    ┌────▼────────────▼───────────▼────┐                         │
│    │          Shared Services          │                         │
│    │  ┌────────┐ ┌────────┐ ┌───────┐ │                         │
│    │  │ Tools  │ │ Memory │ │ State │ │                         │
│    │  │ Registry│ │ Store │ │ Mgr   │ │                         │
│    │  └────────┘ └────────┘ └───────┘ │                         │
│    └──────────────────────────────────┘                         │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Orchestrator** — Receives high-level tasks, decomposes them into subtasks, routes subtasks to appropriate agents, and aggregates results.
- **Specialized Agents** — Each agent has a system prompt, a set of tools, and optionally its own memory. Agents are experts in specific domains.
- **Tool Registry** — Central registry of all available tools. Each tool has a name, description, parameter schema, and execution function. Agents discover tools at runtime.
- **Memory Store** — Persistent memory using ChromaDB for semantic search. Agents store and retrieve relevant context from past interactions.
- **State Manager** — Tracks the execution state of multi-step tasks: which steps are complete, what the intermediate results are, and whether the task should continue or stop.

## Key Concepts

### ReAct Pattern

ReAct (Reasoning + Acting) is the foundational pattern for tool-using agents. The agent alternates between reasoning (thinking about what to do) and acting (calling a tool). The loop is:

1. **Observe**: Receive the current state (user message + tool results)
2. **Think**: Reason about what information is needed and which tool to use
3. **Act**: Call a tool with specific parameters
4. **Observe**: Receive the tool's output
5. **Repeat** until the task is complete, then respond to the user

The key design decision is how to handle the thinking step. Some implementations use explicit chain-of-thought in the prompt ("Think step by step"). Others rely on the model's built-in reasoning. Explicit thinking is more reliable for complex tasks but uses more tokens.

### Tool Design Principles

Well-designed tools make or break an agent system:

- **Atomic**: Each tool does one thing. "search_and_summarize" should be two tools: "search" and "summarize."
- **Descriptive**: The tool's description must clearly explain what it does, what it returns, and when to use it. The LLM reads this to decide which tool to call.
- **Typed**: Parameter schemas with types and descriptions. Use Pydantic models for validation. The clearer the schema, the fewer tool call errors.
- **Idempotent**: Calling the same tool with the same parameters should produce the same result (where possible). This enables retry logic.
- **Error-informative**: Return clear error messages that help the agent recover. "File not found: report.pdf" is better than "Error."

### Memory Architecture

Agents need memory at three levels:

**Working memory** (conversation context): The current conversation history. Limited by context window size. This is the "RAM" — fast access, limited capacity.

**Episodic memory** (past interactions): Stored in a vector database. When a new query arrives, retrieve relevant past interactions. This is the "hard drive" — slower access, larger capacity. Useful for personalizing responses and avoiding repeated mistakes.

**Semantic memory** (knowledge): Facts, procedures, and domain knowledge stored as embeddings. Different from episodic in that it's abstracted knowledge, not raw interaction history.

### Multi-Agent Coordination

When multiple agents collaborate, coordination is critical:

- **Sequential**: Agent A's output is Agent B's input. Simple but no parallelism.
- **Parallel**: Multiple agents work independently on different subtasks. Results are merged. Fast but requires independent subtasks.
- **Debate**: Two agents propose solutions, then critique each other. Improves quality through adversarial review.
- **Hierarchical**: A manager agent delegates to worker agents, reviews their output, and provides feedback for revision.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
anthropic==0.34.1
openai==1.46.0
pydantic==2.9.1
chromadb==0.5.5
httpx==0.27.2
```

### Step 2: Tool System

```python
# tools.py
from pydantic import BaseModel, Field
from typing import Any, Callable
import json
import httpx

class ToolParameter(BaseModel):
    name: str
    type: str
    description: str
    required: bool = True

class Tool(BaseModel):
    name: str
    description: str
    parameters: list[ToolParameter]
    function: Callable = Field(exclude=True)

    def to_api_schema(self) -> dict:
        """Convert to Anthropic/OpenAI tool schema format."""
        properties = {}
        required = []
        for param in self.parameters:
            properties[param.name] = {
                "type": param.type,
                "description": param.description,
            }
            if param.required:
                required.append(param.name)

        return {
            "name": self.name,
            "description": self.description,
            "input_schema": {
                "type": "object",
                "properties": properties,
                "required": required,
            },
        }

    async def run(self, **kwargs) -> str:
        """Run the tool and return result as string."""
        try:
            result = self.function(**kwargs)
            if isinstance(result, str):
                return result
            return json.dumps(result, indent=2, default=str)
        except Exception as e:
            return json.dumps({"error": str(e)})

class ToolRegistry:
    def __init__(self):
        self.tools: dict[str, Tool] = {}

    def register(self, tool: Tool):
        self.tools[tool.name] = tool

    def get(self, name: str) -> Tool:
        if name not in self.tools:
            raise KeyError(f"Tool not found: {name}")
        return self.tools[name]

    def list_schemas(self) -> list[dict]:
        return [tool.to_api_schema() for tool in self.tools.values()]

# --- Built-in Tools ---
def web_search(query: str) -> dict:
    """Search the web for information."""
    response = httpx.get(
        "https://api.duckduckgo.com/",
        params={"q": query, "format": "json"},
        timeout=10.0,
    )
    data = response.json()
    results = []
    for item in data.get("RelatedTopics", [])[:5]:
        if "Text" in item:
            results.append({"text": item["Text"], "url": item.get("FirstURL", "")})
    return {"query": query, "results": results}

def calculate(expression: str) -> dict:
    """Safely perform a mathematical calculation using ast.literal_eval."""
    import ast
    try:
        # Only allows literal expressions - no function calls or imports
        result = ast.literal_eval(expression)
    except (ValueError, SyntaxError):
        # For arithmetic, parse manually
        import operator
        ops = {"+": operator.add, "-": operator.sub,
               "*": operator.mul, "/": operator.truediv}
        # Simple two-operand arithmetic
        for op_char, op_func in ops.items():
            if op_char in expression:
                parts = expression.split(op_char, 1)
                try:
                    a, b = float(parts[0].strip()), float(parts[1].strip())
                    result = op_func(a, b)
                    break
                except (ValueError, IndexError):
                    continue
        else:
            return {"expression": expression, "error": "Cannot parse expression"}
    return {"expression": expression, "result": result}

# Register built-in tools
registry = ToolRegistry()
registry.register(Tool(
    name="web_search",
    description="Search the web for current information on any topic.",
    parameters=[
        ToolParameter(name="query", type="string",
                     description="The search query"),
    ],
    function=web_search,
))
registry.register(Tool(
    name="calculate",
    description="Perform mathematical calculations. Input is a math expression.",
    parameters=[
        ToolParameter(name="expression", type="string",
                     description="Mathematical expression to compute"),
    ],
    function=calculate,
))
```

### Step 3: Agent with Memory

```python
# agent.py
import anthropic
import chromadb
from typing import Optional

class AgentMemory:
    def __init__(self, collection_name: str = "agent_memory"):
        self.client = chromadb.Client()
        self.collection = self.client.get_or_create_collection(collection_name)
        self._id_counter = 0

    def store(self, text: str, metadata: dict = None):
        self._id_counter += 1
        self.collection.add(
            documents=[text],
            metadatas=[metadata or {}],
            ids=[f"mem_{self._id_counter}"],
        )

    def retrieve(self, query: str, n_results: int = 5) -> list[str]:
        if self.collection.count() == 0:
            return []
        results = self.collection.query(
            query_texts=[query],
            n_results=min(n_results, self.collection.count()),
        )
        return results["documents"][0] if results["documents"] else []

class Agent:
    def __init__(self, name: str, system_prompt: str,
                 tools: list[Tool], model: str = "claude-sonnet-4-20250514",
                 memory: Optional[AgentMemory] = None):
        self.name = name
        self.system_prompt = system_prompt
        self.tools = {t.name: t for t in tools}
        self.model = model
        self.memory = memory
        self.client = anthropic.Anthropic()
        self.conversation: list[dict] = []
        self.max_iterations = 10

    async def invoke(self, user_message: str) -> str:
        """Run the agent loop: reason -> act -> observe -> repeat."""
        # Retrieve relevant memories
        context = ""
        if self.memory:
            memories = self.memory.retrieve(user_message)
            if memories:
                context = "\n\nRelevant context from memory:\n"
                context += "\n".join(f"- {m}" for m in memories)

        system = self.system_prompt + context
        self.conversation.append({"role": "user", "content": user_message})

        tool_schemas = [t.to_api_schema() for t in self.tools.values()]

        for iteration in range(self.max_iterations):
            response = self.client.messages.create(
                model=self.model,
                max_tokens=4096,
                system=system,
                tools=tool_schemas,
                messages=self.conversation,
            )

            # Check if agent wants to use a tool
            if response.stop_reason == "tool_use":
                # Process all tool calls in the response
                assistant_content = response.content
                self.conversation.append({
                    "role": "assistant",
                    "content": assistant_content,
                })

                tool_results = []
                for block in assistant_content:
                    if block.type == "tool_use":
                        tool = self.tools.get(block.name)
                        if tool:
                            result = await tool.run(**block.input)
                        else:
                            result = json.dumps(
                                {"error": f"Unknown tool: {block.name}"}
                            )

                        tool_results.append({
                            "type": "tool_result",
                            "tool_use_id": block.id,
                            "content": result,
                        })

                self.conversation.append({
                    "role": "user",
                    "content": tool_results,
                })
            else:
                # Agent is done — extract text response
                text_parts = [
                    block.text for block in response.content
                    if hasattr(block, "text")
                ]
                final_response = "\n".join(text_parts)

                # Store interaction in memory
                if self.memory:
                    self.memory.store(
                        f"Q: {user_message}\nA: {final_response[:500]}",
                        metadata={"agent": self.name},
                    )

                return final_response

        return "Max iterations reached without completing the task."
```

### Step 4: Orchestrator

```python
# orchestrator.py
from pydantic import BaseModel
import json
import re

class SubTask(BaseModel):
    id: int
    description: str
    agent: str  # which agent should handle this
    dependencies: list[int] = []  # IDs of subtasks that must complete first
    result: str = ""
    status: str = "pending"  # pending, running, completed, failed

class Orchestrator:
    def __init__(self, agents: dict[str, Agent]):
        self.agents = agents
        self.planner = anthropic.Anthropic()

    async def plan(self, task: str) -> list[SubTask]:
        """Decompose a task into subtasks with agent assignments."""
        agent_descriptions = "\n".join(
            f"- {name}: {agent.system_prompt[:100]}..."
            for name, agent in self.agents.items()
        )

        response = self.planner.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=2048,
            messages=[{"role": "user", "content": f"""
Decompose this task into subtasks. Available agents:
{agent_descriptions}

Task: {task}

Return a JSON array of subtasks:
[{{"id": 1, "description": "...", "agent": "agent_name", "dependencies": []}}]

Rules:
- Each subtask should be completable by a single agent
- Use dependencies to express ordering constraints
- Keep subtasks focused and atomic
"""}],
        )

        # Parse the plan
        text = response.content[0].text
        json_match = re.search(r'\[.*\]', text, re.DOTALL)
        if json_match:
            subtasks_data = json.loads(json_match.group())
            return [SubTask(**st) for st in subtasks_data]
        return []

    async def run(self, task: str) -> str:
        """Plan and execute a complex task using multiple agents."""
        subtasks = await self.plan(task)
        completed = {}

        while any(st.status == "pending" for st in subtasks):
            # Find subtasks whose dependencies are all completed
            ready = [
                st for st in subtasks
                if st.status == "pending"
                and all(dep in completed for dep in st.dependencies)
            ]

            if not ready:
                break  # Deadlock or all done

            for subtask in ready:
                subtask.status = "running"
                agent = self.agents.get(subtask.agent)
                if not agent:
                    subtask.status = "failed"
                    subtask.result = f"Agent not found: {subtask.agent}"
                    continue

                # Include dependency results as context
                context = subtask.description
                for dep_id in subtask.dependencies:
                    dep_result = completed.get(dep_id, "")
                    context += f"\n\nResult from previous step: {dep_result}"

                try:
                    result = await agent.invoke(context)
                    subtask.result = result
                    subtask.status = "completed"
                    completed[subtask.id] = result
                except Exception as e:
                    subtask.status = "failed"
                    subtask.result = str(e)

        # Aggregate results
        summary_parts = []
        for st in subtasks:
            summary_parts.append(
                f"Step {st.id} ({st.status}): {st.description}\n"
                f"Result: {st.result[:500]}"
            )
        return "\n\n---\n\n".join(summary_parts)
```

### Step 5: Putting It Together

```python
# main.py
import asyncio

async def main():
    # Create specialized agents
    research_agent = Agent(
        name="researcher",
        system_prompt=(
            "You are a research agent. Your job is to find accurate, "
            "up-to-date information using web search. Always cite sources. "
            "Be thorough but concise."
        ),
        tools=[registry.get("web_search")],
        memory=AgentMemory("research_memory"),
    )

    analysis_agent = Agent(
        name="analyst",
        system_prompt=(
            "You are a data analysis agent. Your job is to analyze "
            "information, compute statistics, identify patterns, and "
            "provide structured insights. Use calculations when needed."
        ),
        tools=[registry.get("calculate")],
        memory=AgentMemory("analysis_memory"),
    )

    # Create orchestrator
    orchestrator = Orchestrator({
        "researcher": research_agent,
        "analyst": analysis_agent,
    })

    # Run a complex task
    result = await orchestrator.run(
        "Research the current market size of the AI agent framework "
        "market and analyze growth trends. Provide specific numbers."
    )
    print(result)

if __name__ == "__main__":
    asyncio.run(main())
```

## Testing & Measurement

### Agent Quality Metrics

- **Task completion rate**: Percentage of tasks the agent successfully completes. Start with a benchmark set of 50 tasks across difficulty levels.
- **Tool selection accuracy**: Does the agent choose the right tool? Track tool calls per task and compare to the optimal tool sequence.
- **Iteration efficiency**: Fewer iterations for the same task quality indicates better reasoning. Track average iterations per task type.
- **Error recovery rate**: When a tool fails, does the agent recover gracefully? Test with intentionally failing tools.

### Testing Strategy

```python
async def test_agent_tool_selection():
    """Verify agent selects correct tools for different query types."""
    test_cases = [
        {"query": "What's 15% of 340?", "expected_tool": "calculate"},
        {"query": "Latest news about AI", "expected_tool": "web_search"},
    ]
    for case in test_cases:
        result = await agent.invoke(case["query"])
        assert case["expected_tool"] in agent.last_tools_used
```

## Interview Angles

### Q1: How do you prevent an agent from getting stuck in an infinite loop?

**Sample Answer:** Multiple safeguards: (1) Hard iteration limit — I set max_iterations=10 and return a partial result if exceeded. (2) Repetition detection — if the agent calls the same tool with the same parameters twice, I inject a message saying "You already tried this, try a different approach." (3) Budget tracking — limit total token usage per task. If the agent is consuming tokens without progress, cut it off. (4) Timeout — overall task timeout of 60 seconds. (5) Diminishing returns check — if the last 3 tool calls didn't add new information to the context, the agent should synthesize what it has and respond. The tradeoff is between letting the agent explore thoroughly and preventing runaway costs. I start strict (5 iterations) and increase only if completion rate is too low.

### Q2: How would you handle tool errors in an agent system?

**Sample Answer:** I implement a three-tier error handling strategy. Tier 1 (tool level): each tool returns structured errors with error codes and messages. "Not found" is different from "service unavailable" — the agent can reason about the difference. Tier 2 (agent level): the agent sees the error in its context and decides whether to retry, use a different tool, or ask the user for clarification. I add explicit instructions in the system prompt about common error patterns and recovery strategies. Tier 3 (orchestrator level): if an agent fails a subtask, the orchestrator can reassign it to a different agent or decompose it further. The key design decision is where to put retry logic — I prefer retries at the tool level (with exponential backoff for transient errors) and strategic recovery at the agent level (choosing alternative approaches for permanent errors).

### Q3: When should you use a single agent vs multiple agents?

**Sample Answer:** Single agent when the task requires one type of expertise, the tool set is small (<10 tools), and the task is straightforward. Multi-agent when: (1) tasks span different domains that need different system prompts and tool sets — a research agent and a coding agent think differently. (2) You need adversarial review — one agent generates, another critiques. (3) Parallel execution is possible — independent subtasks can run simultaneously across agents. The tradeoff is complexity. Multi-agent systems are harder to debug (which agent made the mistake?), cost more (multiple LLM calls), and require careful orchestration. I start with a single agent and split into multiple only when the single agent's quality degrades due to too many tools (>15 tools cause selection confusion) or too broad a system prompt.

### Q4: How do you design effective agent memory systems?

**Sample Answer:** I use a three-tier memory architecture. Working memory is the conversation context — I manage it carefully, summarizing old messages when the context gets long rather than truncating, because truncation loses important early context. Episodic memory stores past interactions in ChromaDB with metadata (timestamp, task type, success/failure). Before each new task, I retrieve the 3-5 most relevant past interactions to provide context and avoid repeating mistakes. Semantic memory stores extracted knowledge — facts, procedures, user preferences — as structured entries rather than raw conversation logs. The key tradeoff is retrieval quality vs storage cost. I store concise summaries rather than full conversations, and I periodically consolidate episodic memories into semantic knowledge (e.g., "User prefers Python over JavaScript" extracted from 10 interactions).
