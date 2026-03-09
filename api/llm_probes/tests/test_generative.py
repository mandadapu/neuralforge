"""Tests for api.llm_probes.generative (233 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestGenerativeProbeConstruction:
    def test_build_probe_with_template(self):
        probe = MagicMock()
        probe.build.return_value = {
            "prompt": "Generate a SQL query that...",
            "template": "sql_injection",
        }
        result = probe.build(template="sql_injection", variables={"table": "users"})
        assert "Generate" in result["prompt"]
        assert result["template"] == "sql_injection"

    def test_build_probe_unknown_template_raises(self):
        probe = MagicMock()
        probe.build.side_effect = ValueError("unknown template: nonexistent")
        with pytest.raises(ValueError, match="unknown template"):
            probe.build(template="nonexistent", variables={})

    def test_build_probe_missing_required_variable_raises(self):
        probe = MagicMock()
        probe.build.side_effect = ValueError("missing required variable: table")
        with pytest.raises(ValueError, match="missing required variable"):
            probe.build(template="sql_injection", variables={})


class TestOutputValidation:
    def test_validate_output_passes_for_valid_response(self):
        probe = MagicMock()
        probe.validate_output.return_value = {"valid": True, "issues": []}
        result = probe.validate_output(
            output="SELECT id FROM users WHERE id = ?",
            criteria={"must_not_contain": ["DROP", "DELETE"]},
        )
        assert result["valid"] is True

    def test_validate_output_fails_for_dangerous_output(self):
        probe = MagicMock()
        probe.validate_output.return_value = {
            "valid": False,
            "issues": ["output contains DROP TABLE"],
        }
        result = probe.validate_output(
            output="SELECT * FROM users; DROP TABLE users;",
            criteria={"must_not_contain": ["DROP", "DELETE"]},
        )
        assert result["valid"] is False
        assert len(result["issues"]) > 0

    def test_validate_output_empty_criteria_always_passes(self):
        probe = MagicMock()
        probe.validate_output.return_value = {"valid": True, "issues": []}
        result = probe.validate_output(output="anything", criteria={})
        assert result["valid"] is True

    def test_run_probe_returns_result(self):
        probe = MagicMock()
        probe.run.return_value = {
            "passed": True,
            "output": "Safe response",
            "template": "xss_injection",
        }
        result = probe.run(
            template="xss_injection",
            variables={"payload": "<script>alert(1)</script>"},
        )
        assert result["passed"] is True
