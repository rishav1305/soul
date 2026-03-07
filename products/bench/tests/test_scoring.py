# tests/test_scoring.py
"""Tests for scoring module."""
import pytest
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
import scoring


class TestJsonSchema:
    def test_valid_json_all_keys(self):
        config = {"required_keys": ["subject", "body"]}
        resp = '{"subject": "Hello", "body": "World"}'
        assert scoring.score_json_schema(resp, config) == 1.0

    def test_valid_json_missing_key(self):
        config = {"required_keys": ["subject", "body"]}
        resp = '{"subject": "Hello"}'
        assert scoring.score_json_schema(resp, config) == pytest.approx(0.67, abs=0.01)

    def test_invalid_json(self):
        config = {"required_keys": ["subject"]}
        assert scoring.score_json_schema("not json", config) == 0.0

    def test_json_in_code_fence(self):
        config = {"required_keys": ["subject"]}
        resp = '```json\n{"subject": "Hello"}\n```'
        assert scoring.score_json_schema(resp, config) == 1.0

    def test_field_value_check(self):
        config = {
            "required_keys": ["confidence"],
            "field_checks": {"confidence": ["high", "medium", "low"]},
        }
        resp = '{"confidence": "high"}'
        assert scoring.score_json_schema(resp, config) == 1.0

    def test_field_value_check_fail(self):
        config = {
            "required_keys": ["confidence"],
            "field_checks": {"confidence": ["high", "medium", "low"]},
        }
        resp = '{"confidence": "maybe"}'
        # JSON valid (1) + key present (1) + field check fail (0) = 2/3
        assert scoring.score_json_schema(resp, config) == pytest.approx(0.67, abs=0.01)

    def test_json_array(self):
        config = {"required_keys": [], "is_array": True}
        resp = '[{"task_id": 1, "agent": "system_agent"}]'
        assert scoring.score_json_schema(resp, config) >= 0.5


class TestContainsKeywords:
    def test_all_present(self):
        config = {"keywords": ["disk", "temperature", "memory"]}
        resp = "Disk is at 89%. Temperature normal. Memory at 62%."
        assert scoring.score_contains_keywords(resp, config) == 1.0

    def test_partial(self):
        config = {"keywords": ["disk", "temperature", "memory"]}
        resp = "Disk is at 89%."
        assert scoring.score_contains_keywords(resp, config) == pytest.approx(0.33, abs=0.01)

    def test_none_present(self):
        config = {"keywords": ["disk", "temperature"]}
        assert scoring.score_contains_keywords("hello world", config) == 0.0

    def test_case_insensitive(self):
        config = {"keywords": ["DISK"]}
        assert scoring.score_contains_keywords("disk usage high", config) == 1.0


class TestCodeExecutes:
    def test_valid_function(self):
        config = {"function_name": "is_even"}
        code = "def is_even(n):\n    return n % 2 == 0"
        assert scoring.score_code_executes(code, config) == 1.0

    def test_syntax_error(self):
        config = {"function_name": "foo"}
        assert scoring.score_code_executes("def foo(:\n  pass", config) == 0.0

    def test_missing_function(self):
        config = {"function_name": "bar"}
        assert scoring.score_code_executes("x = 1", config) == 0.0

    def test_code_in_fence(self):
        config = {"function_name": "add"}
        code = "```python\ndef add(a, b):\n    return a + b\n```"
        assert scoring.score_code_executes(code, config) == 1.0


class TestOrderedSteps:
    def test_correct_order(self):
        config = {"required_order": ["import", "enrich", "draft", "review", "send"]}
        resp = "1. import contacts\n2. enrich\n3. draft emails\n4. review\n5. send"
        assert scoring.score_ordered_steps(resp, config) == 1.0

    def test_wrong_order(self):
        config = {"required_order": ["import", "enrich", "draft"]}
        resp = "1. draft emails\n2. import contacts\n3. enrich"
        assert scoring.score_ordered_steps(resp, config) == 0.0

    def test_partial_order(self):
        config = {"required_order": ["import", "enrich", "draft", "send"]}
        resp = "1. import\n2. draft\n3. enrich\n4. send"
        # import < draft (ok), draft > enrich (fail), enrich < send (ok) = 2/3
        assert scoring.score_ordered_steps(resp, config) == pytest.approx(0.67, abs=0.01)

    def test_missing_step(self):
        config = {"required_order": ["import", "enrich", "draft"]}
        resp = "1. import\n2. draft"
        # enrich not found (inf), so import < inf (ok), inf > draft (fail) = 1/2
        assert scoring.score_ordered_steps(resp, config) == 0.5


class TestExistingMethods:
    def test_exact_match_label(self):
        assert scoring.score_exact_match_label("SPAM", "spam") == 1.0
        assert scoring.score_exact_match_label("SPAM", "NOT_SPAM") == 0.0

    def test_exact_match_number(self):
        assert scoring.score_exact_match_number("9", "The answer is 9.") == 1.0
        assert scoring.score_exact_match_number("9", "The answer is 8.") == 0.0

    def test_contains_function(self):
        assert scoring.score_contains_function("def foo", "def foo(x): pass") == 1.0
        assert scoring.score_contains_function("def foo", "no function") == 0.0


class TestScoreDispatch:
    def test_dispatches_json_schema(self):
        prompt = {
            "scoring": "json_schema",
            "expected_answer": "",
            "scoring_config": {"required_keys": ["a"]},
        }
        assert scoring.score_result('{"a": 1}', prompt) == 1.0

    def test_dispatches_exact_match_label(self):
        prompt = {"scoring": "exact_match_label", "expected_answer": "SPAM"}
        assert scoring.score_result("spam", prompt) == 1.0

    def test_unknown_method_returns_zero(self):
        prompt = {"scoring": "unknown", "expected_answer": "x"}
        assert scoring.score_result("x", prompt) == 0.0
