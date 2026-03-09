"""Classify agent — classifies security findings by severity and type."""

from __future__ import annotations

import json
import logging
import os

import anthropic

logger = logging.getLogger(__name__)

SYSTEM_PROMPT = """\
You are a security classification expert. Given a security finding, classify it by:
1. Severity: CRITICAL, HIGH, MEDIUM, LOW, or INFO
2. Category: e.g., injection, authentication, authorization, cryptography, etc.
3. CWE: the most applicable CWE identifier (e.g., CWE-89)
4. CVSS score estimate (0.0–10.0)

Respond with a JSON object with keys: severity, category, cwe, cvss_score, rationale.
"""


class ClassifyAgent:
    """Agent that classifies security findings using an LLM."""

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
    ):
        self.model = model
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def classify(self, finding: dict) -> dict:
        """Classify a single security finding."""
        user_prompt = json.dumps(finding, indent=2)
        logger.debug("ClassifyAgent.classify finding_id=%s", finding.get("id"))
        response = self._client.messages.create(
            model=self.model,
            system=SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )
        raw = response.content[0].text
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            logger.warning("ClassifyAgent: failed to parse JSON response")
            return {"raw": raw}

    def classify_batch(self, findings: list[dict]) -> list[dict]:
        """Classify a batch of security findings."""
        return [self.classify(f) for f in findings]
