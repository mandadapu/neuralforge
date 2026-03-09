"""SAST scanner — lightweight static analysis security testing wrapper.

Provides a simplified interface to LLM-powered SAST scanning with
sensible defaults for common use cases.
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
import re
import time
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 4096

SYSTEM_PROMPT = """\
You are a security-focused static analysis tool. Analyze the provided source code and
identify security vulnerabilities. For each finding, provide:
- file path and line number
- severity: CRITICAL, HIGH, MEDIUM, LOW, or INFO
- vulnerability title and detailed description
- CWE identifier
- remediation recommendation

Respond with a JSON array of finding objects.
"""


# ---------------------------------------------------------------------------
# Enumerations and data structures
# ---------------------------------------------------------------------------

class Severity(str, Enum):
    CRITICAL = "CRITICAL"
    HIGH = "HIGH"
    MEDIUM = "MEDIUM"
    LOW = "LOW"
    INFO = "INFO"


@dataclass
class ScanConfig:
    """Configuration for a SAST scan run."""
    model: str = "claude-sonnet-4-6"
    min_severity: Severity = Severity.LOW
    max_file_size_kb: int = 512
    include_extensions: set[str] = field(default_factory=lambda: {
        ".py", ".js", ".ts", ".go", ".java", ".rb", ".php"
    })
    enable_caching: bool = True
    timeout_seconds: int = 30


@dataclass
class Finding:
    """A single SAST finding."""
    id: str
    file_path: str
    line: int
    severity: Severity
    title: str
    description: str
    cwe: str
    recommendation: str

    @classmethod
    def from_dict(cls, data: dict) -> "Finding":
        sev_str = data.get("severity", "INFO").upper()
        try:
            sev = Severity(sev_str)
        except ValueError:
            sev = Severity.INFO
        return cls(
            id=data.get("id", ""),
            file_path=data.get("file", ""),
            line=data.get("line", 0),
            severity=sev,
            title=data.get("title", ""),
            description=data.get("description", ""),
            cwe=data.get("cwe", ""),
            recommendation=data.get("recommendation", ""),
        )

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "file": self.file_path,
            "line": self.line,
            "severity": self.severity.value,
            "title": self.title,
            "description": self.description,
            "cwe": self.cwe,
            "recommendation": self.recommendation,
        }


@dataclass
class ScanResult:
    """Result of a SAST scan."""
    file_path: str
    findings: list[Finding] = field(default_factory=list)
    error: str | None = None
    scan_time_ms: float = 0.0

    @property
    def finding_count(self) -> int:
        return len(self.findings)

    @property
    def has_critical(self) -> bool:
        return any(f.severity == Severity.CRITICAL for f in self.findings)


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _detect_language(file_path: str) -> str:
    ext = Path(file_path).suffix.lower()
    mapping = {
        ".py": "python", ".js": "javascript", ".ts": "typescript",
        ".go": "go", ".java": "java", ".rb": "ruby", ".php": "php",
        ".cs": "csharp", ".cpp": "cpp", ".c": "c", ".rs": "rust",
    }
    return mapping.get(ext, "text")


def _build_prompt(file_path: str, content: str, language: str) -> str:
    if len(content) > 50_000:
        half = 25_000
        content = content[:half] + f"\n\n... [truncated] ...\n\n" + content[-half:]
    return f"File: {file_path}\nLanguage: {language}\n\n```{language}\n{content}\n```"


def _parse_findings(raw: str, file_path: str) -> list[Finding]:
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        match = re.search(r"```(?:json)?\s*(\[.*?\])\s*```", raw, re.DOTALL)
        if match:
            try:
                data = json.loads(match.group(1))
            except json.JSONDecodeError:
                return []
        else:
            return []

    if isinstance(data, dict):
        data = data.get("findings", [])

    findings = []
    for item in data:
        try:
            findings.append(Finding.from_dict(item))
        except Exception:  # noqa: BLE001
            pass
    return findings


# ---------------------------------------------------------------------------
# Main scanner class
# ---------------------------------------------------------------------------

class SASTScanner:
    """Lightweight LLM-powered SAST scanner.

    Provides scan_file and scan_files methods for integrating SAST
    into CI/CD pipelines and code review workflows.
    """

    def __init__(
        self,
        config: ScanConfig | None = None,
        api_key: str | None = None,
    ):
        self.config = config or ScanConfig()
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )
        self._cache: dict[str, ScanResult] = {}

    def _cache_key(self, content: str) -> str:
        return hashlib.sha256(content.encode()).hexdigest()

    def scan_file(self, file_path: str, content: str) -> ScanResult:
        """Scan a single source file.

        Args:
            file_path: Path to the file (used for language detection and display).
            content: Source code as a string.

        Returns:
            ScanResult with findings.
        """
        if self.config.enable_caching:
            cache_key = self._cache_key(content)
            if cache_key in self._cache:
                return self._cache[cache_key]

        language = _detect_language(file_path)
        user_prompt = _build_prompt(file_path, content, language)
        start = time.monotonic()

        logger.info("SASTScanner.scan_file file=%s language=%s", file_path, language)

        response = self._client.messages.create(
            model=self.config.model,
            system=SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        findings = _parse_findings(raw, file_path)
        elapsed_ms = (time.monotonic() - start) * 1000

        result = ScanResult(
            file_path=file_path,
            findings=findings,
            scan_time_ms=elapsed_ms,
        )

        if self.config.enable_caching:
            self._cache[self._cache_key(content)] = result

        return result

    def scan_files(self, files: dict[str, str]) -> list[ScanResult]:
        """Scan multiple files.

        Args:
            files: Mapping of file_path -> source code.

        Returns:
            List of ScanResult objects.
        """
        results = []
        for path, content in files.items():
            try:
                results.append(self.scan_file(path, content))
            except Exception as exc:  # noqa: BLE001
                logger.error("SASTScanner.scan_files error file=%s: %s", path, exc)
                results.append(ScanResult(file_path=path, error=str(exc)))
        return results

    def summary(self, results: list[ScanResult]) -> dict:
        """Generate a summary dictionary from scan results."""
        all_findings = [f for r in results for f in r.findings]
        counts: dict[str, int] = {s.value: 0 for s in Severity}
        for f in all_findings:
            counts[f.severity.value] = counts.get(f.severity.value, 0) + 1
        return {
            "files_scanned": len(results),
            "total_findings": len(all_findings),
            "by_severity": counts,
            "files_with_errors": sum(1 for r in results if r.error),
        }
