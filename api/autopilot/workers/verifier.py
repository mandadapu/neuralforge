"""Verifier worker — verifies that applied fixes actually remediate vulnerabilities.

After a fix has been applied to a repository, the verifier re-analyzes the
patched code to confirm the vulnerability has been remediated and no regressions
have been introduced.
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
from typing import Any, Callable

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 2048

# ---------------------------------------------------------------------------
# Enumerations and constants
# ---------------------------------------------------------------------------

class VerificationStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    REMEDIATED = "remediated"
    PARTIAL = "partial"
    FAILED = "failed"
    REGRESSION = "regression"
    SKIPPED = "skipped"


class RegressionSeverity(str, Enum):
    CRITICAL = "CRITICAL"
    HIGH = "HIGH"
    MEDIUM = "MEDIUM"
    LOW = "LOW"
    NONE = "NONE"


VERIFY_SYSTEM_PROMPT = """\
You are a security engineer verifying that a code fix has successfully remediated
a security vulnerability. You will be given:
1. The original finding that was fixed
2. The original vulnerable code
3. The patched code after the fix was applied

Your task:
1. Confirm whether the vulnerability has been fully remediated (TRUE/FALSE)
2. Identify any regressions introduced by the fix
3. Check if the fix is complete or only partial
4. Assess overall fix quality

Respond with a JSON object:
{
  "remediated": <boolean>,
  "status": "remediated|partial|failed|regression",
  "confidence": "HIGH|MEDIUM|LOW",
  "verification_notes": "<detailed explanation>",
  "regressions": [
    {
      "title": "<regression title>",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "description": "<description>",
      "line": <int>
    }
  ],
  "remaining_issues": ["<issue>", ...],
  "fix_quality": "EXCELLENT|GOOD|ACCEPTABLE|POOR"
}
"""


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class VerificationTarget:
    """Input for verifying a security fix."""
    finding: dict
    original_file_path: str
    original_content: str
    patched_content: str
    related_files_original: dict[str, str] = field(default_factory=dict)
    related_files_patched: dict[str, str] = field(default_factory=dict)
    fix_description: str = ""
    pr_number: int | None = None
    repo_name: str = ""

    @property
    def finding_id(self) -> str:
        return str(self.finding.get("id", ""))

    @property
    def severity(self) -> str:
        return str(self.finding.get("severity", "UNKNOWN"))

    @property
    def language(self) -> str:
        ext = Path(self.original_file_path).suffix.lower()
        mapping = {
            ".py": "python", ".js": "javascript", ".ts": "typescript",
            ".go": "go", ".java": "java", ".rb": "ruby",
        }
        return mapping.get(ext, "text")


@dataclass
class RegressionFinding:
    """A regression introduced by the applied fix."""
    title: str
    severity: RegressionSeverity
    description: str
    line: int = 0

    @classmethod
    def from_dict(cls, data: dict) -> "RegressionFinding":
        sev_str = data.get("severity", "LOW").upper()
        try:
            sev = RegressionSeverity(sev_str)
        except ValueError:
            sev = RegressionSeverity.LOW
        return cls(
            title=data.get("title", ""),
            severity=sev,
            description=data.get("description", ""),
            line=data.get("line", 0),
        )

    def to_dict(self) -> dict:
        return {
            "title": self.title,
            "severity": self.severity.value,
            "description": self.description,
            "line": self.line,
        }


@dataclass
class VerificationResult:
    """Result of verifying a security fix."""
    finding_id: str
    status: VerificationStatus
    remediated: bool
    confidence: str
    verification_notes: str
    regressions: list[RegressionFinding] = field(default_factory=list)
    remaining_issues: list[str] = field(default_factory=list)
    fix_quality: str = "ACCEPTABLE"
    verified_at: float = field(default_factory=time.time)
    error: str | None = None

    @property
    def has_regressions(self) -> bool:
        return len(self.regressions) > 0

    @property
    def has_critical_regression(self) -> bool:
        return any(r.severity == RegressionSeverity.CRITICAL for r in self.regressions)

    @property
    def is_fully_remediated(self) -> bool:
        return self.remediated and not self.has_regressions

    @classmethod
    def from_dict(cls, data: dict, finding_id: str) -> "VerificationResult":
        remediated = bool(data.get("remediated", False))
        status_str = data.get("status", "failed")
        try:
            status = VerificationStatus(status_str)
        except ValueError:
            status = VerificationStatus.FAILED if not remediated else VerificationStatus.REMEDIATED

        regressions = [
            RegressionFinding.from_dict(r) for r in data.get("regressions", [])
        ]

        return cls(
            finding_id=finding_id,
            status=status,
            remediated=remediated,
            confidence=data.get("confidence", "LOW"),
            verification_notes=data.get("verification_notes", ""),
            regressions=regressions,
            remaining_issues=data.get("remaining_issues", []),
            fix_quality=data.get("fix_quality", "ACCEPTABLE"),
        )

    def to_dict(self) -> dict:
        return {
            "finding_id": self.finding_id,
            "status": self.status.value,
            "remediated": self.remediated,
            "confidence": self.confidence,
            "verification_notes": self.verification_notes,
            "regressions": [r.to_dict() for r in self.regressions],
            "remaining_issues": self.remaining_issues,
            "fix_quality": self.fix_quality,
            "verified_at": self.verified_at,
        }


@dataclass
class BatchVerificationSummary:
    """Summary of batch verification results."""
    total: int = 0
    remediated: int = 0
    partial: int = 0
    failed: int = 0
    regressions_found: int = 0
    results: list[VerificationResult] = field(default_factory=list)
    started_at: float = field(default_factory=time.time)
    completed_at: float = 0.0

    @property
    def success_rate(self) -> float:
        if not self.total:
            return 0.0
        return self.remediated / self.total

    @property
    def duration_seconds(self) -> float:
        if self.completed_at:
            return self.completed_at - self.started_at
        return 0.0


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _compute_diff_summary(original: str, patched: str) -> str:
    """Compute a simple diff summary between original and patched content."""
    orig_lines = original.splitlines()
    patch_lines = patched.splitlines()
    removed = [l for l in orig_lines if l not in set(patch_lines)]
    added = [l for l in patch_lines if l not in set(orig_lines)]
    return (
        f"Lines removed: {len(removed)}\n"
        f"Lines added: {len(added)}\n"
        f"Net change: {len(patch_lines) - len(orig_lines)}"
    )


def _truncate(content: str, max_chars: int = 8000) -> str:
    if len(content) <= max_chars:
        return content
    half = max_chars // 2
    return content[:half] + "\n... [truncated] ...\n" + content[-half:]


def _build_verification_prompt(target: VerificationTarget) -> str:
    """Build the user prompt for verification."""
    diff_summary = _compute_diff_summary(
        target.original_content, target.patched_content
    )
    return (
        f"Original finding:\n{json.dumps(target.finding, indent=2)}\n\n"
        f"Fix description: {target.fix_description}\n\n"
        f"File: {target.original_file_path}\n"
        f"Language: {target.language}\n"
        f"Diff summary:\n{diff_summary}\n\n"
        f"Original code:\n```{target.language}\n{_truncate(target.original_content)}\n```\n\n"
        f"Patched code:\n```{target.language}\n{_truncate(target.patched_content)}\n```"
    )


# ---------------------------------------------------------------------------
# Retry and resilience helpers
# ---------------------------------------------------------------------------

def _with_retry(fn: Callable, max_attempts: int = 3, delay_seconds: float = 1.0) -> Any:
    """Execute fn with simple retry logic."""
    last_exc: Exception | None = None
    for attempt in range(max_attempts):
        try:
            return fn()
        except Exception as exc:  # noqa: BLE001
            last_exc = exc
            if attempt < max_attempts - 1:
                time.sleep(delay_seconds * (attempt + 1))
            logger.warning(
                "_with_retry: attempt %d/%d failed: %s", attempt + 1, max_attempts, exc
            )
    raise last_exc  # type: ignore[misc]


class VerificationCache:
    """Simple in-memory cache for verification results."""

    def __init__(self, ttl_seconds: float = 3600.0):
        self._cache: dict[str, tuple[VerificationResult, float]] = {}
        self.ttl = ttl_seconds

    def _key(self, finding_id: str, content_hash: str) -> str:
        return f"{finding_id}:{content_hash}"

    def get(self, finding_id: str, content_hash: str) -> VerificationResult | None:
        key = self._key(finding_id, content_hash)
        if key in self._cache:
            result, ts = self._cache[key]
            if time.time() - ts < self.ttl:
                return result
            del self._cache[key]
        return None

    def set(self, finding_id: str, content_hash: str, result: VerificationResult) -> None:
        key = self._key(finding_id, content_hash)
        self._cache[key] = (result, time.time())

    def invalidate(self, finding_id: str) -> None:
        to_delete = [k for k in self._cache if k.startswith(f"{finding_id}:")]
        for k in to_delete:
            del self._cache[k]


class VerificationMetrics:
    """Tracks verification metrics for monitoring."""

    def __init__(self):
        self.total_verifications: int = 0
        self.successful_remediations: int = 0
        self.regressions_detected: int = 0
        self.cache_hits: int = 0
        self.total_duration_ms: float = 0.0

    def record(self, result: VerificationResult, duration_ms: float, cache_hit: bool) -> None:
        self.total_verifications += 1
        if result.remediated:
            self.successful_remediations += 1
        if result.has_regressions:
            self.regressions_detected += 1
        if cache_hit:
            self.cache_hits += 1
        self.total_duration_ms += duration_ms

    def to_dict(self) -> dict:
        avg_ms = (
            self.total_duration_ms / self.total_verifications
            if self.total_verifications > 0
            else 0.0
        )
        return {
            "total_verifications": self.total_verifications,
            "successful_remediations": self.successful_remediations,
            "regressions_detected": self.regressions_detected,
            "cache_hits": self.cache_hits,
            "avg_duration_ms": round(avg_ms, 2),
        }


# ---------------------------------------------------------------------------
# Main verifier worker
# ---------------------------------------------------------------------------

class VerifierWorker:
    """Worker that verifies security fixes have been correctly applied.

    The verifier re-analyzes patched code to confirm:
    1. The original vulnerability is no longer present.
    2. No new vulnerabilities (regressions) have been introduced.
    3. The fix is complete and of acceptable quality.

    Example::

        worker = VerifierWorker()
        result = worker.verify(verification_target)
        if result.is_fully_remediated:
            mark_finding_resolved(result.finding_id)
        elif result.has_critical_regression:
            revert_fix(result.finding_id)
    """

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
        enable_cache: bool = True,
        max_retries: int = 2,
    ):
        self.model = model
        self.enable_cache = enable_cache
        self.max_retries = max_retries
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )
        self._cache = VerificationCache() if enable_cache else None
        self._metrics = VerificationMetrics()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _content_hash(self, target: VerificationTarget) -> str:
        h = hashlib.sha256()
        h.update(target.original_content.encode())
        h.update(target.patched_content.encode())
        return h.hexdigest()[:16]

    def _parse_response(
        self, raw: str, finding_id: str
    ) -> VerificationResult:
        """Parse LLM verification response."""
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", raw, re.DOTALL)
            if match:
                try:
                    data = json.loads(match.group(1))
                except json.JSONDecodeError:
                    data = {}
            else:
                data = {}

        if not data:
            return VerificationResult(
                finding_id=finding_id,
                status=VerificationStatus.FAILED,
                remediated=False,
                confidence="LOW",
                verification_notes="Failed to parse LLM response",
                error="JSON parse failure",
            )

        return VerificationResult.from_dict(data, finding_id)

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def verify(self, target: VerificationTarget) -> VerificationResult:
        """Verify that a security fix has been correctly applied.

        Args:
            target: VerificationTarget containing original and patched code.

        Returns:
            VerificationResult with remediation status and regression findings.
        """
        finding_id = target.finding_id
        content_hash = self._content_hash(target)

        # Check cache
        cache_hit = False
        if self._cache is not None:
            cached = self._cache.get(finding_id, content_hash)
            if cached is not None:
                logger.debug("VerifierWorker.verify: cache hit finding_id=%s", finding_id)
                self._metrics.record(cached, 0.0, cache_hit=True)
                return cached

        start = time.monotonic()
        user_prompt = _build_verification_prompt(target)

        logger.info(
            "VerifierWorker.verify finding_id=%s file=%s severity=%s",
            finding_id, target.original_file_path, target.severity,
        )

        response = self._client.messages.create(
            model=self.model,
            system=VERIFY_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=2048,
        )

        raw = response.content[0].text
        result = self._parse_response(raw, finding_id)
        elapsed_ms = (time.monotonic() - start) * 1000

        # Cache the result
        if self._cache is not None:
            self._cache.set(finding_id, content_hash, result)

        self._metrics.record(result, elapsed_ms, cache_hit=False)

        logger.info(
            "VerifierWorker.verify: result finding_id=%s status=%s remediated=%s regressions=%d",
            finding_id, result.status.value, result.remediated, len(result.regressions),
        )
        return result

    def verify_batch(
        self,
        targets: list[VerificationTarget],
    ) -> BatchVerificationSummary:
        """Verify a batch of security fixes.

        Args:
            targets: List of VerificationTarget objects.

        Returns:
            BatchVerificationSummary with aggregated statistics.
        """
        summary = BatchVerificationSummary(total=len(targets))

        for target in targets:
            try:
                result = self.verify(target)
                summary.results.append(result)
                if result.status == VerificationStatus.REMEDIATED:
                    summary.remediated += 1
                elif result.status == VerificationStatus.PARTIAL:
                    summary.partial += 1
                else:
                    summary.failed += 1
                if result.has_regressions:
                    summary.regressions_found += 1
            except Exception as exc:  # noqa: BLE001
                logger.error(
                    "VerifierWorker.verify_batch: error finding_id=%s: %s",
                    target.finding_id, exc,
                )
                summary.results.append(
                    VerificationResult(
                        finding_id=target.finding_id,
                        status=VerificationStatus.FAILED,
                        remediated=False,
                        confidence="LOW",
                        verification_notes="",
                        error=str(exc),
                    )
                )
                summary.failed += 1

        summary.completed_at = time.time()
        logger.info(
            "VerifierWorker.verify_batch: complete total=%d remediated=%d failed=%d regressions=%d",
            summary.total, summary.remediated, summary.failed, summary.regressions_found,
        )
        return summary

    def get_metrics(self) -> dict:
        """Return current verification metrics."""
        return self._metrics.to_dict()
