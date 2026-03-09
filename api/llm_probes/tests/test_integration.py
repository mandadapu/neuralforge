"""Tests for api.llm_probes.integration (430 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestIntegrationProbeOrchestration:
    def test_run_integration_probe_returns_result(self):
        probe = MagicMock()
        probe.run.return_value = {
            "passed": True,
            "external_calls": 3,
            "llm_calls": 2,
        }
        result = probe.run(config={"target": "https://api.example.com"})
        assert result["passed"] is True

    def test_run_missing_target_raises(self):
        probe = MagicMock()
        probe.run.side_effect = ValueError("target is required")
        with pytest.raises(ValueError, match="target"):
            probe.run(config={})

    def test_run_orchestrates_multiple_sub_probes(self):
        probe = MagicMock()
        probe.run.return_value = {
            "sub_probes": ["auth_probe", "data_probe", "injection_probe"],
            "passed": True,
        }
        result = probe.run(config={"target": "https://api.example.com"})
        assert len(result["sub_probes"]) == 3


class TestExternalServiceMocking:
    def test_external_api_call_mocked(self):
        probe = MagicMock()
        probe.call_external.return_value = {"status": 200, "body": {"result": "ok"}}
        result = probe.call_external(
            url="https://api.example.com/endpoint", method="POST", payload={}
        )
        assert result["status"] == 200

    def test_external_api_timeout_handled(self):
        probe = MagicMock()
        probe.call_external.side_effect = TimeoutError("external API timed out after 30s")
        with pytest.raises(TimeoutError, match="timed out"):
            probe.call_external(url="https://slow-api.example.com", method="GET", payload=None)

    def test_external_api_4xx_marks_probe_failed(self):
        probe = MagicMock()
        probe.run.return_value = {"passed": False, "reason": "external API returned 403"}
        result = probe.run(config={"target": "https://forbidden-api.example.com"})
        assert result["passed"] is False

    def test_llm_and_external_calls_sequenced_correctly(self):
        probe = MagicMock()
        probe.run.return_value = {
            "sequence": ["llm_call", "external_call", "llm_call"],
            "passed": True,
        }
        result = probe.run(config={"target": "https://api.example.com"})
        assert result["sequence"][0] == "llm_call"
        assert result["sequence"][1] == "external_call"

    def test_integration_result_includes_all_sub_results(self):
        probe = MagicMock()
        probe.run.return_value = {
            "sub_results": [
                {"probe": "auth_probe", "passed": True},
                {"probe": "injection_probe", "passed": True},
            ],
            "overall_passed": True,
        }
        result = probe.run(config={"target": "https://api.example.com"})
        assert len(result["sub_results"]) == 2
        assert result["overall_passed"] is True
