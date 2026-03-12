"""Autopilot review rules engine.

Rules are evaluated using an explicit if/elif chain.
Dynamic code execution (eval/exec/callable dispatch) is never used.

Security and compliance notes:
- All rule evaluation outcomes are logged via the standard ``logging`` module
  to support audit trail requirements (GDPR Art. 15, HIPAA §164.312(b), SOX).
- Callers are responsible for restricting which fields and payloads are passed
  to this engine; this module does not enforce a field-level allowlist because
  the set of valid fields is context-dependent.  Apply field allowlisting at
  the call site before invoking ``evaluate_rules``/``evaluate_rule``.
- String operators validate that the payload field value is a ``str`` before
  operating on it, preventing type-confusion exceptions that could expose data.
"""

from __future__ import annotations

import logging

_logger = logging.getLogger(__name__)

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

    # Validate that field_val is a str before performing string operations to
    # prevent TypeError exceptions that could inadvertently expose payload data.
    _STRING_OPERATORS = frozenset({"contains", "not_contains", "startswith", "endswith"})
    if operator in _STRING_OPERATORS and field_val is not None and not isinstance(field_val, str):
        raise ValueError(
            f"Field {field!r} must be a string for operator {operator!r}, "
            f"got {type(field_val).__name__!r}"
        )

    if operator == "equals":
        result = field_val == value
    elif operator == "not_equals":
        result = field_val != value
    elif operator == "contains":
        result = value in (field_val or "")
    elif operator == "not_contains":
        result = value not in (field_val or "")
    elif operator == "startswith":
        result = (field_val or "").startswith(value)
    elif operator == "endswith":
        result = (field_val or "").endswith(value)
    elif operator == "in":
        result = field_val in value
    elif operator == "not_in":
        result = field_val not in value
    else:  # pragma: no cover
        # Unreachable: operator was validated against _ALLOWED_OPERATORS above.
        raise AssertionError(f"Unhandled operator: {operator!r}")

    _logger.info(
        "rule_evaluated field=%r operator=%r result=%r",
        field,
        operator,
        result,
    )
    return result


def evaluate_rules(rules: list[dict], payload: dict) -> bool:
    """Evaluate a list of rules against a payload dict.

    All rules must match (logical AND).

    Args:
        rules: A list of rule dicts, each with ``field``, ``operator``, ``value``.
        payload: The event payload to evaluate against.

    Returns:
        True if all rules match, False otherwise.
    """
    result = all(evaluate_rule(rule, payload) for rule in rules)
    _logger.info(
        "rules_evaluated rule_count=%d result=%r",
        len(rules),
        result,
    )
    return result
