"""Tests for benchmark runner scoring and formatting logic."""
import json
import pytest
from pathlib import Path
import sys

# Add scripts to path so we can import benchmark
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
import benchmark


class TestDetectModelFamily:
    def test_phi_model(self):
        assert benchmark.detect_model_family("Phi-3.5-mini-instruct-Q4_K_M.gguf") == "phi"

    def test_qwen_model(self):
        assert benchmark.detect_model_family("qwen2.5-3b-instruct-q4_k_m.gguf") == "qwen"

    def test_unknown_defaults_to_qwen(self):
        assert benchmark.detect_model_family("some-random-model.gguf") == "qwen"

    def test_case_insensitive(self):
        assert benchmark.detect_model_family("PHI-3.5-MINI.gguf") == "phi"
        assert benchmark.detect_model_family("QWEN2.5-3B.gguf") == "qwen"


class TestFormatPrompt:
    def test_phi_template(self):
        result = benchmark.format_prompt("Hello", "phi")
        assert result == "<|user|>\nHello<|end|>\n<|assistant|>\n"

    def test_qwen_template(self):
        result = benchmark.format_prompt("Hello", "qwen")
        assert result == "<|im_start|>user\nHello<|im_end|>\n<|im_start|>assistant\n"


class TestCleanResponse:
    def test_strips_qwen_banner(self):
        raw = (
            "Loading model... \x08\n\nllama.cpp banner\n\n"
            "> <|im_start|>user\nHello<|im_end|>\n<|im_start|>assistant\n\n"
            "|\x08-\x08 \x08The answer is 42.\n\n"
            "[ Prompt: 70.0 t/s | Generation: 9.0 t/s ]\n\nExiting..."
        )
        assert benchmark.clean_response(raw, "qwen") == "The answer is 42."

    def test_strips_phi_banner(self):
        raw = (
            "Loading model...\n\nllama.cpp banner\n\n"
            "> <|user|>\nHello<|end|>\n<|assistant|>\n\n"
            "|\x08-\x08 \x08SPAM\n\n"
            "[ Prompt: 47.0 t/s | Generation: 11.0 t/s ]\n\nExiting..."
        )
        assert benchmark.clean_response(raw, "phi") == "SPAM"

    def test_no_marker_returns_stripped(self):
        raw = "Just some text"
        assert benchmark.clean_response(raw, "qwen") == "Just some text"

    def test_multiline_response_preserved(self):
        raw = (
            "<|im_start|>assistant\n\n"
            "Line 1\nLine 2\nLine 3\n\n"
            "[ Prompt: 70.0 t/s | Generation: 9.0 t/s ]\n\nExiting..."
        )
        assert benchmark.clean_response(raw, "qwen") == "Line 1\nLine 2\nLine 3"


class TestDirectoryLoading:
    def test_prompts_dir_has_json_files(self):
        prompts_dir = Path(__file__).parent.parent / "prompts"
        json_files = list(prompts_dir.glob("*.json"))
        assert len(json_files) >= 1


class TestSmokeTestPrompts:
    def test_prompts_file_valid_json(self):
        prompts_path = Path(__file__).parent.parent / "prompts" / "smoke-test.json"
        with open(prompts_path) as f:
            prompts = json.load(f)
        assert len(prompts) == 3

    def test_prompts_have_required_fields(self):
        prompts_path = Path(__file__).parent.parent / "prompts" / "smoke-test.json"
        with open(prompts_path) as f:
            prompts = json.load(f)
        for p in prompts:
            assert "id" in p
            assert "task" in p
            assert "prompt" in p
            assert "expected_answer" in p
            assert "scoring" in p

    def test_prompts_cover_three_tasks(self):
        prompts_path = Path(__file__).parent.parent / "prompts" / "smoke-test.json"
        with open(prompts_path) as f:
            prompts = json.load(f)
        tasks = {p["task"] for p in prompts}
        assert tasks == {"reasoning", "code", "classification"}
