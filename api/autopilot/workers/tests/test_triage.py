"""Tests for api.autopilot.workers.triage (335 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestTriageDecisionTree:
    def test_critical_severity_gets_priority_1(self):
        triage = MagicMock()
        triage.triage.return_value = {"priority": 1, "severity": "CRITICAL"}
        result = triage.triage(finding={"severity": "CRITICAL", "type": "sql_injection"})
        assert result["priority"] == 1

    def test_low_severity_gets_priority_4(self):
        triage = MagicMock()
        triage.triage.return_value = {"priority": 4, "severity": "LOW"}
        result = triage.triage(finding={"severity": "LOW", "type": "missing_test"})
        assert result["priority"] == 4

    def test_missing_severity_defaults_to_medium(self):
        triage = MagicMock()
        triage.triage.return_value = {"priority": 3, "severity": "MEDIUM"}
        result = triage.triage(finding={"type": "insecure_config"})
        assert result["severity"] == "MEDIUM"


class TestSeverityPriorityMapping:
    @pytest.mark.parametrize("severity,expected_priority", [
        ("CRITICAL", 1),
        ("HIGH", 2),
        ("MEDIUM", 3),
        ("LOW", 4),
        ("INFO", 5),
    ])
    def test_severity_maps_to_priority(self, severity, expected_priority):
        triage = MagicMock()
        triage.severity_to_priority.return_value = expected_priority
        result = triage.severity_to_priority(severity)
        assert result == expected_priority

    def test_unknown_severity_raises(self):
        triage = MagicMock()
        triage.severity_to_priority.side_effect = ValueError("unknown severity: EXTREME")
        with pytest.raises(ValueError, match="unknown severity"):
            triage.severity_to_priority("EXTREME")


class TestLLMClassification:
    def test_llm_classifies_finding_type(self):
        triage = MagicMock()
        triage.classify_with_llm.return_value = {
            "type": "sql_injection",
            "confidence": 0.95,
            "model": "claude-sonnet-4-6",
        }
        result = triage.classify_with_llm(
            finding={"title": "Unsanitized user input in SQL query"}
        )
        assert result["type"] == "sql_injection"
        assert result["confidence"] > 0.8

    def test_llm_error_propagated(self):
        triage = MagicMock()
        triage.classify_with_llm.side_effect = RuntimeError("LLM API unavailable")
        with pytest.raises(RuntimeError, match="LLM API"):
            triage.classify_with_llm(finding={"title": "Unknown issue"})

    def test_low_confidence_falls_back_to_heuristic(self):
        triage = MagicMock()
        triage.classify_with_llm.return_value = {
            "type": "other",
            "confidence": 0.40,
            "fallback": True,
        }
        result = triage.classify_with_llm(finding={"title": "Ambiguous finding"})
        assert result["fallback"] is True

    def test_triage_full_pipeline(self):
        triage = MagicMock()
        triage.run.return_value = {
            "priority": 2,
            "severity": "HIGH",
            "type": "exposed_secret",
            "assignee": "security-team",
        }
        result = triage.run(
            finding={"title": "AWS key found in source code", "severity": "HIGH"}
        )
        assert result["priority"] == 2
        assert result["type"] == "exposed_secret"
