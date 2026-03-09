"""LLM-powered SAST scanner — comprehensive static analysis security testing tool.

Combines traditional pattern-based SAST with LLM-powered semantic analysis to
detect a wide range of security vulnerabilities across multiple languages.
"""

from __future__ import annotations

import ast
import hashlib
import json
import logging
import os
import re
import textwrap
import time
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import Any, Callable, Generator, Iterator

import anthropic

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

DEFAULT_MAX_TOKENS = 4096
MAX_FILE_BYTES = 1024 * 1024  # 1 MB
MAX_PROMPT_CHARS = 60_000
SUPPORTED_LANGUAGES = frozenset({
    "python", "javascript", "typescript", "go", "java", "ruby",
    "php", "csharp", "cpp", "c", "rust", "kotlin", "swift",
})

SAST_SYSTEM_PROMPT = """\
You are an expert SAST (Static Application Security Testing) engine. Analyze the provided
source code for security vulnerabilities. For each vulnerability found, output a structured
finding with precise location, severity assessment, CWE mapping, and actionable remediation.

Output a JSON object with the following schema:
{
  "findings": [
    {
      "id": "<unique-id>",
      "file": "<file-path>",
      "line_start": <int>,
      "line_end": <int>,
      "severity": "CRITICAL|HIGH|MEDIUM|LOW|INFO",
      "confidence": "HIGH|MEDIUM|LOW",
      "title": "<short vulnerability title>",
      "description": "<detailed technical description>",
      "cwe": "<CWE-NNN>",
      "owasp": "<OWASP-AXXX:YYYY>",
      "recommendation": "<concrete remediation steps>",
      "code_snippet": "<relevant code>",
      "references": ["<url>", ...]
    }
  ],
  "scan_summary": {
    "language": "<lang>",
    "lines_scanned": <int>,
    "patterns_matched": <int>,
    "analysis_confidence": "HIGH|MEDIUM|LOW"
  }
}
"""


# ---------------------------------------------------------------------------
# Enumerations
# ---------------------------------------------------------------------------

class Severity(str, Enum):
    CRITICAL = "CRITICAL"
    HIGH = "HIGH"
    MEDIUM = "MEDIUM"
    LOW = "LOW"
    INFO = "INFO"

    @classmethod
    def from_string(cls, s: str) -> "Severity":
        try:
            return cls(s.upper())
        except ValueError:
            return cls.INFO


class Confidence(str, Enum):
    HIGH = "HIGH"
    MEDIUM = "MEDIUM"
    LOW = "LOW"


class ScanStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class ScanTarget:
    """Represents a single file to be scanned."""
    file_path: str
    content: str
    language: str = ""
    priority: int = 0

    def __post_init__(self):
        if not self.language:
            self.language = _detect_language(self.file_path)

    @property
    def line_count(self) -> int:
        return self.content.count("\n") + 1

    @property
    def byte_size(self) -> int:
        return len(self.content.encode())

    @property
    def content_hash(self) -> str:
        return hashlib.sha256(self.content.encode()).hexdigest()


@dataclass
class SASTFinding:
    """A single SAST finding."""
    id: str
    file_path: str
    line_start: int
    line_end: int
    severity: Severity
    confidence: Confidence
    title: str
    description: str
    cwe: str
    recommendation: str
    owasp: str = ""
    code_snippet: str = ""
    references: list[str] = field(default_factory=list)
    source: str = "llm"  # "llm" or "pattern"
    rule_id: str = ""

    @classmethod
    def from_dict(cls, data: dict) -> "SASTFinding":
        return cls(
            id=data.get("id", ""),
            file_path=data.get("file", ""),
            line_start=data.get("line_start", 0),
            line_end=data.get("line_end", 0),
            severity=Severity.from_string(data.get("severity", "INFO")),
            confidence=Confidence(data.get("confidence", "LOW")),
            title=data.get("title", ""),
            description=data.get("description", ""),
            cwe=data.get("cwe", ""),
            recommendation=data.get("recommendation", ""),
            owasp=data.get("owasp", ""),
            code_snippet=data.get("code_snippet", ""),
            references=data.get("references", []),
        )

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "file": self.file_path,
            "line_start": self.line_start,
            "line_end": self.line_end,
            "severity": self.severity.value,
            "confidence": self.confidence.value,
            "title": self.title,
            "description": self.description,
            "cwe": self.cwe,
            "owasp": self.owasp,
            "recommendation": self.recommendation,
            "code_snippet": self.code_snippet,
            "references": self.references,
            "source": self.source,
        }


@dataclass
class ScanResult:
    """Result of scanning a single file."""
    target: ScanTarget
    findings: list[SASTFinding] = field(default_factory=list)
    status: ScanStatus = ScanStatus.PENDING
    error: str | None = None
    scan_duration_ms: float = 0.0
    patterns_matched: int = 0
    analysis_confidence: str = "MEDIUM"

    @property
    def critical_count(self) -> int:
        return sum(1 for f in self.findings if f.severity == Severity.CRITICAL)

    @property
    def high_count(self) -> int:
        return sum(1 for f in self.findings if f.severity == Severity.HIGH)

    @property
    def medium_count(self) -> int:
        return sum(1 for f in self.findings if f.severity == Severity.MEDIUM)

    @property
    def finding_count(self) -> int:
        return len(self.findings)


@dataclass
class SASTScanReport:
    """Aggregated report from scanning multiple files."""
    scan_id: str
    targets_scanned: int = 0
    total_findings: int = 0
    critical: int = 0
    high: int = 0
    medium: int = 0
    low: int = 0
    scan_results: list[ScanResult] = field(default_factory=list)
    errors: list[str] = field(default_factory=list)
    started_at: float = field(default_factory=time.time)
    completed_at: float = 0.0

    @property
    def duration_seconds(self) -> float:
        if self.completed_at:
            return self.completed_at - self.started_at
        return 0.0

    def all_findings(self) -> list[SASTFinding]:
        findings = []
        for r in self.scan_results:
            findings.extend(r.findings)
        return findings


# ---------------------------------------------------------------------------
# Pattern library
# ---------------------------------------------------------------------------

@dataclass
class PatternRule:
    """A single pattern-matching rule."""
    rule_id: str
    name: str
    pattern: str
    severity: Severity
    cwe: str
    description: str
    languages: set[str] = field(default_factory=set)  # empty = all languages


_PATTERN_RULES: list[PatternRule] = [
    PatternRule("P001", "eval-usage", r"\beval\s*\(", Severity.HIGH, "CWE-95",
                "Dynamic code evaluation via eval()", {"python", "javascript"}),
    PatternRule("P002", "exec-usage", r"\bexec\s*\(", Severity.HIGH, "CWE-78",
                "OS command execution", {"python"}),
    PatternRule("P003", "shell-true", r"shell\s*=\s*True", Severity.HIGH, "CWE-78",
                "Shell injection via subprocess", {"python"}),
    PatternRule("P004", "hardcoded-secret", r"(?i)(password|secret|api_key|token)\s*=\s*['\"][^'\"]{8,}",
                Severity.HIGH, "CWE-798", "Hardcoded credential"),
    PatternRule("P005", "sql-concat", r"['\"].*SELECT.*\+", Severity.HIGH, "CWE-89",
                "Potential SQL injection via string concatenation"),
    PatternRule("P006", "weak-hash-md5", r"\bmd5\s*\(|\bhashlib\.md5\b", Severity.MEDIUM, "CWE-327",
                "Weak cryptographic hash MD5", {"python"}),
    PatternRule("P007", "weak-hash-sha1", r"\bsha1\s*\(|\bhashlib\.sha1\b", Severity.MEDIUM, "CWE-327",
                "Weak cryptographic hash SHA-1", {"python"}),
    PatternRule("P008", "pickle-load", r"\bpickle\.loads?\s*\(", Severity.HIGH, "CWE-502",
                "Insecure deserialization via pickle", {"python"}),
    PatternRule("P009", "yaml-unsafe", r"\byaml\.load\s*\((?!.*Loader)", Severity.HIGH, "CWE-502",
                "Unsafe YAML deserialization", {"python"}),
    PatternRule("P010", "http-cleartext", r"http://(?!localhost|127\.|0\.0\.0\.0)",
                Severity.MEDIUM, "CWE-319", "Cleartext HTTP transmission"),
    PatternRule("P011", "bind-all", r"0\.0\.0\.0", Severity.LOW, "CWE-605",
                "Service bound to all interfaces"),
    PatternRule("P012", "insecure-random", r"\brandom\.random\(\)|\brandom\.randint\b",
                Severity.MEDIUM, "CWE-330", "Insecure random number generation", {"python"}),
    PatternRule("P013", "path-traversal", r"\.\./|\.\./\.\./", Severity.HIGH, "CWE-22",
                "Potential path traversal"),
    PatternRule("P014", "open-redirect", r"redirect\s*\(.*request\.", Severity.MEDIUM, "CWE-601",
                "Potential open redirect"),
    PatternRule("P015", "debug-mode", r"(?i)debug\s*=\s*true|DEBUG\s*=\s*True",
                Severity.LOW, "CWE-489", "Debug mode enabled"),
]


def _detect_language(file_path: str) -> str:
    ext = Path(file_path).suffix.lower()
    mapping = {
        ".py": "python", ".js": "javascript", ".ts": "typescript",
        ".go": "go", ".java": "java", ".rb": "ruby", ".php": "php",
        ".cs": "csharp", ".cpp": "cpp", ".c": "c", ".rs": "rust",
        ".kt": "kotlin", ".swift": "swift",
    }
    return mapping.get(ext, "text")


def _run_pattern_rules(content: str, file_path: str, language: str) -> list[dict]:
    """Apply pattern rules to source code and return preliminary findings."""
    hits: list[dict] = []
    lines = content.splitlines()
    for rule in _PATTERN_RULES:
        if rule.languages and language not in rule.languages:
            continue
        regex = re.compile(rule.pattern, re.IGNORECASE)
        for lineno, line in enumerate(lines, start=1):
            m = regex.search(line)
            if m:
                hit_id = hashlib.sha256(
                    f"{file_path}:{lineno}:{rule.rule_id}".encode()
                ).hexdigest()[:12]
                hits.append({
                    "id": hit_id,
                    "rule_id": rule.rule_id,
                    "file": file_path,
                    "line": lineno,
                    "severity": rule.severity.value,
                    "cwe": rule.cwe,
                    "title": rule.name,
                    "matched": m.group(0)[:100],
                })
    return hits


def _truncate_content(content: str, max_chars: int = MAX_PROMPT_CHARS) -> str:
    if len(content) <= max_chars:
        return content
    half = max_chars // 2
    notice = f"\n\n... [{len(content) - max_chars} chars truncated] ...\n\n"
    return content[:half] + notice + content[-half:]


def _build_scan_prompt(target: ScanTarget, pattern_hits: list[dict]) -> str:
    """Build the user prompt for the SAST LLM call."""
    hints_section = ""
    if pattern_hits:
        hints_list = [
            f"  [{h['id']}] line {h['line']}: {h['title']} ({h['cwe']}) — {h['matched']!r}"
            for h in pattern_hits
        ]
        hints_section = "Pre-computed pattern hits:\n" + "\n".join(hints_list) + "\n\n"

    code = _truncate_content(target.content)
    return (
        f"File: {target.file_path}\n"
        f"Language: {target.language}\n"
        f"Lines: {target.line_count}\n"
        f"Hash: {target.content_hash[:16]}\n\n"
        f"{hints_section}"
        f"Source code:\n```{target.language}\n{code}\n```"
    )


# ---------------------------------------------------------------------------
# Scanner class
# ---------------------------------------------------------------------------

class LLMSASTScanner:
    """LLM-powered SAST scanner combining pattern matching with semantic analysis.

    This scanner runs in two phases:
    1. Fast pattern-matching phase using regex rules.
    2. Deep semantic analysis phase using an LLM.

    The pattern hits from phase 1 are provided as hints to the LLM in phase 2,
    reducing false negatives and improving precision.

    Example usage::

        scanner = LLMSASTScanner(model="claude-sonnet-4-6")
        result = scanner.scan_file("/app/auth.py", code)
        report = scanner.scan_files({"app/auth.py": auth_code, "app/db.py": db_code})
    """

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
        enable_pattern_phase: bool = True,
        min_severity: Severity = Severity.INFO,
        deduplicate: bool = True,
    ):
        self.model = model
        self.enable_pattern_phase = enable_pattern_phase
        self.min_severity = min_severity
        self.deduplicate = deduplicate
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )
        self._scan_cache: dict[str, ScanResult] = {}

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _filter_severity(self, findings: list[SASTFinding]) -> list[SASTFinding]:
        severity_order = [s.value for s in Severity]
        min_idx = severity_order.index(self.min_severity.value)
        return [f for f in findings if severity_order.index(f.severity.value) <= min_idx]

    def _deduplicate_findings(self, findings: list[SASTFinding]) -> list[SASTFinding]:
        seen: set[tuple] = set()
        unique: list[SASTFinding] = []
        for f in findings:
            key = (f.file_path, f.line_start, f.cwe, f.title)
            if key not in seen:
                seen.add(key)
                unique.append(f)
        return unique

    def _parse_llm_response(
        self, raw: str, file_path: str
    ) -> tuple[list[SASTFinding], dict]:
        """Parse LLM JSON response into SASTFinding objects."""
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", raw, re.DOTALL)
            if match:
                try:
                    data = json.loads(match.group(1))
                except json.JSONDecodeError:
                    logger.warning("LLMSASTScanner: JSON parse failed for %s", file_path)
                    return [], {}
            else:
                logger.warning("LLMSASTScanner: no JSON found in response for %s", file_path)
                return [], {}

        raw_findings = data.get("findings", [])
        scan_summary = data.get("scan_summary", {})
        findings = []
        for rf in raw_findings:
            try:
                f = SASTFinding.from_dict(rf)
                f.source = "llm"
                findings.append(f)
            except Exception as exc:  # noqa: BLE001
                logger.debug("LLMSASTScanner: skipping malformed finding: %s", exc)
        return findings, scan_summary

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def scan_file(self, file_path: str, content: str) -> ScanResult:
        """Scan a single file for security vulnerabilities.

        Args:
            file_path: Path to the source file.
            content: Source code content.

        Returns:
            ScanResult with all detected findings.
        """
        target = ScanTarget(file_path=file_path, content=content)

        # Check cache
        cache_key = target.content_hash
        if cache_key in self._scan_cache:
            logger.debug("LLMSASTScanner: cache hit for %s", file_path)
            return self._scan_cache[cache_key]

        start_time = time.monotonic()
        result = ScanResult(target=target, status=ScanStatus.RUNNING)

        # Phase 1: pattern matching
        pattern_hits: list[dict] = []
        if self.enable_pattern_phase:
            pattern_hits = _run_pattern_rules(content, file_path, target.language)
            result.patterns_matched = len(pattern_hits)
            logger.debug(
                "LLMSASTScanner: pattern phase file=%s hits=%d",
                file_path, len(pattern_hits)
            )

        # Phase 2: LLM semantic analysis
        user_prompt = _build_scan_prompt(target, pattern_hits)
        logger.info(
            "LLMSASTScanner.scan_file file=%s language=%s lines=%d",
            file_path, target.language, target.line_count,
        )

        response = self._client.messages.create(
            model=self.model,
            system=SAST_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        findings, scan_summary = self._parse_llm_response(raw, file_path)
        result.analysis_confidence = scan_summary.get("analysis_confidence", "MEDIUM")

        if self.deduplicate:
            findings = self._deduplicate_findings(findings)
        findings = self._filter_severity(findings)

        result.findings = findings
        result.status = ScanStatus.COMPLETED
        result.scan_duration_ms = (time.monotonic() - start_time) * 1000

        self._scan_cache[cache_key] = result
        return result

    def scan_files(
        self,
        files: dict[str, str],
        scan_id: str | None = None,
    ) -> SASTScanReport:
        """Scan multiple files and produce an aggregated report.

        Args:
            files: Mapping of file_path -> source code content.
            scan_id: Optional scan identifier.

        Returns:
            SASTScanReport with all results and statistics.
        """
        import uuid
        report = SASTScanReport(
            scan_id=scan_id or str(uuid.uuid4()),
            targets_scanned=len(files),
        )

        for path, content in files.items():
            try:
                result = self.scan_file(path, content)
                report.scan_results.append(result)
                report.total_findings += result.finding_count
                report.critical += result.critical_count
                report.high += result.high_count
                report.medium += result.medium_count
            except Exception as exc:  # noqa: BLE001
                logger.error("LLMSASTScanner.scan_files error file=%s: %s", path, exc)
                report.errors.append(f"{path}: {exc}")

        report.completed_at = time.time()
        return report

    def scan_directory(
        self,
        directory: str,
        extensions: set[str] | None = None,
        max_files: int = 100,
    ) -> SASTScanReport:
        """Scan all source files in a directory.

        Args:
            directory: Root directory to scan.
            extensions: File extensions to include (e.g., {".py", ".js"}).
            max_files: Maximum number of files to scan.

        Returns:
            SASTScanReport with results.
        """
        exts = extensions or {".py", ".js", ".ts", ".go", ".java", ".rb", ".php"}
        files: dict[str, str] = {}
        root = Path(directory)

        for path in root.rglob("*"):
            if len(files) >= max_files:
                break
            if path.is_file() and path.suffix.lower() in exts:
                try:
                    content = path.read_text(encoding="utf-8", errors="replace")
                    if len(content.encode()) <= MAX_FILE_BYTES:
                        files[str(path)] = content
                except OSError as exc:
                    logger.warning("LLMSASTScanner: cannot read %s: %s", path, exc)

        logger.info(
            "LLMSASTScanner.scan_directory dir=%s files=%d", directory, len(files)
        )
        return self.scan_files(files)
