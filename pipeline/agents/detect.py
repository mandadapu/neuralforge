"""Detect agent — detects security vulnerabilities in source code."""

from __future__ import annotations

import json
import logging
import os

import anthropic

logger = logging.getLogger(__name__)

SYSTEM_PROMPT = """\
You are an expert security engineer performing static analysis. Analyze the provided source code
and identify security vulnerabilities. For each finding output a JSON object with keys:
- file: source file path
- line: line number (integer)
- severity: CRITICAL / HIGH / MEDIUM / LOW / INFO
- title: short vulnerability title
- description: detailed explanation
- cwe: applicable CWE identifier
- recommendation: remediation guidance

Respond with a JSON array of finding objects.
"""


class DetectAgent:
    """Agent that detects security vulnerabilities in source code."""

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
    ):
        self.model = model
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def detect(self, code: str, file_path: str = "<unknown>") -> list[dict]:
        """Detect vulnerabilities in a code snippet."""
        user_prompt = f"File: {file_path}\n\n```\n{code}\n```"
        logger.debug("DetectAgent.detect file=%s", file_path)
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
            logger.warning("DetectAgent: failed to parse JSON response for %s", file_path)
            return []

    def detect_files(self, files: dict[str, str]) -> list[dict]:
        """Detect vulnerabilities across multiple files."""
        all_findings: list[dict] = []
        for path, code in files.items():
            all_findings.extend(self.detect(code, path))
        return all_findings
