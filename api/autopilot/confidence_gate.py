"""Confidence gate — evaluates whether autopilot findings meet confidence thresholds.

The confidence gate is a critical component of the autopilot pipeline that
prevents low-confidence findings from being automatically actioned. It uses
an LLM to re-evaluate borderline findings and determine whether they should
proceed through the automated fix workflow or be escalated for human review.
"""

from __future__ import annotations

import json
import logging
import os
import re
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 2048

# Severities that must never auto-proceed without human review (SOX control)
_HUMAN_REVIEW_REQUIRED_SEVERITIES = {"CRITICAL", "HIGH"}

# Fields allowed in finding data sent to external LLM (data minimization, GDPR Art. 25)
_FINDING_ALLOWED_FIELDS = {
    "id", "severity", "title", "description", "cwe", "rule_id",
    "category", "confidence", "file", "line", "column",
}

# Sensitive value patterns redacted before LLM transmission (GDPR/HIPAA/PCI-DSS)
_SENSITIVE_PATTERNS: list[tuple[re.Pattern, str]] = [
    (re.compile(r"\b(?:\d[ -]?){12,18}\d\b"), "[REDACTED-PAN]"),
    (re.compile(r"\b\d{3}-\d{2}-\d{4}\b"), "[REDACTED-SSN]"),
    (re.compile(r"[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}"), "[REDACTED-EMAIL]"),
    (re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----", re.DOTALL), "[REDACTED-PRIVATE-KEY]"),
    (re.compile(r'(?i)(token|api_?key|secret|password|passwd)\s*[=:]\s*["\']?[A-Za-z0-9_\-./+]{20,}["\']?'), r"\1=[REDACTED-SECRET]"),
]


def _redact_sensitive_data(text: str) -> str:
    """Redact PII/PHI/PAN/secrets from text before external LLM transmission."""
    for pattern, replacement in _SENSITIVE_PATTERNS:
        text = pattern.sub(replacement, text)
    return text


def _minimize_finding_for_llm(finding: dict) -> dict:
    """Return a data-minimized copy of the finding for LLM transmission.

    Only allowed fields are forwarded (GDPR Art. 25 data minimization).
    Remaining string values are redacted of sensitive patterns.
    """
    minimized: dict = {}
    for key in _FINDING_ALLOWED_FIELDS:
        if key in finding:
            value = finding[key]
            if isinstance(value, str):
                minimized[key] = _redact_sensitive_data(value)
            else:
                minimized[key] = value
    return minimized


def _emit_audit_event(event_type: str, **fields: Any) -> None:
    """Emit a structured audit event for compliance trail.

    All external LLM transmissions and gate decisions must be recorded here.
    These log records must be forwarded to an immutable audit store (GDPR Art. 32,
    SOX Section 404, HIPAA audit controls).
    """
    event = {
        "audit_event": event_type,
        "timestamp": time.time(),
        **fields,
    }
    logger.warning("AUDIT %s", json.dumps(event))

# ---------------------------------------------------------------------------
# Enumerations and constants
# ---------------------------------------------------------------------------

class GateDecision(str, Enum):
    PASS = "PASS"
    FAIL = "FAIL"
    ESCALATE = "ESCALATE"
    DEFER = "DEFER"


class ConfidenceLevel(str, Enum):
    HIGH = "HIGH"
    MEDIUM = "MEDIUM"
    LOW = "LOW"
    UNCERTAIN = "UNCERTAIN"


# Severity-based minimum confidence thresholds
SEVERITY_CONFIDENCE_THRESHOLDS: dict[str, ConfidenceLevel] = {
    "CRITICAL": ConfidenceLevel.HIGH,
    "HIGH": ConfidenceLevel.HIGH,
    "MEDIUM": ConfidenceLevel.MEDIUM,
    "LOW": ConfidenceLevel.LOW,
    "INFO": ConfidenceLevel.LOW,
}

_CONFIDENCE_ORDER = [
    ConfidenceLevel.UNCERTAIN,
    ConfidenceLevel.LOW,
    ConfidenceLevel.MEDIUM,
    ConfidenceLevel.HIGH,
]

GATE_SYSTEM_PROMPT = """\
You are a security triage expert evaluating whether an automated security finding is
ready for automated remediation or requires human review.

Assess the finding's confidence level based on:
1. Specificity: Does the finding point to a concrete, exploitable vulnerability?
2. Context: Is there sufficient code context to confirm the issue?
3. False positive risk: How likely is this to be a false positive?
4. Fix complexity: Is the recommended fix straightforward and safe to apply automatically?

Respond with a JSON object:
{
  "decision": "PASS|FAIL|ESCALATE|DEFER",
  "confidence": "HIGH|MEDIUM|LOW|UNCERTAIN",
  "reasoning": "<explanation>",
  "risk_score": <float 0.0-1.0>,
  "auto_fixable": <boolean>,
  "escalation_reason": "<reason if ESCALATE, else null>"
}
"""

BATCH_GATE_SYSTEM_PROMPT = """\
You are a security triage expert. Evaluate a batch of security findings and determine
which are ready for automated remediation (PASS), which should be rejected (FAIL),
which need human review (ESCALATE), and which should be deferred (DEFER).

For each finding, assess: specificity, false positive risk, and fix safety.

Respond with a JSON array, one object per finding:
[
  {
    "id": "<finding-id>",
    "decision": "PASS|FAIL|ESCALATE|DEFER",
    "confidence": "HIGH|MEDIUM|LOW|UNCERTAIN",
    "reasoning": "<explanation>",
    "risk_score": <float 0.0-1.0>,
    "auto_fixable": <boolean>
  }
]
"""


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class GateResult:
    """Result of a confidence gate evaluation for a single finding."""
    finding_id: str
    decision: GateDecision
    confidence: ConfidenceLevel
    reasoning: str
    risk_score: float = 0.5
    auto_fixable: bool = False
    escalation_reason: str | None = None
    evaluated_at: float = field(default_factory=time.time)

    @property
    def should_proceed(self) -> bool:
        return self.decision == GateDecision.PASS

    @property
    def needs_human_review(self) -> bool:
        return self.decision in (GateDecision.ESCALATE, GateDecision.DEFER)

    @classmethod
    def from_dict(cls, data: dict, finding_id: str = "") -> "GateResult":
        decision_str = data.get("decision", "FAIL").upper()
        try:
            decision = GateDecision(decision_str)
        except ValueError:
            decision = GateDecision.FAIL

        conf_str = data.get("confidence", "UNCERTAIN").upper()
        try:
            confidence = ConfidenceLevel(conf_str)
        except ValueError:
            confidence = ConfidenceLevel.UNCERTAIN

        return cls(
            finding_id=finding_id or data.get("id", ""),
            decision=decision,
            confidence=confidence,
            reasoning=data.get("reasoning", ""),
            risk_score=float(data.get("risk_score", 0.5)),
            auto_fixable=bool(data.get("auto_fixable", False)),
            escalation_reason=data.get("escalation_reason"),
        )


@dataclass
class GateConfig:
    """Configuration for the confidence gate."""
    model: str = "claude-sonnet-4-6"
    min_confidence: ConfidenceLevel = ConfidenceLevel.MEDIUM
    auto_fail_severity: list[str] = field(default_factory=list)
    auto_pass_severity: list[str] = field(default_factory=list)
    batch_size: int = 10
    require_auto_fixable: bool = False


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _meets_threshold(
    confidence: ConfidenceLevel, threshold: ConfidenceLevel
) -> bool:
    return _CONFIDENCE_ORDER.index(confidence) >= _CONFIDENCE_ORDER.index(threshold)


def _apply_rule_based_gate(
    finding: dict, config: GateConfig
) -> GateResult | None:
    """Apply fast rule-based gate checks before invoking the LLM."""
    finding_id = str(finding.get("id", ""))
    severity = finding.get("severity", "").upper()

    if severity in config.auto_fail_severity:
        return GateResult(
            finding_id=finding_id,
            decision=GateDecision.FAIL,
            confidence=ConfidenceLevel.HIGH,
            reasoning=f"Auto-fail rule applied for severity {severity}",
            risk_score=1.0,
        )

    if severity in config.auto_pass_severity:
        return GateResult(
            finding_id=finding_id,
            decision=GateDecision.PASS,
            confidence=ConfidenceLevel.HIGH,
            reasoning=f"Auto-pass rule applied for severity {severity}",
            risk_score=0.1,
            auto_fixable=True,
        )

    return None


# ---------------------------------------------------------------------------
# Confidence gate class
# ---------------------------------------------------------------------------

class ConfidenceGate:
    """Evaluates security findings for automated remediation readiness.

    Uses a combination of rule-based checks and LLM evaluation to determine
    whether a finding should proceed through the automated fix pipeline.

    Example::

        gate = ConfidenceGate(config=GateConfig(min_confidence=ConfidenceLevel.MEDIUM))
        result = gate.evaluate(finding)
        if result.should_proceed:
            trigger_auto_fix(finding)
    """

    def __init__(
        self,
        config: GateConfig | None = None,
        api_key: str | None = None,
    ):
        self.config = config or GateConfig()
        resolved_key = api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        # GDPR Art. 32: Validate key presence; rotation must be managed externally.
        if not resolved_key:
            logger.warning(
                "ANTHROPIC_API_KEY is not set. LLM gate evaluations will fail. "
                "Manage API keys via a secrets manager with a documented rotation policy."
            )
        self._client = anthropic.Anthropic(api_key=resolved_key)

    def evaluate(self, finding: dict) -> GateResult:
        """Evaluate a single finding through the confidence gate.

        Args:
            finding: Finding dictionary with at minimum id, severity, title, description.

        Returns:
            GateResult with decision and reasoning.
        """
        finding_id = str(finding.get("id", ""))
        severity = finding.get("severity", "").upper()

        # Fast path: rule-based gate
        rule_result = _apply_rule_based_gate(finding, self.config)
        if rule_result is not None:
            logger.debug(
                "ConfidenceGate.evaluate: rule-based decision finding_id=%s decision=%s",
                finding_id, rule_result.decision.value,
            )
            # SOX: rule-based PASS for HIGH/CRITICAL is still subject to
            # human-review enforcement downstream; emit audit event.
            _emit_audit_event(
                "gate_rule_decision",
                finding_id=finding_id,
                severity=severity,
                decision=rule_result.decision.value,
            )
            return rule_result

        # Data minimization: only send allowlisted fields, with sensitive
        # patterns redacted (GDPR Art. 25, HIPAA, PCI-DSS Req. 3.4)
        safe_finding = _minimize_finding_for_llm(finding)
        user_prompt = json.dumps(safe_finding, indent=2)
        logger.info("ConfidenceGate.evaluate finding_id=%s", finding_id)

        # Audit: record external LLM transmission
        _emit_audit_event(
            "llm_transmission",
            operation="evaluate",
            finding_id=finding_id,
            severity=severity,
            model=self.config.model,
            data_minimization_applied=True,
            fields_transmitted=list(safe_finding.keys()),
        )

        response = self._client.messages.create(
            model=self.config.model,
            system=GATE_SYSTEM_PROMPT,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=2048,
        )

        raw = response.content[0].text
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            logger.warning("ConfidenceGate: JSON parse failed for finding %s", finding_id)
            return GateResult(
                finding_id=finding_id,
                decision=GateDecision.ESCALATE,
                confidence=ConfidenceLevel.UNCERTAIN,
                reasoning="LLM response parse failure — escalating for human review",
            )

        result = GateResult.from_dict(data, finding_id)

        # Apply minimum confidence threshold
        threshold = SEVERITY_CONFIDENCE_THRESHOLDS.get(
            severity, ConfidenceLevel.MEDIUM
        )
        effective_threshold = max(
            _CONFIDENCE_ORDER.index(threshold),
            _CONFIDENCE_ORDER.index(self.config.min_confidence),
        )
        effective_threshold_level = _CONFIDENCE_ORDER[effective_threshold]

        if not _meets_threshold(result.confidence, effective_threshold_level):
            result.decision = GateDecision.ESCALATE
            result.escalation_reason = (
                f"Confidence {result.confidence.value} below threshold "
                f"{effective_threshold_level.value}"
            )

        if self.config.require_auto_fixable and not result.auto_fixable:
            if result.decision == GateDecision.PASS:
                result.decision = GateDecision.DEFER
                result.escalation_reason = "Not marked as auto-fixable"

        # SOX Section 404: HIGH/CRITICAL findings must not auto-proceed —
        # force ESCALATE so a human approval step is required downstream.
        # This prevents the bypass risk where a PASS from an LLM-only review
        # could allow automated changes to financial/critical code paths.
        if severity in _HUMAN_REVIEW_REQUIRED_SEVERITIES and result.decision == GateDecision.PASS:
            result.decision = GateDecision.ESCALATE
            result.escalation_reason = (
                f"Severity {severity} requires human review before automated action "
                f"(SOX segregation-of-duties control)"
            )

        _emit_audit_event(
            "gate_decision",
            finding_id=finding_id,
            severity=severity,
            decision=result.decision.value,
            confidence=result.confidence.value,
            risk_score=result.risk_score,
            escalation_reason=result.escalation_reason,
        )
        return result

    def evaluate_batch(self, findings: list[dict]) -> list[GateResult]:
        """Evaluate a batch of findings in a single LLM call.

        More efficient than calling evaluate() individually for large batches.

        Args:
            findings: List of finding dictionaries.

        Returns:
            List of GateResult objects in the same order as input findings.
        """
        if not findings:
            return []

        # Apply rule-based gate first
        results: dict[str, GateResult] = {}
        pending: list[dict] = []

        for finding in findings:
            rule_result = _apply_rule_based_gate(finding, self.config)
            if rule_result is not None:
                results[str(finding.get("id", ""))] = rule_result
            else:
                pending.append(finding)

        # Batch LLM evaluation for remaining findings
        if pending:
            for i in range(0, len(pending), self.config.batch_size):
                batch = pending[i:i + self.config.batch_size]
                # Data minimization before external transmission (GDPR Art. 25)
                safe_batch = [_minimize_finding_for_llm(f) for f in batch]
                user_prompt = json.dumps(safe_batch, indent=2)
                logger.info(
                    "ConfidenceGate.evaluate_batch size=%d offset=%d", len(batch), i
                )
                # Audit: record LLM transmission
                _emit_audit_event(
                    "llm_transmission",
                    operation="evaluate_batch",
                    batch_size=len(batch),
                    offset=i,
                    model=self.config.model,
                    data_minimization_applied=True,
                    finding_ids=[str(f.get("id", "")) for f in batch],
                )

                response = self._client.messages.create(
                    model=self.config.model,
                    system=BATCH_GATE_SYSTEM_PROMPT,
                    messages=[{"role": "user", "content": user_prompt}],
                    max_tokens=2048,
                )

                raw = response.content[0].text
                try:
                    batch_data = json.loads(raw)
                    if isinstance(batch_data, dict):
                        batch_data = batch_data.get("results", [])
                except json.JSONDecodeError:
                    logger.warning("ConfidenceGate.evaluate_batch: JSON parse failed")
                    batch_data = []

                for item in batch_data:
                    gate_result = GateResult.from_dict(item)
                    # SOX: HIGH/CRITICAL must not auto-proceed from batch evaluation
                    original_finding = next(
                        (f for f in batch if str(f.get("id", "")) == gate_result.finding_id),
                        {},
                    )
                    orig_severity = original_finding.get("severity", "").upper()
                    if orig_severity in _HUMAN_REVIEW_REQUIRED_SEVERITIES and gate_result.decision == GateDecision.PASS:
                        gate_result.decision = GateDecision.ESCALATE
                        gate_result.escalation_reason = (
                            f"Severity {orig_severity} requires human review before automated action "
                            f"(SOX segregation-of-duties control)"
                        )
                    results[gate_result.finding_id] = gate_result

        # Reconstruct ordered output
        output: list[GateResult] = []
        for finding in findings:
            fid = str(finding.get("id", ""))
            if fid in results:
                output.append(results[fid])
            else:
                output.append(
                    GateResult(
                        finding_id=fid,
                        decision=GateDecision.ESCALATE,
                        confidence=ConfidenceLevel.UNCERTAIN,
                        reasoning="No gate result returned — escalating",
                    )
                )
        return output

    def filter_actionable(self, findings: list[dict]) -> list[dict]:
        """Filter a list of findings to only those that pass the gate."""
        results = self.evaluate_batch(findings)
        return [
            f for f, r in zip(findings, results) if r.should_proceed
        ]
