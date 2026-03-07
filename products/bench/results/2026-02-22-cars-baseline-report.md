# CARS Baseline Benchmark — Full Knowledge Transfer Report

## 1. What We Set Out To Do

**Goal:** Establish a baseline measurement of how small quantized LLMs perform on your CPU-only hardware (titan-pc), using the CARS metric you designed for the soul-bench project.

**Why:** Before you can evaluate whether a local model is "good enough" to replace Claude API calls (saving cost), you need hard numbers — accuracy, speed, memory usage — on your actual hardware. This baseline answers: "What can a 3B parameter model do on an i5 with 8GB RAM?"

---

## 2. The CARS Metric

```
CARS = Reasoning Accuracy / (Resource_Cost x Latency)
```

**What it measures:** Efficiency — how much accuracy you get per unit of resource and time. A model that's accurate but slow scores lower. A model that's fast but wrong also scores lower.

**Original formula** uses VRAM (GPU memory). Since titan-pc has no GPU, we adapted it into two variants:

| Variant | Formula | What it captures |
|---------|---------|-----------------|
| CARS_RAM | Accuracy / (Peak_RAM_GB x Latency_s) | Runtime efficiency — how much system memory the model actually consumes |
| CARS_Size | Accuracy / (Model_Size_GB x Latency_s) | Storage efficiency — how compact the model is relative to performance |

**Why two variants:** RAM usage includes KV cache and llama.cpp overhead (varies by context length, prompt size). Model size is fixed and reproducible. Both are useful signals.

**Impact on model selection:** Higher CARS = better. A model with 90% accuracy but 30s latency might score lower than one with 80% accuracy and 5s latency. This forces you to think about deployment constraints, not just benchmarks on paper.

---

## 3. Hardware — What We're Working With

| Spec | Value | Impact |
|------|-------|--------|
| CPU | Intel i5-8400 (6 cores, 2.80 GHz, no HT) | Limits inference speed to ~7-14 tokens/sec |
| RAM | 7.6 GB total (~5.7 GB available) | Constrains model size — anything over ~4GB GGUF is risky |
| GPU | None (Intel UHD 630 iGPU) | CPU-only inference, no CUDA/Metal acceleration |
| Disk | 841 GB free | Plenty for model storage |
| OS | Ubuntu 24.04, kernel 6.14 | |

**Key constraint:** 7.6 GB RAM means you can run Q4_K_M quantized models up to ~3B parameters. Anything larger (7B+) would OOM or swap heavily.

---

## 4. What Was Built (9 files, 797 lines)

### 4.1 `scripts/setup-titan.sh` — Automated Setup

Installs everything needed on titan-pc:
1. Installs build dependencies (`cmake`, `g++`)
2. Clones llama.cpp from GitHub
3. Compiles CPU-only (no CUDA flags)
4. Downloads two GGUF models from HuggingFace

### 4.2 `scripts/benchmark.py` — The Benchmark Runner (374 lines)

Core functions and what they do:

| Function | Purpose |
|----------|---------|
| `detect_model_family(path)` | Looks at filename to determine "phi" or "qwen" — needed for chat templates |
| `format_prompt(prompt, family)` | Wraps raw prompt in the model's chat template (each model has its own special tokens) |
| `clean_response(raw, family)` | Strips llama.cpp banner text from output, extracts just the model's answer |
| `run_prompt(model, prompt)` | Launches llama-cli subprocess, monitors /proc for peak RAM, captures timing and output |
| `score_result(response, prompt)` | Binary scoring (1.0 or 0.0) based on scoring type |
| `run_benchmark(model, prompts)` | Orchestrates all prompts, computes averages and CARS scores |

**How `run_prompt` measures RAM:** It polls `/proc/[pid]/status` for `VmHWM` (high water mark) every 100ms while the subprocess runs. This gives peak RSS — the maximum physical memory the process used.

**How it captures generation speed:** Parses the stdout stats line that llama-cli prints: `[ Prompt: 46.4 t/s | Generation: 7.7 t/s ]`

### 4.3 `prompts/smoke-test.json` — The 3 Benchmark Tasks

```json
[
  {
    "id": "reasoning-01",
    "task": "reasoning",
    "prompt": "A farmer has 17 sheep. All but 9 die. How many are left?
               Think step by step, then give the final answer as a single number.",
    "expected_answer": "9",
    "scoring": "exact_match_number"
  },
  {
    "id": "code-01",
    "task": "code",
    "prompt": "Write a Python function called `is_palindrome`...",
    "expected_answer": "def is_palindrome",
    "scoring": "contains_function"
  },
  {
    "id": "classification-01",
    "task": "classification",
    "prompt": "Classify this email as exactly one of: SPAM or NOT_SPAM...",
    "expected_answer": "SPAM",
    "scoring": "exact_match_label"
  }
]
```

### What Each Benchmark Tests and Why It Matters

**1. Reasoning (trick question):**
- Tests logical comprehension — "all but 9 die" means 9 survive, not 17-9=8
- **Why it matters:** If a model can't handle basic reading comprehension tricks, it will misinterpret user instructions in production. This is a litmus test for instruction-following quality.

**2. Code generation:**
- Tests ability to produce syntactically valid, functional Python
- **Why it matters:** If you want local models to generate code (tool use, script generation), they need to produce working functions, not just prose about functions.

**3. Classification (single-label):**
- Tests ability to follow strict output format — reply with just "SPAM", nothing else
- **Why it matters:** Structured output is critical for pipelines. If a model can't output a clean label, it can't be used in automated classification workflows without post-processing.

### Scoring Methods

| Method | How it works |
|--------|-------------|
| `exact_match_number` | Finds all standalone numbers (`\b\d+\b`) in response, checks if the **last** one matches expected |
| `contains_function` | Checks if `def is_palindrome` appears anywhere in response |
| `exact_match_label` | Strips whitespace, lowercases, checks exact string equality |

---

## 5. Models Tested

| Model | Parameters | Quantization | File Size | Why chosen |
|-------|-----------|-------------|-----------|------------|
| Phi-3.5-mini-instruct | 3.8B | Q4_K_M | 2.23 GB | Microsoft's best small model, strong reasoning reputation |
| Qwen2.5-3B-instruct | 3B | Q4_K_M | 1.96 GB | Alibaba's competitor, known for efficiency |

**Q4_K_M quantization:** 4-bit with k-quant mixed precision. Reduces model size ~4x from float16 while preserving most quality. The "K_M" means medium quality — balances size vs accuracy.

**Chat templates** — each model uses different special tokens:
- Phi: `<|user|>...<|end|><|assistant|>`
- Qwen: `<|im_start|>user...<|im_end|><|im_start|>assistant`

Using the wrong template means the model doesn't understand it's in a conversation. This is why `detect_model_family` and `format_prompt` exist.

---

## 6. Every Error Encountered and How It Was Fixed

### Error 1: SSH hostname resolution failure
```
ssh: Could not resolve hostname titan-pc
```
**Cause:** titan-pc wasn't in DNS or /etc/hosts.
**Fix:** Added to `~/.ssh/config`:
```
Host titan-pc
    HostName 192.168.0.113
    User rishav
    IdentityFile ~/.ssh/id_ed25519
```

### Error 2: Segfault (exit code 139)
```
Signal: SIGSEGV (segmentation fault)
```
**Cause:** Phi-3.5-mini has a default context window of 128k tokens. llama.cpp tried to allocate a KV cache for 128k context, which would need ~49 GB of RAM. On a 7.6 GB machine, this caused an out-of-memory segfault.
**Fix:** Added `-c 2048` flag to limit context to 2048 tokens. This makes the KV cache fit comfortably in RAM.
**Lesson:** Always set explicit context size when running models with large default context windows on constrained hardware.

### Error 3: Process hangs in interactive mode (attempt 1)
**Cause:** llama-cli enters an interactive conversation loop after generating a response, waiting for the next user input. Even with `--no-display-prompt`, it sits at the `>` prompt forever.
**First fix attempt:** Added `--no-conversation` flag. Still hung.
**Why it failed:** `--no-conversation` alone doesn't exit after generation in all modes.

### Error 4: Process still hangs (attempt 2)
**Final fix:** Used `-cnv --single-turn` — this tells llama-cli to enter conversation mode but exit after exactly one turn (one user message, one assistant response).
**Also added:** `stdin=subprocess.DEVNULL` in the subprocess call to ensure no stdin is available.
**Lesson:** llama-cli's flag behavior is not always intuitive. `-cnv --single-turn` is the reliable way to do single-shot inference in conversation mode.

### Error 5: Token counting shows 0
**Cause:** The benchmark was looking for eval stats in stderr (`eval time = ... ms / N tokens`), but in `-cnv` mode, llama-cli prints stats in stdout as `[ Prompt: X t/s | Generation: Y t/s ]`.
**Fix:** Added stdout stats parsing regex, falling back to stderr format:
```python
re.search(r'\[\s*Prompt:\s*([\d.]+)\s*t/s\s*\|\s*Generation:\s*([\d.]+)\s*t/s\s*\]', stdout)
```

### Error 6: Classification scored FAIL despite correct output
**Cause:** The captured response included the entire llama.cpp banner — ASCII art logo, build info, available commands, the echoed prompt, spinner characters, then "SPAM", then stats. The `exact_match_label` scorer compared this entire string against "SPAM" and failed.
**Fix:** Added `clean_response()` function that:
1. Finds the last `<|assistant|>` or `<|im_start|>assistant` marker
2. Extracts everything after it
3. Removes backspace-character spinner sequences
4. Strips the stats line and "Exiting..."
5. Returns just the model's generated text

### Error 7: Reasoning scored PASS despite wrong answer
**Cause:** Phi answered "8" (wrong) but the original scoring used `if "9" in response` — substring match. The number "9" appeared in the response because the model repeated "all but 9 die" in its reasoning steps.
**Fix:** Changed `exact_match_number` to use last-number matching:
```python
numbers = re.findall(r'\b(\d+)\b', response)
return 1.0 if numbers and numbers[-1] == expected else 0.0
```
This checks the final standalone number in the response — typically the answer the model converges on.

### Error 8: SCP directory copy failures
**Cause:** `scp -r scripts/ titan-pc:~/bench-scripts/` failed when the target directory didn't exist and choked on `__pycache__`.
**Fix:** Pre-created directories with `ssh titan-pc 'mkdir -p ...'` and copied specific files instead of entire directories.

### Error 9: pytest not installed on Pi
**Fix:** `sudo apt-get install -y python3-pytest`

---

## 7. Smoke Test Results — Detailed Breakdown

### Phi-3.5-mini-instruct Q4_K_M

**Reasoning (FAIL):**

The model correctly identifies "9 survive" in step 1, then contradicts itself by computing 17-9=8. The last number is 8, not 9. **Score: 0.0**

**Code (PASS):**
Generated a working `is_palindrome` function with docstring, proper case/space handling. **Score: 1.0**

**Classification (PASS):**
Output: `SPAM` — clean, single-word label matching the expected format exactly. **Score: 1.0**

| Metric | Reasoning | Code | Classification |
|--------|-----------|------|---------------|
| Accuracy | 0.0 | 1.0 | 1.0 |
| Latency | 19.4s | 20.4s | 4.6s |
| Peak RAM | 4322 MB | 4325 MB | 4327 MB |
| Gen Speed | 7.7 t/s | 7.7 t/s | 11.5 t/s |

### Qwen2.5-3B-instruct Q4_K_M

**Reasoning (FAIL):**

Same mistake as Phi — both models compute 17-9=8 instead of recognizing that 9 sheep remain. **Score: 0.0**

**Code (PASS):**
Generated a working `is_palindrome` with docstring, correct logic. **Score: 1.0**

**Classification (PASS):**
Output: `SPAM` — exact match. **Score: 1.0**

| Metric | Reasoning | Code | Classification |
|--------|-----------|------|---------------|
| Accuracy | 0.0 | 1.0 | 1.0 |
| Latency | 20.0s | 15.8s | 4.0s |
| Peak RAM | 3423 MB | 3427 MB | 3430 MB |
| Gen Speed | 9.4 t/s | 9.3 t/s | 13.9 t/s |

### Final Comparison

| Model | Accuracy | Avg Latency | Avg RAM | Avg Gen Speed | CARS_RAM | CARS_Size |
|-------|----------|-------------|---------|---------------|----------|-----------|
| Phi-3.5-mini | 66.7% | 14.8s | 4324 MB | 8.9 t/s | 0.0107 | 0.0202 |
| Qwen2.5-3B | 66.7% | 13.2s | 3426 MB | 10.9 t/s | 0.0150 | 0.0257 |

**Winner: Qwen2.5-3B** — 27% better CARS_Size, 40% better CARS_RAM. Same accuracy but faster, lighter, and smaller.

---

## 8. How To Reproduce This Yourself

### On any Linux machine:

```bash
# 1. Clone the repo, cd to soul-bench
cd ~/soul/soul-bench

# 2. Run setup (installs llama.cpp + downloads models)
bash scripts/setup-titan.sh

# 3. Run the benchmark
python3 scripts/benchmark.py \
  --prompts prompts/smoke-test.json \
  --results-dir results/

# 4. Results appear as JSON in results/
```

### To add a new model:
1. Download the GGUF file to `~/models/`
2. Run `python3 scripts/benchmark.py` — it auto-discovers all `.gguf` files

### To add new prompts:
Edit `prompts/smoke-test.json` — add entries with `id`, `task`, `prompt`, `expected_answer`, and `scoring` (one of: `exact_match_number`, `contains_function`, `exact_match_label`).

### To run tests:
```bash
python3 -m pytest tests/ -v  # 23 tests
```

---

## 9. What This Tells Us Going Forward

1. **3B models can't do trick reasoning** — both fail the "all but 9" question. This sets a quality floor. You'll need 7B+ or better prompting to get reasoning reliability.

2. **3B models CAN do structured output** — classification and code generation work well. For pipeline tasks (labeling, formatting, simple code), local models are viable.

3. **CPU inference is ~10 t/s** — roughly 1 paragraph every 5-10 seconds. Acceptable for batch processing, too slow for interactive chat.

4. **RAM overhead is ~1.5-2x model size** — a 2 GB model needs 3.4-4.3 GB RAM at runtime due to KV cache and llama.cpp overhead. Plan accordingly.

5. **The CARS deploy gate** (>=80% accuracy, no task below 70%, P95 latency <5s) — neither model passes. Both are at 66.7% with a 0% task. This means: these models are NOT ready for production deployment under your quality bar, but they establish the baseline to compare future models against.
