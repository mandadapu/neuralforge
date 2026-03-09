"""
review_rules.py — Safe rule evaluation for autopilot review automation.

Rules are evaluated using an explicit allowlist dispatch table.
Dynamic code execution (eval/exec) is never used.
"""

from __future__ import annotations

from typing import Any

# Allowlist of supported rule operators mapped to safe lambda functions.
# Each operator receives (field, value, payload) and returns a bool.
ALLOWED_RULE_OPERATORS: dict[str, Any] = {
    "equals": lambda field, value, payload: payload.get(field) == value,
    "not_equals": lambda field, value, payload: payload.get(field) != value,
    "contains": lambda field, value, payload: value in (payload.get(field) or ""),
    "not_contains": lambda field, value, payload: value not in (payload.get(field) or ""),
    "startswith": lambda field, value, payload: (payload.get(field) or "").startswith(value),
    "endswith": lambda field, value, payload: (payload.get(field) or "").endswith(value),
    "in": lambda field, value, payload: payload.get(field) in value,
    "not_in": lambda field, value, payload: payload.get(field) not in value,
}

_REQUIRED_RULE_KEYS = frozenset({"field", "operator", "value"})
_ALLOWED_RULE_KEYS = _REQUIRED_RULE_KEYS | frozenset({"description"})


def _validate_rule(rule: dict) -> None:
    """Validate that a rule dict contains only expected keys with string types."""
    if not isinstance(rule, dict):
        raise TypeError(f"Rule must be a dict, got {type(rule).__name__!r}")

    missing = _REQUIRED_RULE_KEYS - rule.keys()
    if missing:
        raise ValueError(f"Rule is missing required keys: {missing!r}")

    extra = rule.keys() - _ALLOWED_RULE_KEYS
    if extra:
        raise ValueError(f"Rule contains unexpected keys: {extra!r}")

    if not isinstance(rule["field"], str) or not rule["field"]:
        raise ValueError("Rule 'field' must be a non-empty string")

    if not isinstance(rule["operator"], str) or not rule["operator"]:
        raise ValueError("Rule 'operator' must be a non-empty string")


def evaluate_rule(rule: dict, payload: dict) -> bool:
    """
    Evaluate a single review rule against an event payload.

    Args:
        rule: A dict with keys 'field', 'operator', and 'value'.
              Optionally may include a 'description' key.
        payload: The event payload (e.g., a GitHub PR or issue payload).

    Returns:
        True if the rule matches the payload, False otherwise.

    Raises:
        TypeError: If rule is not a dict.
        ValueError: If the rule is malformed or uses an unsupported operator.
    """
    _validate_rule(rule)

    operator = rule["operator"]
    if operator not in ALLOWED_RULE_OPERATORS:
        raise ValueError(
            f"Unsupported rule operator: {operator!r}. "
            f"Allowed operators: {sorted(ALLOWED_RULE_OPERATORS)!r}"
        )

    field = rule["field"]
    value = rule["value"]

    return ALLOWED_RULE_OPERATORS[operator](field, value, payload)


def evaluate_rules(rules: list[dict], payload: dict, match_all: bool = True) -> bool:
    """
    Evaluate a list of review rules against a payload.

    Args:
        rules: List of rule dicts, each with 'field', 'operator', 'value'.
        payload: The event payload to evaluate against.
        match_all: If True (default), all rules must match (AND logic).
                   If False, any rule matching is sufficient (OR logic).

    Returns:
        True if the rules match according to the match_all strategy.
    """
    if not rules:
        return False

    results = (evaluate_rule(rule, payload) for rule in rules)

    if match_all:
        return all(results)
    return any(results)
