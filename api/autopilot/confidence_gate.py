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
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 2048

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
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def evaluate(self, finding: dict) -> GateResult:
        """Evaluate a single finding through the confidence gate.

        Args:
            finding: Finding dictionary with at minimum id, severity, title, description.

        Returns:
            GateResult with decision and reasoning.
        """
        finding_id = str(finding.get("id", ""))

        # Fast path: rule-based gate
        rule_result = _apply_rule_based_gate(finding, self.config)
        if rule_result is not None:
            logger.debug(
                "ConfidenceGate.evaluate: rule-based decision finding_id=%s decision=%s",
                finding_id, rule_result.decision.value,
            )
            return rule_result

        # LLM-based evaluation
        user_prompt = json.dumps(finding, indent=2)
        logger.info("ConfidenceGate.evaluate finding_id=%s", finding_id)

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
            finding.get("severity", "").upper(), ConfidenceLevel.MEDIUM
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
                user_prompt = json.dumps(batch, indent=2)
                logger.info(
                    "ConfidenceGate.evaluate_batch size=%d offset=%d", len(batch), i
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
