"""Report agent — generates structured security reports from pipeline findings."""

from __future__ import annotations

import json
import logging
import os
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any

import anthropic

logger = logging.getLogger(__name__)

REPORT_MAX_TOKENS = 8192

# ---------------------------------------------------------------------------
# Enumerations and data structures
# ---------------------------------------------------------------------------

class ReportFormat(str, Enum):
    MARKDOWN = "markdown"
    JSON = "json"
    HTML = "html"


class ReportSection(str, Enum):
    EXECUTIVE_SUMMARY = "executive_summary"
    SCOPE = "scope"
    FINDINGS_OVERVIEW = "findings_overview"
    DETAILED_FINDINGS = "detailed_findings"
    RISK_MATRIX = "risk_matrix"
    REMEDIATION = "remediation"
    CONCLUSION = "conclusion"
    APPENDIX = "appendix"


@dataclass
class ReportMetadata:
    """Metadata block for a generated report."""
    title: str
    target: str
    generated_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    author: str = "Automated Security Pipeline"
    version: str = "1.0"
    classification: str = "CONFIDENTIAL"
    tags: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "title": self.title,
            "target": self.target,
            "generated_at": self.generated_at,
            "author": self.author,
            "version": self.version,
            "classification": self.classification,
            "tags": self.tags,
        }


@dataclass
class ReportInput:
    """Input bundle for the report agent."""
    metadata: ReportMetadata
    findings: list[dict]
    scope: list[str] = field(default_factory=list)
    methodology: str = "OWASP Testing Guide v4.2"
    sections: list[ReportSection] = field(default_factory=list)
    additional_context: dict[str, Any] = field(default_factory=dict)

    def __post_init__(self):
        if not self.sections:
            self.sections = list(ReportSection)

    @property
    def critical_findings(self) -> list[dict]:
        return [f for f in self.findings if f.get("severity") == "CRITICAL"]

    @property
    def high_findings(self) -> list[dict]:
        return [f for f in self.findings if f.get("severity") == "HIGH"]

    @property
    def medium_findings(self) -> list[dict]:
        return [f for f in self.findings if f.get("severity") == "MEDIUM"]

    @property
    def low_findings(self) -> list[dict]:
        return [f for f in self.findings if f.get("severity") in ("LOW", "INFO")]

    def severity_summary(self) -> dict[str, int]:
        return {
            "CRITICAL": len(self.critical_findings),
            "HIGH": len(self.high_findings),
            "MEDIUM": len(self.medium_findings),
            "LOW": len(self.low_findings),
            "TOTAL": len(self.findings),
        }


# ---------------------------------------------------------------------------
# Prompt builders
# ---------------------------------------------------------------------------

_MARKDOWN_SYSTEM_PROMPT = """\
You are a senior security consultant generating a professional security assessment report.
Write clear, structured Markdown. Use headers, tables, and bullet lists appropriately.
Be precise, professional, and actionable. Do not include placeholder text.
"""

_JSON_SYSTEM_PROMPT = """\
You are a senior security consultant. Generate a structured security report as a JSON object.
Include all sections: executive_summary, scope, findings_overview, detailed_findings,
risk_matrix, remediation_roadmap, and conclusion. Be precise and actionable.
"""


def _build_user_prompt(report_input: ReportInput) -> str:
    """Build the user prompt from a ReportInput."""
    payload = {
        "metadata": report_input.metadata.to_dict(),
        "scope": report_input.scope,
        "methodology": report_input.methodology,
        "severity_summary": report_input.severity_summary(),
        "findings": report_input.findings,
        "sections_requested": [s.value for s in report_input.sections],
    }
    if report_input.additional_context:
        payload["additional_context"] = report_input.additional_context
    return json.dumps(payload, indent=2)


# ---------------------------------------------------------------------------
# Report agent
# ---------------------------------------------------------------------------

class ReportAgent:
    """Generates comprehensive security reports using an LLM.

    Supports both Markdown and JSON output formats. Handles large finding
    sets by batching into multiple LLM calls when necessary.
    """

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
        output_format: ReportFormat = ReportFormat.MARKDOWN,
    ):
        self.model = model
        self.output_format = output_format
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def generate(self, report_input: ReportInput) -> str:
        """Generate a full security report.

        Args:
            report_input: ReportInput containing findings, metadata, and scope.

        Returns:
            Generated report as a string (Markdown or JSON depending on format).
        """
        system_prompt = (
            _MARKDOWN_SYSTEM_PROMPT
            if self.output_format == ReportFormat.MARKDOWN
            else _JSON_SYSTEM_PROMPT
        )
        user_prompt = _build_user_prompt(report_input)
        logger.info(
            "ReportAgent.generate target=%s findings=%d format=%s",
            report_input.metadata.target,
            len(report_input.findings),
            self.output_format.value,
        )
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=8192,
        )
        return response.content[0].text

    def generate_executive_summary(self, report_input: ReportInput) -> str:
        """Generate only the executive summary section of a report."""
        system_prompt = (
            "You are a senior security consultant. Write a concise executive summary "
            "for a security assessment report. Use professional language suitable for "
            "C-suite audience. Keep it under 500 words. Output Markdown."
        )
        payload = {
            "target": report_input.metadata.target,
            "severity_summary": report_input.severity_summary(),
            "critical_findings": [
                {"title": f.get("title"), "cwe": f.get("cwe")}
                for f in report_input.critical_findings
            ],
            "high_findings": [
                {"title": f.get("title"), "cwe": f.get("cwe")}
                for f in report_input.high_findings
            ],
        }
        user_prompt = json.dumps(payload, indent=2)
        logger.info(
            "ReportAgent.generate_executive_summary target=%s",
            report_input.metadata.target,
        )
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=8192,
        )
        return response.content[0].text
