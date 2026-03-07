#!/usr/bin/env bash
set -euo pipefail

# CARS Baseline — titan-pc setup script
# Installs llama.cpp (CPU-only) and downloads GGUF models
# Run ON titan-pc: bash setup-titan.sh

LLAMA_DIR="$HOME/llama.cpp"
MODELS_DIR="$HOME/models"

echo "=== CARS Baseline Setup ==="
echo "Target: $(hostname) — $(uname -m)"
echo ""

# 1. Install build dependencies
echo "--- Installing build dependencies ---"
sudo apt-get update -qq
sudo apt-get install -y -qq cmake build-essential curl wget

# 2. Clone and build llama.cpp (CPU-only)
if [ -d "$LLAMA_DIR" ]; then
    echo "--- llama.cpp already cloned, pulling latest ---"
    cd "$LLAMA_DIR" && git pull
else
    echo "--- Cloning llama.cpp ---"
    git clone https://github.com/ggml-org/llama.cpp.git "$LLAMA_DIR"
fi

echo "--- Building llama.cpp (CPU-only) ---"
cd "$LLAMA_DIR"
cmake -B build -DGGML_CUDA=OFF -DGGML_METAL=OFF
cmake --build build --config Release -j$(nproc)

# Verify build
if [ ! -f "$LLAMA_DIR/build/bin/llama-cli" ]; then
    echo "ERROR: llama-cli not found after build"
    exit 1
fi
echo "llama-cli built: $LLAMA_DIR/build/bin/llama-cli"

# Add to PATH for this session
export PATH="$LLAMA_DIR/build/bin:$PATH"
echo ""
echo "Add to your .bashrc:"
echo "  export PATH=\"$LLAMA_DIR/build/bin:\$PATH\""
echo ""

# 3. Download models
mkdir -p "$MODELS_DIR"

# Phi-3.5-mini-instruct Q4_K_M (~2.4GB)
PHI_URL="https://huggingface.co/bartowski/Phi-3.5-mini-instruct-GGUF/resolve/main/Phi-3.5-mini-instruct-Q4_K_M.gguf"
PHI_FILE="$MODELS_DIR/Phi-3.5-mini-instruct-Q4_K_M.gguf"
if [ -f "$PHI_FILE" ]; then
    echo "--- Phi-3.5-mini already downloaded ---"
else
    echo "--- Downloading Phi-3.5-mini-instruct Q4_K_M (~2.4GB) ---"
    wget -q --show-progress -O "$PHI_FILE" "$PHI_URL"
fi

# Qwen2.5-3B-Instruct Q4_K_M (~2.0GB)
QWEN_URL="https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf"
QWEN_FILE="$MODELS_DIR/qwen2.5-3b-instruct-q4_k_m.gguf"
if [ -f "$QWEN_FILE" ]; then
    echo "--- Qwen2.5-3B already downloaded ---"
else
    echo "--- Downloading Qwen2.5-3B-Instruct Q4_K_M (~2.0GB) ---"
    wget -q --show-progress -O "$QWEN_FILE" "$QWEN_URL"
fi

# 4. Verify
echo ""
echo "=== Verification ==="
llama-cli --version 2>&1 || echo "(version flag may not be supported — checking binary exists)"
ls -lh "$MODELS_DIR"/*.gguf
echo ""
echo "=== Setup complete ==="
echo "Models directory: $MODELS_DIR"
echo "llama-cli: $LLAMA_DIR/build/bin/llama-cli"

# 5. Quick smoke test — run a tiny prompt to verify inference works
echo ""
echo "--- Quick inference smoke test ---"
echo "Running: 'What is 2+2?' with Phi-3.5-mini..."
llama-cli -m "$PHI_FILE" -p "<|user|>\nWhat is 2+2? Answer with just the number.<|end|>\n<|assistant|>\n" -n 32 --no-display-prompt 2>/dev/null
echo ""
echo "If you see a response above, inference is working."
