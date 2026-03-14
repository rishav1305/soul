# LLM Evaluation Framework: Systematic Quality Assessment

## Overview

Evaluating LLMs is one of the most critical and nuanced challenges in applied AI. Unlike traditional ML where accuracy on a test set is definitive, LLM outputs are open-ended, context-dependent, and multidimensional — you need to assess factuality, coherence, helpfulness, safety, and task-specific quality simultaneously. A systematic evaluation framework lets you compare models, detect regressions, validate fine-tuning, and make informed deployment decisions.

This project builds a comprehensive evaluation pipeline: automated benchmarks for capability testing, LLM-as-judge for open-ended quality assessment, and human evaluation workflows for ground truth. These skills are essential for any team shipping LLM-powered products — without rigorous evaluation, you're deploying blind.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    Evaluation Framework                          │
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                │
│  │ Benchmark  │  │ LLM-as-    │  │ Human      │                │
│  │ Suite      │  │ Judge      │  │ Eval       │                │
│  │ (Auto)     │  │ (Semi-Auto)│  │ (Manual)   │                │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘                │
│        │               │               │                        │
│        ▼               ▼               ▼                        │
│  ┌─────────────────────────────────────────────┐                │
│  │            Results Aggregator               │                │
│  │  - Score normalization                      │                │
│  │  - Statistical significance testing         │                │
│  │  - Regression detection                     │                │
│  └──────────────────────┬──────────────────────┘                │
│                         │                                        │
│                         ▼                                        │
│  ┌─────────────────────────────────────────────┐                │
│  │            Report Generator                 │                │
│  │  - Model comparison dashboards              │                │
│  │  - Per-category breakdowns                  │                │
│  │  - Failure case analysis                    │                │
│  └─────────────────────────────────────────────┘                │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Benchmark Suite** — Runs models against standardized test sets (MMLU, HumanEval, GSM8K, TruthfulQA) with deterministic scoring.
- **LLM-as-Judge** — Uses a strong model (GPT-4, Claude) to evaluate open-ended responses on rubric-defined criteria like helpfulness, accuracy, and tone.
- **Human Eval** — Structured interface for human raters with inter-annotator agreement tracking and calibration.
- **Results Aggregator** — Normalizes scores across evaluation methods, runs significance tests, and detects regressions.
- **Report Generator** — Creates comparison reports with per-category breakdowns and failure analysis.

## Key Concepts

### Types of Evaluation

**Capability benchmarks** test specific skills: reasoning (GSM8K, ARC), knowledge (MMLU), coding (HumanEval, MBPP), truthfulness (TruthfulQA), and instruction following (IFEval). These give quantitative scores but don't capture overall user experience.

**Preference evaluation** compares two model responses head-to-head. Humans or a judge model picks the better response. This is more natural than absolute scoring and is how Chatbot Arena works. The ELO rating system aggregates pairwise comparisons into a global ranking.

**Task-specific evaluation** measures performance on your actual use case. If you're building a customer support bot, evaluate on your own support tickets with domain-specific rubrics. Generic benchmarks are necessary but not sufficient.

### LLM-as-Judge

Using a strong LLM to evaluate a weaker LLM's outputs has become the standard approach for scalable evaluation. Key design decisions:

**Reference-free vs Reference-based**: Reference-free judging asks "Is this response good?" while reference-based asks "Does this response match the gold answer?" Reference-free is more general; reference-based is more reliable when gold answers exist.

**Pointwise vs Pairwise**: Pointwise scoring rates a single response on a rubric (1-5 scale). Pairwise comparison picks the better of two responses. Pairwise has higher inter-rater agreement and avoids scale calibration issues, but requires O(n^2) comparisons for n models.

**Position Bias**: LLM judges tend to prefer the first response in a pair. Mitigate this by evaluating each pair twice with swapped positions and averaging. If the judge contradicts itself (picks A first, then B when swapped), mark as a tie.

**Self-Bias**: Models tend to prefer their own outputs. Never use a model as a judge for its own outputs unless you control for this bias.

### Statistical Rigor

When comparing model A vs model B, you need statistical significance. A 2% improvement on 100 examples is noise; a 2% improvement on 5000 examples may be real. Use:

- **Bootstrap confidence intervals**: Resample your test set 1000 times, compute the metric each time, and report the 95% confidence interval.
- **Paired tests**: When comparing two models on the same examples, use paired bootstrap or McNemar's test rather than independent tests.
- **Multiple comparison correction**: If you're comparing across many categories, apply Bonferroni or Holm-Bonferroni correction to avoid false discoveries.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
openai==1.46.0
anthropic==0.34.1
datasets==3.0.0
lm-eval==0.4.4
scipy==1.14.1
pandas==2.2.2
numpy==2.1.1
jinja2==3.1.4
```

### Step 2: Benchmark Runner

```python
import json
from dataclasses import dataclass, field
from typing import Callable

@dataclass
class BenchmarkResult:
    model_name: str
    benchmark: str
    score: float
    total: int
    correct: int
    per_category: dict = field(default_factory=dict)
    failures: list = field(default_factory=list)

class BenchmarkSuite:
    def __init__(self):
        self.benchmarks: dict[str, Callable] = {}

    def register(self, name: str):
        def decorator(func):
            self.benchmarks[name] = func
            return func
        return decorator

    def run_all(self, model_fn: Callable, benchmarks: list[str] = None
                ) -> list[BenchmarkResult]:
        targets = benchmarks or list(self.benchmarks.keys())
        results = []
        for name in targets:
            if name in self.benchmarks:
                result = self.benchmarks[name](model_fn)
                results.append(result)
        return results

suite = BenchmarkSuite()

@suite.register("mmlu_sample")
def eval_mmlu(model_fn: Callable) -> BenchmarkResult:
    """Evaluate on a sample of MMLU (Massive Multitask Language Understanding)."""
    from datasets import load_dataset
    ds = load_dataset("cais/mmlu", "all", split="test")
    # Sample for efficiency
    ds = ds.shuffle(seed=42).select(range(min(500, len(ds))))

    correct = 0
    total = 0
    per_category = {}
    failures = []
    choices = ["A", "B", "C", "D"]

    for item in ds:
        question = item["question"]
        options = "\n".join(
            f"{choices[i]}. {item['choices'][i]}" for i in range(4)
        )
        prompt = (
            f"Answer the following multiple-choice question. "
            f"Reply with ONLY the letter (A, B, C, or D).\n\n"
            f"Question: {question}\n{options}\n\nAnswer:"
        )
        response = model_fn(prompt).strip().upper()
        expected = choices[item["answer"]]

        cat = item.get("subject", "unknown")
        if cat not in per_category:
            per_category[cat] = {"correct": 0, "total": 0}
        per_category[cat]["total"] += 1

        if response.startswith(expected):
            correct += 1
            per_category[cat]["correct"] += 1
        else:
            failures.append({
                "question": question[:100],
                "expected": expected,
                "got": response[:10],
            })
        total += 1

    return BenchmarkResult(
        model_name="",
        benchmark="mmlu_sample",
        score=correct / total if total > 0 else 0,
        total=total,
        correct=correct,
        per_category=per_category,
        failures=failures[:20],
    )
```

### Step 3: LLM-as-Judge

```python
from openai import OpenAI
from dataclasses import dataclass

@dataclass
class JudgeResult:
    score: int  # 1-5
    reasoning: str
    criteria_scores: dict  # per-criterion breakdown

JUDGE_PROMPT = """You are an expert evaluator. Score the following AI response
on a scale of 1-5 for each criterion.

## Criteria
- **Accuracy** (1-5): Are all factual claims correct?
- **Completeness** (1-5): Does the response fully address the question?
- **Clarity** (1-5): Is the response well-organized and easy to understand?
- **Helpfulness** (1-5): Would this response help the user accomplish their goal?

## User Question
{question}

## AI Response
{response}

## Reference Answer (if available)
{reference}

Provide your evaluation in this exact JSON format:
{{
    "accuracy": <score>,
    "completeness": <score>,
    "clarity": <score>,
    "helpfulness": <score>,
    "overall": <score>,
    "reasoning": "<brief explanation of scores>"
}}"""

class LLMJudge:
    def __init__(self, judge_model: str = "gpt-4o"):
        self.client = OpenAI()
        self.judge_model = judge_model

    def evaluate(self, question: str, response: str,
                 reference: str = "N/A") -> JudgeResult:
        prompt = JUDGE_PROMPT.format(
            question=question, response=response, reference=reference
        )
        result = self.client.chat.completions.create(
            model=self.judge_model,
            messages=[{"role": "user", "content": prompt}],
            temperature=0,
            response_format={"type": "json_object"},
        )
        data = json.loads(result.choices[0].message.content)
        return JudgeResult(
            score=data["overall"],
            reasoning=data["reasoning"],
            criteria_scores={
                k: data[k] for k in
                ["accuracy", "completeness", "clarity", "helpfulness"]
            },
        )

    def pairwise_compare(self, question: str, response_a: str,
                         response_b: str) -> str:
        """Compare two responses, returning 'A', 'B', or 'tie'."""
        # Run comparison in both orders to detect position bias
        result_ab = self._compare_once(question, response_a, response_b)
        result_ba = self._compare_once(question, response_b, response_a)

        # If judge is consistent across orderings, trust the result
        if result_ab == "A" and result_ba == "B":
            return "A"
        elif result_ab == "B" and result_ba == "A":
            return "B"
        else:
            return "tie"  # Inconsistent = tie

    def _compare_once(self, question, first, second) -> str:
        prompt = (
            f"Compare these two responses to the question: {question}\n\n"
            f"Response A:\n{first}\n\nResponse B:\n{second}\n\n"
            f"Which is better? Reply with ONLY 'A' or 'B'."
        )
        result = self.client.chat.completions.create(
            model=self.judge_model,
            messages=[{"role": "user", "content": prompt}],
            temperature=0,
        )
        answer = result.choices[0].message.content.strip().upper()
        return "A" if "A" in answer else "B"
```

### Step 4: Statistical Analysis

```python
import numpy as np
from scipy import stats

def bootstrap_confidence_interval(scores: list[float], n_bootstrap: int = 1000,
                                   confidence: float = 0.95) -> tuple:
    """Compute bootstrap confidence interval for a metric."""
    scores = np.array(scores)
    bootstrap_means = []
    for _ in range(n_bootstrap):
        sample = np.random.choice(scores, size=len(scores), replace=True)
        bootstrap_means.append(np.mean(sample))

    bootstrap_means = np.array(bootstrap_means)
    alpha = (1 - confidence) / 2
    lower = np.percentile(bootstrap_means, alpha * 100)
    upper = np.percentile(bootstrap_means, (1 - alpha) * 100)
    return float(lower), float(np.mean(bootstrap_means)), float(upper)

def paired_comparison(scores_a: list[float], scores_b: list[float]
                      ) -> dict:
    """Test whether model A is significantly different from model B."""
    scores_a = np.array(scores_a)
    scores_b = np.array(scores_b)
    diff = scores_a - scores_b

    mean_diff = np.mean(diff)
    ci_low, ci_mean, ci_high = bootstrap_confidence_interval(diff.tolist())

    # Wilcoxon signed-rank test (non-parametric paired test)
    if np.any(diff != 0):
        stat, p_value = stats.wilcoxon(diff[diff != 0])
    else:
        stat, p_value = 0, 1.0

    return {
        "mean_difference": mean_diff,
        "ci_95": (ci_low, ci_high),
        "p_value": p_value,
        "significant": p_value < 0.05,
        "better_model": "A" if mean_diff > 0 else "B" if mean_diff < 0 else "tie",
    }
```

### Step 5: Evaluation Pipeline

```python
class EvaluationPipeline:
    def __init__(self):
        self.suite = BenchmarkSuite()
        self.judge = LLMJudge()

    def evaluate_model(self, model_fn: Callable, model_name: str,
                       test_set: list[dict]) -> dict:
        """Run full evaluation: benchmarks + LLM-as-judge."""
        # Run benchmarks
        benchmark_results = self.suite.run_all(model_fn)

        # Run LLM-as-judge on open-ended questions
        judge_scores = []
        for item in test_set:
            response = model_fn(item["question"])
            result = self.judge.evaluate(
                question=item["question"],
                response=response,
                reference=item.get("reference", "N/A"),
            )
            judge_scores.append({
                "question": item["question"],
                "response": response,
                "scores": result.criteria_scores,
                "overall": result.score,
                "reasoning": result.reasoning,
            })

        # Aggregate
        overall_scores = [s["overall"] for s in judge_scores]
        ci_low, ci_mean, ci_high = bootstrap_confidence_interval(overall_scores)

        return {
            "model_name": model_name,
            "benchmarks": {r.benchmark: r.score for r in benchmark_results},
            "judge_overall": ci_mean,
            "judge_ci_95": (ci_low, ci_high),
            "judge_details": judge_scores,
            "n_evaluated": len(judge_scores),
        }
```

## Testing & Evaluation

### Validating the Evaluator

Your evaluation framework itself needs validation:

- **Inter-rater agreement**: If you have human evaluations, compute Cohen's Kappa between LLM-judge scores and human scores. Kappa > 0.6 is acceptable; > 0.8 is good.
- **Position bias test**: Run 100 pairwise comparisons with swapped positions. If the judge picks the same position >60% regardless of content, position bias is a problem.
- **Self-consistency**: Run the same evaluation twice. Scores should agree >90% of the time at temperature=0.
- **Calibration**: Check that a judge score of 4 actually corresponds to better responses than a score of 3, using human verification on a sample.

### Metrics to Track

- **Benchmark scores** with confidence intervals for each category
- **Judge agreement rate** with human evaluations (if available)
- **Cost per evaluation** (API calls) for budgeting
- **Evaluation latency** for CI/CD integration feasibility

## Interview Angles

### Q1: How do you handle the fact that LLM-as-judge has known biases?

**Sample Answer:** LLM judges exhibit several biases: position bias (preferring the first response), verbosity bias (preferring longer responses), self-bias (preferring their own outputs), and style bias (preferring formal language). I mitigate position bias by evaluating each pair twice with swapped order and only accepting consistent verdicts — inconsistent comparisons become ties. For verbosity bias, I include explicit instructions in the judge prompt to "prefer concise responses when both are equally correct." Self-bias is avoided by never using a model to judge its own outputs. For style bias, I calibrate with human-annotated examples and adjust the rubric to explicitly state that informal but accurate responses should score as high as formal ones. Despite these mitigations, LLM judges are imperfect — I always validate with a human evaluation sample (at least 100 examples) to ensure the automated scores correlate with human preference.

### Q2: When would you use benchmarks vs LLM-as-judge vs human evaluation?

**Sample Answer:** I use all three in layers. Benchmarks (MMLU, HumanEval, GSM8K) are the first gate — they're fast, deterministic, and reproducible. If a model fails benchmarks, no point doing expensive evaluation. They test capabilities but not user experience. LLM-as-judge is the middle layer — it scales to thousands of examples at moderate cost ($10-50 per evaluation run), handles open-ended responses, and correlates well with human preference (typically 80-85% agreement). I use it for comparing model versions during development. Human evaluation is the gold standard for high-stakes decisions (production deployment, model selection). It's expensive and slow but catches issues that automated methods miss, like subtle factual errors or culturally inappropriate responses. The tradeoff is coverage vs depth: benchmarks cover breadth cheaply, human eval covers depth expensively, and LLM-as-judge is the practical middle ground.

### Q3: How do you design a good evaluation dataset?

**Sample Answer:** A good evaluation dataset has four properties: (1) Representative — it covers the actual distribution of queries your system will face in production, including edge cases and adversarial inputs. I sample from production logs (anonymized) when possible. (2) Balanced — if you have categories, each should have enough examples (at least 30) for meaningful per-category statistics. (3) Annotated — gold-standard answers are critical for reference-based evaluation. I use domain experts, not crowd workers, for specialized tasks. (4) Versioned — the eval set must be frozen and versioned. If you keep changing it, you can't track progress over time. I also include a "canary" subset of deliberately easy examples — if a model fails these, something is fundamentally broken. Size depends on the required precision: for 95% confidence intervals within +/- 2%, you need about 2,500 examples.

### Q4: How would you set up LLM evaluation in a CI/CD pipeline?

**Sample Answer:** I'd structure it in three tiers. Fast tier (runs on every PR, <5 min): a small deterministic benchmark (100-200 examples) checking for regressions on critical capabilities — if accuracy drops more than 2% from the baseline, the PR is blocked. Medium tier (runs nightly, 30-60 min): full benchmark suite plus LLM-as-judge on 500+ examples across all categories. Results are posted to a dashboard with trend charts. Slow tier (runs before release, hours): comprehensive evaluation including human review of 100+ examples, adversarial testing, and safety checks. The key design choice is determining regression thresholds — too tight and you get false alarms, too loose and regressions slip through. I use the bootstrap confidence interval approach: a change is a regression only if the lower bound of the new score's CI is below the upper bound of the baseline's CI.
