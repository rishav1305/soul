#!/usr/bin/env python3
"""Setup script for running soul-bench on Google Colab with T4 GPU.

Usage (in Colab cell):
    !python scripts/colab_setup.py
"""
import os
import subprocess
import sys


def run(cmd, **kwargs):
    """Run a command, printing it first."""
    print(f"$ {' '.join(cmd) if isinstance(cmd, list) else cmd}")
    subprocess.run(cmd, check=True, **kwargs)


def setup_llama_cpp():
    """Clone and build llama.cpp with CUDA support."""
    if not os.path.exists("llama.cpp"):
        run(["git", "clone", "--depth", "1", "https://github.com/ggerganov/llama.cpp.git"])

    os.makedirs("llama.cpp/build", exist_ok=True)
    run(["cmake", "-B", "build", "-DGGML_CUDA=ON"], cwd="llama.cpp")
    run(["cmake", "--build", "build", "--config", "Release", "-j"], cwd="llama.cpp")

    cli = "llama.cpp/build/bin/llama-cli"
    if os.path.exists(cli):
        print(f"llama-cli built successfully at {cli}")
    else:
        print("ERROR: llama-cli not found after build", file=sys.stderr)
        sys.exit(1)


def download_models():
    """Download GGUF models from HuggingFace."""
    os.makedirs("models", exist_ok=True)
    models = [
        (
            "https://huggingface.co/bartowski/Phi-3.5-mini-instruct-GGUF/resolve/main/Phi-3.5-mini-instruct-Q4_K_M.gguf",
            "models/Phi-3.5-mini-instruct-Q4_K_M.gguf",
        ),
        (
            "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf",
            "models/qwen2.5-3b-instruct-q4_k_m.gguf",
        ),
    ]
    for url, path in models:
        if os.path.exists(path):
            print(f"Already downloaded: {path}")
            continue
        print(f"Downloading {os.path.basename(path)}...")
        run(["wget", "-q", "--show-progress", "-O", path, url])
    print("All models ready.")


def verify():
    """Quick sanity check."""
    import shutil

    cli = "llama.cpp/build/bin/llama-cli"
    assert os.path.exists(cli), f"llama-cli not found at {cli}"

    model_count = len([f for f in os.listdir("models") if f.endswith(".gguf")])
    assert model_count >= 1, "No .gguf models found in models/"

    # Check GPU availability
    nvidia_smi = shutil.which("nvidia-smi")
    if nvidia_smi:
        result = subprocess.run(
            ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader"],
            capture_output=True, text=True,
        )
        print(f"GPU: {result.stdout.strip()}")
    else:
        print("WARNING: nvidia-smi not found — GPU acceleration unavailable")

    print(f"Setup complete: {model_count} model(s), llama-cli ready")


if __name__ == "__main__":
    setup_llama_cpp()
    download_models()
    verify()
