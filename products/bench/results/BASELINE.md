# CARS Baseline Results — 2026-02-22

## Hardware

| Spec | Value |
|------|-------|
| Host | titan-pc |
| CPU | Intel Core i5-8400 (6 cores, 2.80 GHz, no HT) |
| RAM | 7.6 GB total (~5.7 GB available) |
| GPU | None (Intel UHD 630 iGPU only) |
| Disk | 841 GB free |
| OS | Ubuntu 24.04, kernel 6.14 |
| Inference | llama.cpp (build b8129-cacc371f9, CPU-only) |

## Models

| Model | Quantization | File Size |
|-------|-------------|-----------|
| Phi-3.5-mini-instruct | Q4_K_M | 2.23 GB |
| Qwen2.5-3B-instruct | Q4_K_M | 1.96 GB |

## Results

### Per-Task Breakdown

| Model | Task | Accuracy | Latency | Peak RAM | Gen Speed |
|-------|------|----------|---------|----------|-----------|
| Phi-3.5-mini | reasoning | FAIL | 19.4s | 4322 MB | 7.7 t/s |
| Phi-3.5-mini | code | PASS | 20.4s | 4325 MB | 7.7 t/s |
| Phi-3.5-mini | classification | PASS | 4.6s | 4327 MB | 11.5 t/s |
| Qwen2.5-3B | reasoning | FAIL | 20.0s | 3423 MB | 9.4 t/s |
| Qwen2.5-3B | code | PASS | 15.8s | 3427 MB | 9.3 t/s |
| Qwen2.5-3B | classification | PASS | 4.0s | 3430 MB | 13.9 t/s |

### Summary

| Model | Accuracy | Avg Latency | Avg RAM | Avg Gen Speed | CARS_RAM | CARS_Size |
|-------|----------|-------------|---------|---------------|----------|-----------|
| Phi-3.5-mini Q4_K_M | 66.7% | 14.8s | 4324 MB | 8.9 t/s | 0.0107 | 0.0202 |
| Qwen2.5-3B Q4_K_M | 66.7% | 13.2s | 3426 MB | 10.9 t/s | 0.0150 | 0.0257 |

### CARS Metric

```
CARS_RAM  = Accuracy / (Peak_RAM_GB x Latency_s)
CARS_Size = Accuracy / (Model_Size_GB x Latency_s)
```

Qwen2.5-3B wins on both CARS variants: 27% better CARS_Size, 40% better CARS_RAM. Advantages: smaller model (1.96 vs 2.23 GB), lower RAM usage (3.4 vs 4.2 GB), faster generation (10.9 vs 8.9 t/s).

## Observations

1. **Both models fail the reasoning trick question.** "All but 9 die" — both compute 17-9=8 instead of recognizing 9 survive. This is a known weakness of small quantized models on trick questions.

2. **Classification works after response cleaning.** Both correctly output "SPAM" but the original benchmark scored it as FAIL because llama-cli output included banner text. Fixed by adding `clean_response()`.

3. **Memory usage is high relative to model size.** Phi-3.5-mini (2.2 GB model) uses 4.3 GB RAM; Qwen2.5-3B (2.0 GB model) uses 3.4 GB. The KV cache and llama.cpp overhead add 1.4-2.1 GB.

4. **Generation speed is CPU-bound at 7-14 t/s.** Short responses (classification) generate faster. Longer responses (code, reasoning) sustain 7-10 t/s.

5. **Scoring methodology note:** `exact_match_number` checks if the last standalone number in the response matches the expected answer. This correctly catches models that mention the right number during reasoning but arrive at a wrong final answer.

## Benchmark Configuration

- Context size: 2048 tokens (limited from model defaults to fit in RAM)
- Max generation: 256 tokens
- Mode: `-cnv --single-turn` (single conversation turn, then exit)
- 3 prompts: reasoning, code generation, classification
