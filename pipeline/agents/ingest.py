"""Ingest agent — ingests and pre-processes source files for the pipeline."""

from __future__ import annotations

import logging
import os

import anthropic

logger = logging.getLogger(__name__)


class IngestAgent:
    """Ingests source files and extracts metadata using an LLM."""

    SYSTEM_PROMPT = (
        "You are a code analyst. Given source code, extract metadata: language, framework, "
        "dependencies, entry points, and any security-sensitive patterns. "
        "Respond with a compact JSON object."
    )

    def __init__(self, model: str = "claude-sonnet-4-6", api_key: str | None = None):
        self.model = model
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def ingest(self, code: str, file_path: str = "<unknown>") -> dict:
        """Ingest a single source file and return extracted metadata."""
        import json

        user_prompt = f"File: {file_path}\n\n```\n{code}\n```"
        response = self._client.messages.create(
            model=self.model,
            system=self.SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )
        raw = response.content[0].text
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            logger.warning("IngestAgent: JSON parse error for %s", file_path)
            return {"file": file_path, "raw": raw}
