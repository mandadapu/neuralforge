"""LLM probe chains — composable chains of LLM probes for security analysis.

Probe chains allow multiple LLM calls to be chained together, where the output
of one probe feeds into the next. This enables multi-step reasoning pipelines
for complex security analysis tasks.
"""

from __future__ import annotations

import json
import logging
import os
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable

import anthropic

logger = logging.getLogger(__name__)

DEFAULT_MAX_TOKENS = 4096

# ---------------------------------------------------------------------------
# Enumerations and types
# ---------------------------------------------------------------------------

class ChainStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    ABORTED = "aborted"


class ProbeType(str, Enum):
    DETECTION = "detection"
    CLASSIFICATION = "classification"
    VERIFICATION = "verification"
    REMEDIATION = "remediation"
    SUMMARIZATION = "summarization"
    EXTRACTION = "extraction"
    REASONING = "reasoning"
    CRITIQUE = "critique"


# ---------------------------------------------------------------------------
# Probe definitions
# ---------------------------------------------------------------------------

@dataclass
class ProbeDefinition:
    """Defines a single LLM probe step in a chain."""
    name: str
    probe_type: ProbeType
    system_prompt: str
    user_prompt_template: str
    output_key: str
    max_tokens: int = DEFAULT_MAX_TOKENS
    temperature: float = 0.0
    required_input_keys: list[str] = field(default_factory=list)
    optional_input_keys: list[str] = field(default_factory=list)
    transform_output: Callable[[str], Any] | None = None
    condition: Callable[[dict], bool] | None = None  # Skip if returns False

    def should_run(self, context: dict) -> bool:
        if self.condition is not None:
            return self.condition(context)
        return True

    def build_user_prompt(self, context: dict) -> str:
        try:
            return self.user_prompt_template.format(**context)
        except KeyError as exc:
            logger.warning("ProbeDefinition.build_user_prompt: missing key %s", exc)
            return self.user_prompt_template


@dataclass
class ProbeResult:
    """Result of executing a single probe."""
    probe_name: str
    output_key: str
    raw_response: str
    parsed_output: Any
    tokens_used: int = 0
    duration_ms: float = 0.0
    error: str | None = None

    @property
    def succeeded(self) -> bool:
        return self.error is None


@dataclass
class ChainResult:
    """Result of executing a full probe chain."""
    chain_name: str
    status: ChainStatus
    probe_results: list[ProbeResult] = field(default_factory=list)
    final_output: Any = None
    context: dict = field(default_factory=dict)
    error: str | None = None
    started_at: float = field(default_factory=time.time)
    completed_at: float = 0.0

    @property
    def duration_seconds(self) -> float:
        if self.completed_at:
            return self.completed_at - self.started_at
        return 0.0

    @property
    def total_tokens(self) -> int:
        return sum(r.tokens_used for r in self.probe_results)

    @property
    def succeeded_probes(self) -> list[ProbeResult]:
        return [r for r in self.probe_results if r.succeeded]

    @property
    def failed_probes(self) -> list[ProbeResult]:
        return [r for r in self.probe_results if not r.succeeded]


# ---------------------------------------------------------------------------
# Built-in probe library
# ---------------------------------------------------------------------------

def _make_detection_probe(name: str = "detect") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.DETECTION,
        system_prompt=(
            "You are a security detection engine. Analyze the input for security vulnerabilities. "
            "Respond with a JSON array of finding objects with keys: title, severity, cwe, description."
        ),
        user_prompt_template="{code}",
        output_key="raw_findings",
        max_tokens=4096,
    )


def _make_classification_probe(name: str = "classify") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.CLASSIFICATION,
        system_prompt=(
            "You are a security classification expert. Classify the provided findings by "
            "severity (CRITICAL/HIGH/MEDIUM/LOW/INFO), CWE, and attack category. "
            "Respond with a JSON array of classified findings."
        ),
        user_prompt_template="{raw_findings}",
        output_key="classified_findings",
        required_input_keys=["raw_findings"],
        max_tokens=4096,
    )


def _make_verification_probe(name: str = "verify") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.VERIFICATION,
        system_prompt=(
            "You are a security verification expert. For each finding, determine if it is "
            "a TRUE POSITIVE or FALSE POSITIVE. Respond with JSON."
        ),
        user_prompt_template="{classified_findings}",
        output_key="verified_findings",
        required_input_keys=["classified_findings"],
        max_tokens=2048,
    )


def _make_summarization_probe(name: str = "summarize") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.SUMMARIZATION,
        system_prompt=(
            "You are a security analyst. Summarize the verified findings into a concise "
            "executive summary. Include total counts, most critical issues, and top recommendations."
        ),
        user_prompt_template="{verified_findings}",
        output_key="summary",
        required_input_keys=["verified_findings"],
        max_tokens=2048,
    )


def _make_remediation_probe(name: str = "remediate") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.REMEDIATION,
        system_prompt=(
            "You are a security engineer. For each finding, provide a specific, actionable "
            "remediation recommendation. Consider the code context and best practices. "
            "Respond with JSON."
        ),
        user_prompt_template="{verified_findings}\n\nCode context:\n{code}",
        output_key="remediation_plan",
        required_input_keys=["verified_findings", "code"],
        max_tokens=4096,
    )


def _make_critique_probe(name: str = "critique") -> ProbeDefinition:
    return ProbeDefinition(
        name=name,
        probe_type=ProbeType.CRITIQUE,
        system_prompt=(
            "You are a senior security auditor. Critically review the analysis output for "
            "completeness, accuracy, and missed vulnerabilities. "
            "Respond with JSON: {missed_findings, quality_score, critique_notes}."
        ),
        user_prompt_template=(
            "Original code:\n{code}\n\nAnalysis output:\n{verified_findings}"
        ),
        output_key="critique",
        required_input_keys=["code", "verified_findings"],
        max_tokens=4096,
    )


# ---------------------------------------------------------------------------
# Built-in chain templates
# ---------------------------------------------------------------------------

DETECT_AND_CLASSIFY_CHAIN = [
    _make_detection_probe(),
    _make_classification_probe(),
]

FULL_ANALYSIS_CHAIN = [
    _make_detection_probe(),
    _make_classification_probe(),
    _make_verification_probe(),
    _make_summarization_probe(),
]

DETECT_REMEDIATE_CHAIN = [
    _make_detection_probe(),
    _make_classification_probe(),
    _make_verification_probe(),
    _make_remediation_probe(),
]

FULL_AUDIT_CHAIN = [
    _make_detection_probe(),
    _make_classification_probe(),
    _make_verification_probe(),
    _make_remediation_probe(),
    _make_critique_probe(),
]


# ---------------------------------------------------------------------------
# Output transformers
# ---------------------------------------------------------------------------

def _parse_json_list(raw: str) -> list:
    try:
        result = json.loads(raw)
        if isinstance(result, list):
            return result
        return result.get("findings", result.get("results", [raw]))
    except json.JSONDecodeError:
        return []


def _parse_json_object(raw: str) -> dict:
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {"raw": raw}


def _pass_through(raw: str) -> str:
    return raw


# ---------------------------------------------------------------------------
# Chain execution engine
# ---------------------------------------------------------------------------

class ChainExecutionError(Exception):
    """Raised when a chain execution encounters an unrecoverable error."""
    pass


class ProbeChainExecutor:
    """Executes a sequence of LLM probes as a chain.

    Context flows between probes: each probe's output is stored in the context
    under its output_key and is available to subsequent probes via template vars.

    Example::

        executor = ProbeChainExecutor()
        result = executor.run(
            chain=FULL_ANALYSIS_CHAIN,
            initial_context={"code": source_code},
            chain_name="full_analysis",
        )
        summary = result.context.get("summary")
    """

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
        abort_on_failure: bool = True,
    ):
        self.model = model
        self.abort_on_failure = abort_on_failure
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _execute_probe(
        self,
        probe: ProbeDefinition,
        context: dict,
    ) -> ProbeResult:
        """Execute a single probe and return its result."""
        start = time.monotonic()

        if not probe.should_run(context):
            logger.debug("ProbeChainExecutor: skipping probe=%s", probe.name)
            return ProbeResult(
                probe_name=probe.name,
                output_key=probe.output_key,
                raw_response="",
                parsed_output=None,
                error="skipped",
            )

        user_prompt = probe.build_user_prompt(context)

        logger.debug(
            "ProbeChainExecutor: executing probe=%s type=%s",
            probe.name, probe.probe_type.value,
        )

        response = self._client.messages.create(
            model=self.model,
            system=probe.system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=probe.max_tokens,
        )

        raw = response.content[0].text
        tokens = getattr(response.usage, "output_tokens", 0)
        elapsed_ms = (time.monotonic() - start) * 1000

        if probe.transform_output is not None:
            try:
                parsed = probe.transform_output(raw)
            except Exception as exc:  # noqa: BLE001
                parsed = raw
                logger.warning("ProbeChainExecutor: transform failed probe=%s: %s", probe.name, exc)
        else:
            parsed = raw

        return ProbeResult(
            probe_name=probe.name,
            output_key=probe.output_key,
            raw_response=raw,
            parsed_output=parsed,
            tokens_used=tokens,
            duration_ms=elapsed_ms,
        )

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def run(
        self,
        chain: list[ProbeDefinition],
        initial_context: dict,
        chain_name: str = "unnamed",
    ) -> ChainResult:
        """Run a probe chain with the given initial context.

        Args:
            chain: List of ProbeDefinition objects defining the chain steps.
            initial_context: Initial context dict (e.g., {"code": source_code}).
            chain_name: Name for the chain (used in logging).

        Returns:
            ChainResult with all probe results and final context.
        """
        result = ChainResult(
            chain_name=chain_name,
            status=ChainStatus.RUNNING,
            context=dict(initial_context),
        )

        logger.info(
            "ProbeChainExecutor.run chain=%s steps=%d", chain_name, len(chain)
        )

        for probe in chain:
            probe_result = self._execute_probe(probe, result.context)
            result.probe_results.append(probe_result)

            if probe_result.error and probe_result.error != "skipped":
                logger.error(
                    "ProbeChainExecutor: probe failed chain=%s probe=%s error=%s",
                    chain_name, probe.name, probe_result.error,
                )
                if self.abort_on_failure:
                    result.status = ChainStatus.ABORTED
                    result.error = f"Probe {probe.name} failed: {probe_result.error}"
                    result.completed_at = time.time()
                    return result
            elif probe_result.parsed_output is not None:
                # Store output in context for downstream probes
                output = probe_result.parsed_output
                if isinstance(output, (list, dict)):
                    result.context[probe.output_key] = json.dumps(output, indent=2)
                else:
                    result.context[probe.output_key] = str(output)

        result.status = ChainStatus.COMPLETED
        result.final_output = result.context.get(chain[-1].output_key) if chain else None
        result.completed_at = time.time()

        logger.info(
            "ProbeChainExecutor.run: complete chain=%s probes=%d tokens=%d duration=%.2fs",
            chain_name, len(chain), result.total_tokens, result.duration_seconds,
        )
        return result

    def run_parallel(
        self,
        chains: list[tuple[list[ProbeDefinition], dict, str]],
    ) -> list[ChainResult]:
        """Run multiple chains sequentially (parallel execution via threads is optional).

        Args:
            chains: List of (chain, initial_context, chain_name) tuples.

        Returns:
            List of ChainResult objects.
        """
        return [
            self.run(chain, context, name)
            for chain, context, name in chains
        ]
