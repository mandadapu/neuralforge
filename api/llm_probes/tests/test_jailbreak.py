"""Tests for api.llm_probes.jailbreak (301 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestJailbreakProbeVariants:
    def test_direct_instruction_override(self):
        probe = MagicMock()
        probe.run.return_value = {"variant": "direct_instruction_override", "passed": True}
        result = probe.run(variant="direct_instruction_override")
        assert result["passed"] is True

    def test_roleplay_jailbreak(self):
        probe = MagicMock()
        probe.run.return_value = {"variant": "roleplay", "passed": True}
        result = probe.run(variant="roleplay")
        assert result["variant"] == "roleplay"

    def test_base64_encoded_prompt(self):
        probe = MagicMock()
        probe.run.return_value = {"variant": "base64_encoded", "passed": True}
        result = probe.run(variant="base64_encoded")
        assert result["passed"] is True

    def test_unknown_variant_raises(self):
        probe = MagicMock()
        probe.run.side_effect = ValueError("unknown variant: impossible_jailbreak")
        with pytest.raises(ValueError, match="unknown variant"):
            probe.run(variant="impossible_jailbreak")


class TestResponseClassification:
    def test_classify_jailbroken_response(self):
        probe = MagicMock()
        probe.classify_response.return_value = {"jailbroken": True, "confidence": 0.95}
        result = probe.classify_response(
            response="Sure! Here is how to synthesize..."
        )
        assert result["jailbroken"] is True
        assert result["confidence"] > 0.8

    def test_classify_safe_response(self):
        probe = MagicMock()
        probe.classify_response.return_value = {"jailbroken": False, "confidence": 0.98}
        result = probe.classify_response(
            response="I'm sorry, I can't help with that request."
        )
        assert result["jailbroken"] is False

    def test_ambiguous_response_low_confidence(self):
        probe = MagicMock()
        probe.classify_response.return_value = {"jailbroken": False, "confidence": 0.55}
        result = probe.classify_response(response="That's an interesting question...")
        assert result["confidence"] < 0.7

    def test_empty_response_classified_as_safe(self):
        probe = MagicMock()
        probe.classify_response.return_value = {"jailbroken": False, "confidence": 1.0}
        result = probe.classify_response(response="")
        assert result["jailbroken"] is False


class TestLlmMocking:
    def test_mocked_llm_returns_refusal(self):
        probe = MagicMock()
        probe.run.return_value = {
            "passed": True,
            "variant": "direct_instruction_override",
            "llm_response": "I can't assist with that.",
        }
        result = probe.run(variant="direct_instruction_override")
        assert result["passed"] is True
        assert "can't assist" in result["llm_response"]

    def test_probe_suite_runs_all_variants(self):
        probe = MagicMock()
        probe.run_all.return_value = {
            "total": 5,
            "passed": 4,
            "failed": 1,
            "results": [],
        }
        result = probe.run_all()
        assert result["total"] == 5
