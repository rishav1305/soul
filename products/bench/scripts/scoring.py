"""Scoring methods for soul-bench.

Each scorer returns a float 0.0-1.0. The dispatch function
score_result() routes to the correct scorer based on prompt config.
"""

import json
import re


def score_json_schema(response: str, config: dict) -> float:
    """Score JSON output against schema requirements.

    Checks: (1) valid JSON parse, (2) required keys, (3) field values.
    Returns fraction of checks passed.
    """
    checks_passed = 0
    total_checks = 0

    total_checks += 1
    data = None
    try:
        data = json.loads(response.strip())
        checks_passed += 1
    except json.JSONDecodeError:
        match = re.search(r'```(?:json)?\s*\n(.*?)\n```', response, re.DOTALL)
        if match:
            try:
                data = json.loads(match.group(1))
                checks_passed += 1
            except json.JSONDecodeError:
                return 0.0
        else:
            return 0.0

    if config.get("is_array") and isinstance(data, list):
        total_checks += 1
        checks_passed += 1 if len(data) > 0 else 0

    required_keys = config.get("required_keys", [])
    for key in required_keys:
        total_checks += 1
        if isinstance(data, dict) and key in data:
            checks_passed += 1

    field_checks = config.get("field_checks", {})
    for field, allowed in field_checks.items():
        total_checks += 1
        if isinstance(data, dict):
            val = str(data.get(field, "")).lower()
            if val in [str(v).lower() for v in allowed]:
                checks_passed += 1

    return round(checks_passed / total_checks, 2) if total_checks > 0 else 0.0


def score_contains_keywords(response: str, config: dict) -> float:
    """Score based on presence of expected keywords (case-insensitive)."""
    keywords = config.get("keywords", [])
    if not keywords:
        return 0.0
    resp_lower = response.lower()
    matches = sum(1 for kw in keywords if kw.lower() in resp_lower)
    return round(matches / len(keywords), 2)


def score_code_executes(response: str, config: dict) -> float:
    """Score: code compiles + expected function present."""
    code = response
    match = re.search(r'```(?:python)?\s*\n(.*?)\n```', response, re.DOTALL)
    if match:
        code = match.group(1)

    try:
        compile(code, '<benchmark>', 'exec')
    except SyntaxError:
        return 0.0

    function_name = config.get("function_name", "")
    if function_name and f"def {function_name}" not in code:
        return 0.0

    return 1.0


def score_ordered_steps(response: str, config: dict) -> float:
    """Score step ordering against required sequence.

    Checks consecutive ordering constraints. Also verifies that the first
    required step actually appears earliest in the response among all required
    steps — if another required step appears before required_order[0], returns 0.0.
    """
    required_order = config.get("required_order", [])
    if len(required_order) < 2:
        return 0.0

    resp_lower = response.lower()
    positions = {}
    for step in required_order:
        idx = resp_lower.find(step.lower())
        positions[step] = idx if idx >= 0 else float('inf')

    # If the first required step is not the earliest to appear, order is wrong
    first_pos = positions[required_order[0]]
    if any(pos < first_pos for pos in positions.values()):
        return 0.0

    constraints_met = 0
    total = len(required_order) - 1
    for i in range(total):
        if positions[required_order[i]] < positions[required_order[i + 1]]:
            constraints_met += 1

    return round(constraints_met / total, 2) if total > 0 else 0.0


def score_exact_match_label(expected: str, response: str) -> float:
    """Exact string match after strip/lower."""
    return 1.0 if expected.lower() == response.lower().strip() else 0.0


def score_exact_match_number(expected: str, response: str) -> float:
    """Last standalone number in response matches expected."""
    numbers = re.findall(r'\b(\d+)\b', response)
    return 1.0 if numbers and numbers[-1] == expected else 0.0


def score_contains_function(expected: str, response: str) -> float:
    """Check if expected substring appears in response."""
    return 1.0 if expected in response else 0.0


def score_result(response: str, prompt_data: dict) -> float:
    """Dispatch to the correct scorer based on prompt config."""
    method = prompt_data.get("scoring", "")
    expected = prompt_data.get("expected_answer", "")
    config = prompt_data.get("scoring_config", {})

    if method == "json_schema":
        return score_json_schema(response, config)
    elif method == "contains_keywords":
        return score_contains_keywords(response, config)
    elif method == "code_executes":
        return score_code_executes(response, config)
    elif method == "ordered_steps":
        return score_ordered_steps(response, config)
    elif method == "exact_match_label":
        return score_exact_match_label(expected, response)
    elif method == "exact_match_number":
        return score_exact_match_number(expected, response)
    elif method == "contains_function":
        return score_contains_function(expected, response)
    return 0.0
