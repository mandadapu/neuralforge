"""Generative LLM probes — probes that generate artifacts such as PoCs and exploit code.

These probes are used in controlled security research contexts to generate
proof-of-concept exploit code, attack payloads, and security test cases
for validating vulnerability detection and remediation systems.
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

DEFAULT_MAX_TOKENS = 4096

# ---------------------------------------------------------------------------
# Enumerations and constants
# ---------------------------------------------------------------------------

class GenerativeProbeType(str, Enum):
    POC_EXPLOIT = "poc_exploit"
    TEST_PAYLOAD = "test_payload"
    ATTACK_SCENARIO = "attack_scenario"
    UNIT_TEST = "unit_test"
    FUZZING_INPUT = "fuzzing_input"
    SECURITY_ASSERTION = "security_assertion"


POC_SYSTEM_PROMPT = """\
You are a security researcher in a controlled research environment generating
proof-of-concept code to demonstrate security vulnerabilities for defensive purposes.

Generate a minimal, safe PoC that demonstrates the vulnerability described.
The PoC should:
1. Be clearly commented explaining what it demonstrates
2. Include safety guards to prevent accidental misuse
3. Be scoped specifically to the described vulnerability
4. Include assertions that demonstrate the vulnerability exists

Respond with JSON:
{
  "poc_code": "<code>",
  "language": "<language>",
  "description": "<what the PoC demonstrates>",
  "prerequisites": ["<requirement>", ...],
  "expected_outcome": "<what happens when run>",
  "safety_notes": "<important safety information>"
}
"""

TEST_PAYLOAD_SYSTEM_PROMPT = """\
You are a security test engineer generating test payloads for validating
security controls. Generate payloads that test the specified vulnerability class.

Respond with JSON:
{
  "payloads": [
    {
      "id": "<id>",
      "payload": "<payload string or object>",
      "category": "<category>",
      "description": "<what this tests>",
      "expected_blocked": <boolean>
    }
  ],
  "vulnerability_class": "<class>",
  "test_notes": "<testing guidance>"
}
"""

UNIT_TEST_SYSTEM_PROMPT = """\
You are a security-focused test engineer generating unit tests to validate
that a security fix has been correctly implemented. Generate comprehensive
test cases including both positive (attack) and negative (legitimate use) cases.

Respond with JSON:
{
  "test_file": "<complete test file content>",
  "language": "<language>",
  "framework": "<test framework>",
  "test_cases": [
    {"name": "<name>", "type": "positive|negative", "description": "<desc>"}
  ]
}
"""

ATTACK_SCENARIO_SYSTEM_PROMPT = """\
You are a threat modeling expert. Generate a detailed attack scenario for the
described vulnerability. Include attacker motivation, prerequisites, attack steps,
and business impact.

Respond with JSON:
{
  "scenario_title": "<title>",
  "attacker_profile": "<description>",
  "prerequisites": ["<requirement>"],
  "attack_steps": [{"step": <int>, "action": "<action>", "detail": "<detail>"}],
  "business_impact": "<impact description>",
  "likelihood": "HIGH|MEDIUM|LOW",
  "mitre_techniques": ["<TXXXX>"]
}
"""


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------

@dataclass
class GenerativeProbeInput:
    """Input for a generative probe."""
    probe_type: GenerativeProbeType
    finding: dict
    code_context: str = ""
    language: str = ""
    additional_context: dict = field(default_factory=dict)

    def to_prompt(self) -> str:
        parts = [
            f"Finding:\n{json.dumps(self.finding, indent=2)}",
        ]
        if self.code_context:
            lang = self.language or "text"
            parts.append(f"Code context:\n```{lang}\n{self.code_context}\n```")
        if self.additional_context:
            parts.append(f"Additional context:\n{json.dumps(self.additional_context, indent=2)}")
        return "\n\n".join(parts)


@dataclass
class GenerativeProbeOutput:
    """Output from a generative probe."""
    probe_type: GenerativeProbeType
    finding_id: str
    raw_response: str
    parsed_output: dict | list | None
    duration_ms: float = 0.0
    error: str | None = None

    @property
    def succeeded(self) -> bool:
        return self.error is None and self.parsed_output is not None

    def get(self, key: str, default: Any = None) -> Any:
        if isinstance(self.parsed_output, dict):
            return self.parsed_output.get(key, default)
        return default


@dataclass
class PoCSuite:
    """A collection of PoC artifacts for a vulnerability."""
    finding_id: str
    poc_code: str
    language: str
    description: str
    prerequisites: list[str]
    expected_outcome: str
    safety_notes: str
    test_payloads: list[dict] = field(default_factory=list)
    unit_tests: str = ""
    attack_scenario: dict = field(default_factory=dict)
    generated_at: float = field(default_factory=time.time)

    def to_dict(self) -> dict:
        return {
            "finding_id": self.finding_id,
            "poc_code": self.poc_code,
            "language": self.language,
            "description": self.description,
            "prerequisites": self.prerequisites,
            "expected_outcome": self.expected_outcome,
            "safety_notes": self.safety_notes,
            "test_payloads": self.test_payloads,
            "unit_tests": self.unit_tests,
            "attack_scenario": self.attack_scenario,
        }


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _parse_json_response(raw: str) -> dict | list | None:
    """Parse a JSON response, handling markdown wrapping."""
    import re
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        match = re.search(r"```(?:json)?\s*(\{.*?\}|\[.*?\])\s*```", raw, re.DOTALL)
        if match:
            try:
                return json.loads(match.group(1))
            except json.JSONDecodeError:
                pass
    return None


def _select_system_prompt(probe_type: GenerativeProbeType) -> str:
    mapping = {
        GenerativeProbeType.POC_EXPLOIT: POC_SYSTEM_PROMPT,
        GenerativeProbeType.TEST_PAYLOAD: TEST_PAYLOAD_SYSTEM_PROMPT,
        GenerativeProbeType.UNIT_TEST: UNIT_TEST_SYSTEM_PROMPT,
        GenerativeProbeType.ATTACK_SCENARIO: ATTACK_SCENARIO_SYSTEM_PROMPT,
        GenerativeProbeType.FUZZING_INPUT: TEST_PAYLOAD_SYSTEM_PROMPT,
        GenerativeProbeType.SECURITY_ASSERTION: UNIT_TEST_SYSTEM_PROMPT,
    }
    return mapping.get(probe_type, POC_SYSTEM_PROMPT)


# ---------------------------------------------------------------------------
# Generative probe class
# ---------------------------------------------------------------------------

class GenerativeProbe:
    """Generates security artifacts (PoCs, test cases, attack scenarios) using an LLM.

    Used in security research and validation workflows to create material for
    testing vulnerability detection and remediation pipelines.

    Example::

        probe = GenerativeProbe()
        output = probe.run(
            GenerativeProbeInput(
                probe_type=GenerativeProbeType.POC_EXPLOIT,
                finding=finding_dict,
                code_context=vulnerable_code,
            )
        )
        poc_code = output.get("poc_code")
    """

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        api_key: str | None = None,
    ):
        self.model = model
        self._client = anthropic.Anthropic(
            api_key=api_key or os.environ.get("ANTHROPIC_API_KEY", "")
        )

    def run(self, probe_input: GenerativeProbeInput) -> GenerativeProbeOutput:
        """Execute a generative probe.

        Args:
            probe_input: GenerativeProbeInput specifying the probe type and context.

        Returns:
            GenerativeProbeOutput with the generated artifact.
        """
        finding_id = str(probe_input.finding.get("id", ""))
        system_prompt = _select_system_prompt(probe_input.probe_type)
        user_prompt = probe_input.to_prompt()
        start = time.monotonic()

        logger.info(
            "GenerativeProbe.run probe_type=%s finding_id=%s",
            probe_input.probe_type.value, finding_id,
        )

        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=4096,
        )

        raw = response.content[0].text
        elapsed_ms = (time.monotonic() - start) * 1000
        parsed = _parse_json_response(raw)

        return GenerativeProbeOutput(
            probe_type=probe_input.probe_type,
            finding_id=finding_id,
            raw_response=raw,
            parsed_output=parsed,
            duration_ms=elapsed_ms,
        )

    def generate_poc_suite(
        self,
        finding: dict,
        code_context: str = "",
        language: str = "",
    ) -> PoCSuite:
        """Generate a full PoC suite for a finding.

        Generates PoC code, test payloads, unit tests, and an attack scenario.

        Args:
            finding: Finding dictionary.
            code_context: Vulnerable source code.
            language: Programming language.

        Returns:
            PoCSuite with all generated artifacts.
        """
        finding_id = str(finding.get("id", ""))

        poc_output = self.run(GenerativeProbeInput(
            probe_type=GenerativeProbeType.POC_EXPLOIT,
            finding=finding,
            code_context=code_context,
            language=language,
        ))

        payload_output = self.run(GenerativeProbeInput(
            probe_type=GenerativeProbeType.TEST_PAYLOAD,
            finding=finding,
            code_context=code_context,
            language=language,
        ))

        unit_test_output = self.run(GenerativeProbeInput(
            probe_type=GenerativeProbeType.UNIT_TEST,
            finding=finding,
            code_context=code_context,
            language=language,
        ))

        scenario_output = self.run(GenerativeProbeInput(
            probe_type=GenerativeProbeType.ATTACK_SCENARIO,
            finding=finding,
            code_context=code_context,
        ))

        poc_data = poc_output.parsed_output or {}
        if isinstance(poc_data, list):
            poc_data = poc_data[0] if poc_data else {}

        payload_data = payload_output.parsed_output or {}
        payloads = (
            payload_data.get("payloads", [])
            if isinstance(payload_data, dict)
            else []
        )

        unit_test_data = unit_test_output.parsed_output or {}
        unit_tests = (
            unit_test_data.get("test_file", "")
            if isinstance(unit_test_data, dict)
            else ""
        )

        scenario_data = (
            scenario_output.parsed_output
            if isinstance(scenario_output.parsed_output, dict)
            else {}
        )

        return PoCSuite(
            finding_id=finding_id,
            poc_code=poc_data.get("poc_code", ""),
            language=poc_data.get("language", language),
            description=poc_data.get("description", ""),
            prerequisites=poc_data.get("prerequisites", []),
            expected_outcome=poc_data.get("expected_outcome", ""),
            safety_notes=poc_data.get("safety_notes", ""),
            test_payloads=payloads,
            unit_tests=unit_tests,
            attack_scenario=scenario_data,
        )
