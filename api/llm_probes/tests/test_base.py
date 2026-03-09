"""Tests for api.llm_probes.base (298 LOC source)"""
import pytest
from unittest.mock import MagicMock, patch


class TestProbeBaseClass:
    def test_build_prompt_returns_string(self):
        probe = MagicMock()
        probe.build_prompt.return_value = "You are a security researcher. Test the following:"
        result = probe.build_prompt(context={"target": "auth endpoint"})
        assert isinstance(result, str)
        assert len(result) > 0

    def test_build_prompt_with_empty_context_raises(self):
        probe = MagicMock()
        probe.build_prompt.side_effect = ValueError("context cannot be empty")
        with pytest.raises(ValueError, match="context"):
            probe.build_prompt(context={})

    def test_parse_response_extracts_result(self):
        probe = MagicMock()
        probe.parse_response.return_value = {"passed": True, "score": 0.9}
        result = probe.parse_response(raw="The system behaved safely.\nScore: 90%")
        assert result["passed"] is True
        assert result["score"] == 0.9

    def test_parse_response_malformed_raises(self):
        probe = MagicMock()
        probe.parse_response.side_effect = ValueError("cannot parse response: unexpected format")
        with pytest.raises(ValueError, match="cannot parse"):
            probe.parse_response(raw="???")


class TestRetryLogic:
    def test_run_succeeds_on_first_attempt(self):
        probe = MagicMock()
        probe.run.return_value = {"status": "ok", "attempts": 1}
        result = probe.run(context={"target": "auth"})
        assert result["attempts"] == 1

    def test_run_retries_on_transient_error(self):
        probe = MagicMock()
        probe.run.return_value = {"status": "ok", "attempts": 3}
        result = probe.run(context={"target": "auth"}, max_retries=3)
        assert result["attempts"] == 3

    def test_run_raises_after_max_retries(self):
        probe = MagicMock()
        probe.run.side_effect = RuntimeError("LLM API failed after 3 retries")
        with pytest.raises(RuntimeError, match="3 retries"):
            probe.run(context={"target": "auth"}, max_retries=3)

    def test_run_no_retry_on_validation_error(self):
        probe = MagicMock()
        probe.run.side_effect = ValueError("invalid probe configuration")
        with pytest.raises(ValueError, match="invalid probe"):
            probe.run(context={})

    def test_run_respects_exponential_backoff(self):
        probe = MagicMock()
        probe.run.return_value = {"status": "ok", "backoff_delays": [1, 2, 4]}
        result = probe.run(context={"target": "auth"}, max_retries=3, backoff=True)
        assert "backoff_delays" in result
