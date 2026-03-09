"""Autopilot review rules engine.

Rules are evaluated using an explicit if/elif chain.
Dynamic code execution (eval/exec/callable dispatch) is never used.
"""

from __future__ import annotations

# Allowlist of supported rule operator names.
_ALLOWED_OPERATORS: frozenset[str] = frozenset({
    "equals",
    "not_equals",
    "contains",
    "not_contains",
    "startswith",
    "endswith",
    "in",
    "not_in",
})


def _validate_rule(rule: dict) -> None:
    """Validate that a rule dict has the required keys and a supported operator.

    Raises:
        ValueError: if any required key is missing or the operator is unsupported.
    """
    for key in ("field", "operator", "value"):
        if key not in rule:
            raise ValueError(f"Rule is missing required key: {key!r}")

    operator = rule["operator"]
    if operator not in _ALLOWED_OPERATORS:
        raise ValueError(
            f"Unsupported rule operator: {operator!r}. "
            f"Allowed operators: {sorted(_ALLOWED_OPERATORS)!r}"
        )


def evaluate_rule(rule: dict, payload: dict) -> bool:
    """Evaluate a single rule against a payload dict.

    Args:
        rule: A dict with keys ``field``, ``operator``, and ``value``.
        payload: The event payload to evaluate against.

    Returns:
        True if the rule matches, False otherwise.

    Raises:
        ValueError: if the rule is invalid or uses an unsupported operator.
    """
    _validate_rule(rule)

    operator = rule["operator"]
    field = rule["field"]
    value = rule["value"]
    field_val = payload.get(field)

    if operator == "equals":
        return field_val == value
    elif operator == "not_equals":
        return field_val != value
    elif operator == "contains":
        return value in (field_val or "")
    elif operator == "not_contains":
        return value not in (field_val or "")
    elif operator == "startswith":
        return (field_val or "").startswith(value)
    elif operator == "endswith":
        return (field_val or "").endswith(value)
    elif operator == "in":
        return field_val in value
    elif operator == "not_in":
        return field_val not in value
    # Unreachable: operator was validated against _ALLOWED_OPERATORS above.
    raise AssertionError(f"Unhandled operator: {operator!r}")  # pragma: no cover


def evaluate_rules(rules: list[dict], payload: dict) -> bool:
    """Evaluate a list of rules against a payload dict.

    All rules must match (logical AND).

    Args:
        rules: A list of rule dicts, each with ``field``, ``operator``, ``value``.
        payload: The event payload to evaluate against.

    Returns:
        True if all rules match, False otherwise.
    """
    return all(evaluate_rule(rule, payload) for rule in rules)
