#!/usr/bin/env python3
"""CARS Baseline Benchmark Runner.

Runs smoke test prompts against GGUF models via llama-cli,
captures latency, peak memory, and token throughput.
Outputs structured JSON to results/.

Usage (on titan-pc):
    python3 benchmark.py
    python3 benchmark.py --models ~/models/Phi-3.5-mini-instruct-Q4_K_M.gguf
    python3 benchmark.py --prompts ../prompts/smoke-test.json
"""

import argparse
import json
import os
import platform
import re
import subprocess
import time
from datetime import datetime, timezone
from pathlib import Path

from scoring import score_result


SCRIPT_DIR = Path(__file__).parent
DEFAULT_PROMPTS = SCRIPT_DIR.parent / "prompts"
DEFAULT_MODELS_DIR = Path.home() / "models"
DEFAULT_RESULTS_DIR = SCRIPT_DIR.parent / "results"
LLAMA_CLI = Path.home() / "llama.cpp" / "build" / "bin" / "llama-cli"

CHAT_TEMPLATES = {
    "phi": {
        "prefix": "<|user|>\n",
        "suffix": "<|end|>\n<|assistant|>\n",
    },
    "qwen": {
        "prefix": "<|im_start|>user\n",
        "suffix": "<|im_end|>\n<|im_start|>assistant\n",
    },
}


def get_hardware_info() -> dict:
    info = {
        "hostname": platform.node(),
        "cpu": "unknown",
        "cores": os.cpu_count() or 0,
        "ram_gb": 0.0,
    }
    try:
        with open("/proc/cpuinfo") as f:
            for line in f:
                if line.startswith("model name"):
                    info["cpu"] = line.split(":")[1].strip()
                    break
    except OSError:
        pass
    try:
        with open("/proc/meminfo") as f:
            for line in f:
                if line.startswith("MemTotal"):
                    kb = int(line.split()[1])
                    info["ram_gb"] = round(kb / 1024 / 1024, 1)
                    break
    except OSError:
        pass
    return info


def detect_model_family(model_path: str) -> str:
    name = Path(model_path).name.lower()
    if "phi" in name:
        return "phi"
    if "qwen" in name:
        return "qwen"
    return "qwen"


def format_prompt(raw_prompt: str, model_family: str) -> str:
    tmpl = CHAT_TEMPLATES[model_family]
    return f"{tmpl['prefix']}{raw_prompt}{tmpl['suffix']}"


def clean_response(raw_output: str, model_family: str) -> str:
    """Extract the model's actual response from llama-cli output.

    Strips the llama.cpp banner (loading spinner, ASCII art, build info,
    echoed prompt) and trailing stats/exit lines, returning only the
    model's generated text.
    """
    if model_family == "qwen":
        marker = "<|im_start|>assistant"
    else:  # phi
        marker = "<|assistant|>"

    idx = raw_output.rfind(marker)
    if idx >= 0:
        response = raw_output[idx + len(marker):]
    else:
        response = raw_output

    # Remove character+backspace pairs (spinner animation)
    prev = None
    while prev != response:
        prev = response
        response = re.sub(r'[^\n]\x08', '', response)
    response = response.replace('\x08', '')

    # Strip stats line and exit message
    response = re.sub(r'\[\s*Prompt:.*?\]', '', response)
    response = re.sub(r'Exiting\.\.\.', '', response)

    return response.strip()


def get_model_size_gb(model_path: str) -> float:
    return round(os.path.getsize(model_path) / (1024**3), 3)


def poll_vram_mb() -> float:
    """Get current GPU VRAM usage in MB via nvidia-smi."""
    try:
        result = subprocess.run(
            ["nvidia-smi", "--query-gpu=memory.used", "--format=csv,noheader,nounits"],
            capture_output=True, text=True, timeout=5,
        )
        if result.returncode == 0:
            return float(result.stdout.strip().split("\n")[0])
    except (FileNotFoundError, subprocess.TimeoutExpired, ValueError):
        pass
    return 0.0


def run_prompt(model_path: str, prompt: str, max_tokens: int = 256, gpu: bool = False) -> dict:
    model_family = detect_model_family(model_path)
    formatted = format_prompt(prompt, model_family)

    cmd = [
        str(LLAMA_CLI),
        "-m", model_path,
        "-p", formatted,
        "-n", str(max_tokens),
        "-c", "2048",
        "--no-display-prompt",
        "-cnv", "--single-turn",
        "-e",
    ]

    start = time.perf_counter()
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    peak_rss_kb = 0
    peak_vram_mb = 0.0
    pid = proc.pid
    status_path = f"/proc/{pid}/status"

    while proc.poll() is None:
        try:
            with open(status_path) as f:
                for line in f:
                    if line.startswith("VmHWM:"):
                        hwm_kb = int(line.split()[1])
                        peak_rss_kb = max(peak_rss_kb, hwm_kb)
                        break
        except (OSError, ProcessLookupError):
            pass
        if gpu:
            vram = poll_vram_mb()
            peak_vram_mb = max(peak_vram_mb, vram)
        time.sleep(0.1)

    try:
        with open(status_path) as f:
            for line in f:
                if line.startswith("VmHWM:"):
                    hwm_kb = int(line.split()[1])
                    peak_rss_kb = max(peak_rss_kb, hwm_kb)
                    break
    except (OSError, ProcessLookupError):
        pass

    elapsed = time.perf_counter() - start
    stdout = proc.stdout.read().strip() if proc.stdout else ""
    stderr = proc.stderr.read().strip() if proc.stderr else ""

    tokens_per_second = 0.0
    # Parse stdout stats: [ Prompt: 72.7 t/s | Generation: 9.4 t/s ]
    stdout_stats = re.search(
        r'\[\s*Prompt:\s*([\d.]+)\s*t/s\s*\|\s*Generation:\s*([\d.]+)\s*t/s\s*\]',
        stdout,
    )
    if stdout_stats:
        tokens_per_second = float(stdout_stats.group(2))

    # Also try stderr (non-conversation mode)
    if tokens_per_second == 0.0:
        eval_match = re.search(
            r"eval time\s*=\s*[\d.]+\s*ms\s*/\s*(\d+)\s*tokens\s*\([\d.]+\s*ms per token,\s*([\d.]+)\s*tokens per second\)",
            stderr,
        )
        if eval_match:
            tokens_per_second = float(eval_match.group(2))

    cleaned = clean_response(stdout, model_family)

    return {
        "response": cleaned,
        "latency_s": round(elapsed, 2),
        "peak_ram_mb": round(peak_rss_kb / 1024, 1),
        "tokens_per_second": round(tokens_per_second, 2),
        "peak_vram_mb": round(peak_vram_mb, 1),
    }


def run_benchmark(model_path: str, prompts: list[dict], gpu: bool = False) -> dict:
    model_name = Path(model_path).stem
    model_size = get_model_size_gb(model_path)
    hardware = get_hardware_info()

    print(f"\n{'='*60}")
    print(f"Model: {model_name}")
    print(f"Size:  {model_size} GB")
    print(f"{'='*60}")

    results = []
    total_accuracy = 0.0

    for i, prompt_data in enumerate(prompts, 1):
        task = prompt_data["task"]
        prompt_id = prompt_data["id"]
        print(f"\n  [{i}/{len(prompts)}] {prompt_id} ({task})...")

        metrics = run_prompt(model_path, prompt_data["prompt"], gpu=gpu)
        accuracy = score_result(metrics["response"], prompt_data)
        total_accuracy += accuracy

        result = {
            "id": prompt_id,
            "task": task,
            "prompt": prompt_data["prompt"],
            "expected": prompt_data.get("expected_answer", ""),
            "response": metrics["response"],
            "accuracy": accuracy,
            "latency_s": metrics["latency_s"],
            "peak_ram_mb": metrics["peak_ram_mb"],
            "tokens_per_second": metrics["tokens_per_second"],
            "peak_vram_mb": metrics.get("peak_vram_mb", 0.0),
        }
        results.append(result)

        status = "PASS" if accuracy == 1.0 else "FAIL"
        print(f"         {status} | {metrics['latency_s']}s | "
              f"{metrics['peak_ram_mb']}MB RAM | "
              f"{metrics['tokens_per_second']} tok/s")

    avg_accuracy = total_accuracy / len(prompts) if prompts else 0.0
    avg_latency = sum(r["latency_s"] for r in results) / len(results) if results else 0.0
    avg_ram_mb = sum(r["peak_ram_mb"] for r in results) / len(results) if results else 0.0
    avg_ram_gb = avg_ram_mb / 1024

    cars_ram = round(avg_accuracy / (avg_ram_gb * avg_latency), 4) if (avg_ram_gb * avg_latency) > 0 else 0.0
    cars_size = round(avg_accuracy / (model_size * avg_latency), 4) if (model_size * avg_latency) > 0 else 0.0

    avg_vram_mb = sum(r.get("peak_vram_mb", 0) for r in results) / len(results) if results else 0.0

    summary = {
        "avg_accuracy": round(avg_accuracy, 3),
        "avg_latency_s": round(avg_latency, 2),
        "avg_peak_ram_mb": round(avg_ram_mb, 1),
        "avg_tokens_per_second": round(
            sum(r["tokens_per_second"] for r in results) / len(results), 2
        ) if results else 0.0,
    }
    if gpu:
        summary["avg_peak_vram_mb"] = round(avg_vram_mb, 1)

    output = {
        "model": model_name,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "hardware": hardware,
        "model_size_gb": model_size,
        "results": results,
        "summary": summary,
        "cars_ram": cars_ram,
        "cars_size": cars_size,
    }

    if gpu:
        avg_vram_gb = avg_vram_mb / 1024
        output["cars_vram"] = round(avg_accuracy / (avg_vram_gb * avg_latency), 4) if (avg_vram_gb * avg_latency) > 0 else 0.0

    categories = {}
    for r in results:
        cat = r["task"]
        if cat not in categories:
            categories[cat] = {"correct": 0, "total": 0}
        categories[cat]["total"] += 1
        categories[cat]["correct"] += r["accuracy"]

    output["category_accuracy"] = {
        cat: round(v["correct"] / v["total"], 3) if v["total"] > 0 else 0.0
        for cat, v in sorted(categories.items())
    }

    print(f"\n  Summary: accuracy={avg_accuracy:.1%} | "
          f"latency={avg_latency:.1f}s | RAM={avg_ram_mb:.0f}MB")
    print(f"  CARS_RAM={cars_ram} | CARS_Size={cars_size}")

    return output


def find_models(models_dir: Path) -> list[str]:
    if not models_dir.exists():
        return []
    return sorted(str(p) for p in models_dir.glob("*.gguf"))


def main():
    parser = argparse.ArgumentParser(description="CARS Baseline Benchmark Runner")
    parser.add_argument(
        "--models", nargs="*",
        help="Model file paths. Default: all .gguf in ~/models/",
    )
    parser.add_argument(
        "--prompts", type=str, default=str(DEFAULT_PROMPTS),
        help=f"Prompts JSON file. Default: {DEFAULT_PROMPTS}",
    )
    parser.add_argument(
        "--results-dir", type=str, default=str(DEFAULT_RESULTS_DIR),
        help=f"Results output directory. Default: {DEFAULT_RESULTS_DIR}",
    )
    parser.add_argument(
        "--max-tokens", type=int, default=256,
        help="Max tokens to generate per prompt. Default: 256",
    )
    parser.add_argument(
        "--gpu", action="store_true",
        help="Enable VRAM tracking via nvidia-smi",
    )
    args = parser.parse_args()

    if not LLAMA_CLI.exists():
        print(f"ERROR: llama-cli not found at {LLAMA_CLI}")
        print("Run setup-titan.sh first.")
        raise SystemExit(1)

    prompts_path = Path(args.prompts)
    if prompts_path.is_dir():
        prompt_files = sorted(prompts_path.glob("*.json"))
        if not prompt_files:
            print(f"ERROR: No .json files found in {prompts_path}")
            raise SystemExit(1)
        prompts = []
        for pf in prompt_files:
            with open(pf) as f:
                prompts.extend(json.load(f))
        print(f"Loaded {len(prompts)} prompts from {len(prompt_files)} files in {prompts_path}")
    elif prompts_path.is_file():
        with open(prompts_path) as f:
            prompts = json.load(f)
        print(f"Loaded {len(prompts)} prompts from {prompts_path}")
    else:
        print(f"ERROR: Prompts path not found: {prompts_path}")
        raise SystemExit(1)

    if args.models:
        model_paths = args.models
    else:
        model_paths = find_models(DEFAULT_MODELS_DIR)
    if not model_paths:
        print(f"ERROR: No .gguf models found in {DEFAULT_MODELS_DIR}")
        print("Run setup-titan.sh first.")
        raise SystemExit(1)
    print(f"Found {len(model_paths)} model(s)")

    results_dir = Path(args.results_dir)
    results_dir.mkdir(parents=True, exist_ok=True)
    date_str = datetime.now().strftime("%Y-%m-%d")

    all_results = []
    for model_path in model_paths:
        if not os.path.exists(model_path):
            print(f"WARNING: Model not found: {model_path}, skipping")
            continue
        result = run_benchmark(model_path, prompts, gpu=args.gpu)
        all_results.append(result)

        model_name = Path(model_path).stem
        out_file = results_dir / f"{date_str}-{model_name}.json"
        with open(out_file, "w") as f:
            json.dump(result, f, indent=2)
        print(f"\n  Saved: {out_file}")

    if len(all_results) > 1:
        print(f"\n{'='*60}")
        print("COMPARISON")
        print(f"{'='*60}")
        if args.gpu:
            print(f"{'Model':<40} {'Acc':>5} {'Lat':>6} {'RAM':>7} {'VRAM':>7} {'CARS_R':>7} {'CARS_S':>7} {'CARS_V':>7}")
            print("-" * 82)
            for r in all_results:
                vram_mb = r["summary"].get("avg_peak_vram_mb", 0.0)
                cars_vram = r.get("cars_vram", 0.0)
                print(f"{r['model']:<40} "
                      f"{r['summary']['avg_accuracy']:>5.1%} "
                      f"{r['summary']['avg_latency_s']:>5.1f}s "
                      f"{r['summary']['avg_peak_ram_mb']:>6.0f}M "
                      f"{vram_mb:>6.0f}M "
                      f"{r['cars_ram']:>7.4f} "
                      f"{r['cars_size']:>7.4f} "
                      f"{cars_vram:>7.4f}")
        else:
            print(f"{'Model':<40} {'Acc':>5} {'Lat':>6} {'RAM':>7} {'CARS_R':>7} {'CARS_S':>7}")
            print("-" * 73)
            for r in all_results:
                print(f"{r['model']:<40} "
                      f"{r['summary']['avg_accuracy']:>5.1%} "
                      f"{r['summary']['avg_latency_s']:>5.1f}s "
                      f"{r['summary']['avg_peak_ram_mb']:>6.0f}M "
                      f"{r['cars_ram']:>7.4f} "
                      f"{r['cars_size']:>7.4f}")


if __name__ == "__main__":
    main()
