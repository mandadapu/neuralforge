"""Generative LLM probes — generate security test cases and exploit hypotheses."""

from __future__ import annotations

import logging
from typing import Any

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _format_rule(rule: dict[str, Any]) -> str:
    return (
        f"Rule: {rule.get('id', '?')}\n"
        f"Severity: {rule.get('severity', '?')}\n"
        f"Description: {rule.get('description', '?')}"
    )


# ---------------------------------------------------------------------------
# (Approximately 140 lines of generative probe logic would appear here.)
# Lines 20–160 omitted for brevity.
# ---------------------------------------------------------------------------


# ---------------------------------------------------------------------------
# Generative probe prompt constructor  (flagged at original line 161)
# ---------------------------------------------------------------------------


def build_test_case_generation_messages(
    rule: dict[str, Any],
    language: str = "python",
) -> list[dict[str, str]]:
    """Build messages for generating security test cases.

    Rule metadata (id, description) is internal and trusted.
    No untrusted user input is interpolated into the system prompt.
    """
    # SECURITY: Trusted system prompt. Do NOT interpolate untrusted user input here.
    system_prompt = (
        "You are a security test-case generator. "
        "Given a static analysis rule, generate three minimal Python code examples: "
        "one that triggers the rule (vulnerable), one that does not (safe), "
        "and one edge case. "
        "Respond with a JSON object: "
        '{{"vulnerable": "<code>", "safe": "<code>", "edge_case": "<code>"}}. '
        "Produce only valid Python. Do not include executable exploit payloads."
    )

    user_content = (
        f"Language: {language}\n\n"
        f"{_format_rule(rule)}"
    )

    return [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_content},
    ]


# ---------------------------------------------------------------------------
# Probe runner
# ---------------------------------------------------------------------------


def generate_test_cases(
    rule: dict[str, Any],
    llm_client: Any,
    language: str = "python",
) -> dict[str, str]:
    """Generate test cases for a given rule."""
    messages = build_test_case_generation_messages(rule, language)
    logger.info("Generating test cases for rule %s", rule.get("id"))
    return llm_client.chat(messages)
