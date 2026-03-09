"""
api/autopilot/review_rules.py

Autopilot PR review rule engine.

Rules are evaluated via a static dispatch table — no eval() or exec() is used.
User-supplied rule names are looked up in RULE_HANDLERS; unknown names raise
ValueError rather than being executed as arbitrary code.
"""

from __future__ import annotations

import json
import re
from typing import Any


# ---------------------------------------------------------------------------
# Individual rule implementations
# ---------------------------------------------------------------------------

def _check_has_tests(context: dict[str, Any]) -> bool:
    """Return True when the PR diff contains at least one test file."""
    changed_files: list[str] = context.get("changed_files", [])
    test_patterns = (
        re.compile(r"test_.*\.py$"),
        re.compile(r".*_test\.py$"),
        re.compile(r".*_test\.go$"),
        re.compile(r".*\.test\.[jt]sx?$"),
        re.compile(r".*\.spec\.[jt]sx?$"),
    )
    return any(
        any(pat.search(f) for pat in test_patterns)
        for f in changed_files
    )


def _check_no_secrets(context: dict[str, Any]) -> bool:
    """Return True when no obvious secret patterns are present in the diff."""
    diff: str = context.get("diff", "")
    secret_patterns = (
        re.compile(r"(?i)(password|passwd|secret|api[_-]?key)\s*=\s*['\"][^'\"]{6,}"),
        re.compile(r"-----BEGIN (RSA|EC|DSA|OPENSSH) PRIVATE KEY-----"),
        re.compile(r"(?i)aws_secret_access_key\s*=\s*\S+"),
    )
    return not any(pat.search(diff) for pat in secret_patterns)


def _check_lint_passes(context: dict[str, Any]) -> bool:
    """Return True when the lint_status field indicates success."""
    return context.get("lint_status") == "passed"


def _check_size_limit(context: dict[str, Any]) -> bool:
    """Return True when the PR is within the configured line-change limit."""
    additions: int = context.get("additions", 0)
    deletions: int = context.get("deletions", 0)
    limit: int = context.get("size_limit", 500)
    return (additions + deletions) <= limit


def _check_no_debug_code(context: dict[str, Any]) -> bool:
    """Return True when no debug/print statements are added in the diff."""
    diff: str = context.get("diff", "")
    debug_patterns = (
        re.compile(r"^\+.*\bprint\s*\(", re.MULTILINE),
        re.compile(r"^\+.*\bdebugger\b", re.MULTILINE),
        re.compile(r"^\+.*\bconsole\.log\s*\(", re.MULTILINE),
        re.compile(r"^\+.*\bpdb\.set_trace\s*\(", re.MULTILINE),
    )
    return not any(pat.search(diff) for pat in debug_patterns)


def _check_description_present(context: dict[str, Any]) -> bool:
    """Return True when the PR body is non-empty."""
    body: str = context.get("pr_body", "").strip()
    return len(body) > 0


# ---------------------------------------------------------------------------
# Dispatch table — the only place where rule names map to callables.
# This replaces any prior use of eval()/exec() for dynamic rule evaluation.
# ---------------------------------------------------------------------------

RULE_HANDLERS: dict[str, Any] = {
    "has_tests": _check_has_tests,
    "no_secrets": _check_no_secrets,
    "lint_passes": _check_lint_passes,
    "size_limit": _check_size_limit,
    "no_debug_code": _check_no_debug_code,
    "description_present": _check_description_present,
}


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def evaluate_rule(rule_name: str, context: dict[str, Any]) -> bool:
    """Evaluate a single named rule against the given context.

    Args:
        rule_name: The rule identifier (must be a key in RULE_HANDLERS).
        context:   A dictionary containing PR metadata and diff content.

    Returns:
        True if the rule passes, False otherwise.

    Raises:
        ValueError: If rule_name is not a known rule.
        TypeError:  If context is not a dict.
    """
    if not isinstance(context, dict):
        raise TypeError(f"context must be a dict, got {type(context).__name__!r}")

    handler = RULE_HANDLERS.get(rule_name)
    if handler is None:
        raise ValueError(
            f"Unknown rule: {rule_name!r}. "
            f"Valid rules are: {sorted(RULE_HANDLERS)}"
        )
    return bool(handler(context))


def parse_rule_config(raw_config: str) -> dict[str, Any]:
    """Parse a JSON rule configuration string.

    Uses json.loads() instead of eval()/exec() to safely deserialise
    structured rule configuration supplied by callers.

    Args:
        raw_config: A JSON-encoded string describing rule configuration.

    Returns:
        Parsed configuration as a Python dict.

    Raises:
        json.JSONDecodeError: If raw_config is not valid JSON.
        TypeError:            If the parsed result is not a dict.
    """
    result = json.loads(raw_config)
    if not isinstance(result, dict):
        raise TypeError(
            f"Rule config must be a JSON object, got {type(result).__name__!r}"
        )
    return result


class ReviewRuleEngine:
    """Evaluates a set of review rules against a PR context.

    Example
    -------
    engine = ReviewRuleEngine(["has_tests", "no_secrets", "size_limit"])
    results = engine.evaluate(context)
    passed  = engine.all_passed(context)
    """

    def __init__(self, rules: list[str]) -> None:
        unknown = [r for r in rules if r not in RULE_HANDLERS]
        if unknown:
            raise ValueError(
                f"Unknown rules: {unknown!r}. "
                f"Valid rules are: {sorted(RULE_HANDLERS)}"
            )
        self._rules = list(rules)

    @classmethod
    def from_json_config(cls, raw_config: str) -> "ReviewRuleEngine":
        """Construct an engine from a JSON config string.

        Expected format::

            {"rules": ["has_tests", "no_secrets"]}
        """
        config = parse_rule_config(raw_config)
        rules = config.get("rules", [])
        if not isinstance(rules, list):
            raise TypeError("'rules' must be a JSON array")
        return cls(rules)

    def evaluate(self, context: dict[str, Any]) -> dict[str, bool]:
        """Run all rules and return a mapping of rule name to pass/fail."""
        return {rule: evaluate_rule(rule, context) for rule in self._rules}

    def all_passed(self, context: dict[str, Any]) -> bool:
        """Return True only when every rule passes."""
        return all(self.evaluate(context).values())

    def failed_rules(self, context: dict[str, Any]) -> list[str]:
        """Return the names of rules that did not pass."""
        return [rule for rule, passed in self.evaluate(context).items() if not passed]
