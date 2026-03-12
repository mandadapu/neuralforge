"""
review_rules.py — Safe rule evaluation for autopilot review automation.

Rules are evaluated using an explicit allowlist dispatch table.
Dynamic code execution (eval/exec) is never used.
"""

from __future__ import annotations

import logging
from typing import Any

# Audit logger — emits rule evaluation events without sensitive field values.
_audit_log = logging.getLogger("autopilot.review_rules.audit")

# ---------------------------------------------------------------------------
# Field-level access control
# ---------------------------------------------------------------------------
# Only these payload fields may be referenced by rules.  This prevents rules
# from accidentally inspecting PII, tokens, or other sensitive attributes that
# may exist in a raw GitHub webhook payload.
# Extend this set in coordination with a data-classification review.
ALLOWED_PAYLOAD_FIELDS: frozenset[str] = frozenset(
    {
        "action",
        "state",
        "title",
        "body",
        "label",
        "labels",
        "base_ref",
        "head_ref",
        "base_label",
        "head_label",
        "draft",
        "merged",
        "assignees",
        "requested_reviewers",
        "milestone",
        "author_association",
        "changed_files",
        "additions",
        "deletions",
    }
)

# Permitted types for rule 'value'.  Restricts what data can be stored in
# rule config, which limits surface area for data-minimization violations.
_ALLOWED_VALUE_TYPES = (str, int, float, bool, list)

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
    """Validate that a rule dict contains only expected keys with safe types."""
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

    # Field-level access control: only allowlisted fields may be referenced.
    field = rule["field"]
    if field not in ALLOWED_PAYLOAD_FIELDS:
        raise ValueError(
            "Rule references a field that is not in the permitted field allowlist"
        )

    # Value type validation: restrict to safe, primitive types only.
    value = rule["value"]
    if not isinstance(value, _ALLOWED_VALUE_TYPES):
        raise TypeError(
            f"Rule 'value' must be one of {[t.__name__ for t in _ALLOWED_VALUE_TYPES]}, "
            f"got {type(value).__name__!r}"
        )
    # If value is a list, all elements must also be primitive types.
    if isinstance(value, list):
        for item in value:
            if not isinstance(item, (str, int, float, bool)):
                raise TypeError(
                    "Rule 'value' list elements must be str, int, float, or bool"
                )


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
        # Do NOT echo the operator value — it is caller-controlled input.
        raise ValueError(
            f"Unsupported rule operator. "
            f"Allowed operators: {sorted(ALLOWED_RULE_OPERATORS)!r}"
        )

    field = rule["field"]
    value = rule["value"]

    try:
        result = ALLOWED_RULE_OPERATORS[operator](field, value, payload)
    except Exception:
        # Catch operator errors without re-raising payload data in the traceback.
        _audit_log.warning(
            "Rule evaluation error",
            extra={"field": field, "operator": operator},
        )
        raise RuntimeError(
            "Rule evaluation failed; check logs for details"
        ) from None

    _audit_log.info(
        "Rule evaluated",
        extra={"field": field, "operator": operator, "result": result},
    )
    return result


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

    outcome = all(results) if match_all else any(results)
    _audit_log.info(
        "Rules batch evaluated",
        extra={"rule_count": len(rules), "match_all": match_all, "outcome": outcome},
    )
    return outcome
