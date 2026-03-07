# soul-bench

## Overview
Benchmarking framework with CARS metric — Accuracy / (VRAM x Latency).

## Status
**To Implement** | Month 3-4 (Phase 7: Research Portfolio)

## Description
soul-bench provides a practical benchmarking framework for local model selection. The CARS (Cost-Adjusted Response Score) metric balances accuracy against resource consumption, making it useful for comparing models under real-world constraints.

## CARS Metric
```
CARS = Accuracy / (VRAM x Latency)
```
- **Accuracy**: Task-specific correctness score
- **VRAM**: GPU memory consumption
- **Latency**: Response time

## Key Features
- 10-task benchmark suite
- Claude baselines for comparison
- CARS metric computation
- Automated benchmark runner
- Results visualization
- Reproducible configurations

## Target Models for Comparison
- Qwen (various sizes)
- DeepSeek (various sizes)
- Phi (various sizes)
- Gemma (various sizes)

## Output
- Blog: "CARS: A Practical Metric for Local Model Selection"
- Reproducible benchmarks on GitHub
- Comparative results tables
- Interview talking point for research roles

## Portfolio Signal
Demonstrates quantitative research methodology — useful for Anthropic/Google research interviews.

## Timeline
Month 3-4, parallel with Phase 3 (Open-Core Premium).
