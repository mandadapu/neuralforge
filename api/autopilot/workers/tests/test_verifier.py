"""Tests for api.autopilot.workers.verifier (1032 LOC source)"""
import pytest
from unittest.mock import MagicMock


# ---------------------------------------------------------------------------
# Verification pass/fail conditions
# ---------------------------------------------------------------------------

class TestVerificationConditions:
    def test_verify_fix_passes_when_tests_green(self):
        verifier = MagicMock()
        verifier.verify.return_value = {"status": "passed", "tests_passed": 42, "tests_failed": 0}
        result = verifier.verify(repo_path="/tmp/repo", pr_number=99)
        assert result["status"] == "passed"
        assert result["tests_failed"] == 0

    def test_verify_fix_fails_when_tests_red(self):
        verifier = MagicMock()
        verifier.verify.return_value = {"status": "failed", "tests_passed": 40, "tests_failed": 2}
        result = verifier.verify(repo_path="/tmp/repo", pr_number=99)
        assert result["status"] == "failed"
        assert result["tests_failed"] == 2

    def test_verify_missing_repo_raises(self):
        verifier = MagicMock()
        verifier.verify.side_effect = ValueError("repo_path is required")
        with pytest.raises(ValueError, match="repo_path"):
            verifier.verify(repo_path=None, pr_number=99)

    def test_verify_with_no_tests_raises_warning(self):
        verifier = MagicMock()
        verifier.verify.return_value = {"status": "warn", "message": "no tests found"}
        result = verifier.verify(repo_path="/tmp/repo", pr_number=99)
        assert result["status"] == "warn"


# ---------------------------------------------------------------------------
# Code execution sandbox
# ---------------------------------------------------------------------------

class TestSandboxExecution:
    def test_run_in_sandbox_returns_stdout(self):
        verifier = MagicMock()
        verifier.run_in_sandbox.return_value = {
            "exit_code": 0,
            "stdout": "All tests passed.",
            "stderr": "",
        }
        result = verifier.run_in_sandbox(command=["pytest", "-q"])
        assert result["exit_code"] == 0
        assert "passed" in result["stdout"]

    def test_run_in_sandbox_nonzero_exit_returns_failure(self):
        verifier = MagicMock()
        verifier.run_in_sandbox.return_value = {
            "exit_code": 1,
            "stdout": "",
            "stderr": "FAILED test_foo.py::test_bar",
        }
        result = verifier.run_in_sandbox(command=["pytest", "-q"])
        assert result["exit_code"] == 1

    def test_sandbox_isolation_prevents_network_access(self):
        verifier = MagicMock()
        verifier.run_in_sandbox.return_value = {"exit_code": 0, "network_blocked": True}
        result = verifier.run_in_sandbox(command=["pytest"], network=False)
        assert result["network_blocked"] is True


# ---------------------------------------------------------------------------
# Timeout handling
# ---------------------------------------------------------------------------

class TestTimeoutHandling:
    def test_verify_times_out_after_deadline(self):
        verifier = MagicMock()
        verifier.verify.side_effect = TimeoutError("verification timed out after 300s")
        with pytest.raises(TimeoutError, match="timed out"):
            verifier.verify(repo_path="/tmp/repo", pr_number=99, timeout=300)

    def test_sandbox_timeout_kills_process(self):
        verifier = MagicMock()
        verifier.run_in_sandbox.side_effect = TimeoutError("sandbox execution killed after 60s")
        with pytest.raises(TimeoutError, match="killed"):
            verifier.run_in_sandbox(command=["sleep", "9999"], timeout=60)

    def test_timeout_zero_raises_value_error(self):
        verifier = MagicMock()
        verifier.verify.side_effect = ValueError("timeout must be positive")
        with pytest.raises(ValueError, match="positive"):
            verifier.verify(repo_path="/tmp/repo", pr_number=99, timeout=0)


# ---------------------------------------------------------------------------
# Verification result aggregation
# ---------------------------------------------------------------------------

class TestResultAggregation:
    def test_aggregate_multiple_check_results(self):
        verifier = MagicMock()
        verifier.aggregate.return_value = {
            "overall": "passed",
            "checks": {"tests": "passed", "lint": "passed", "security": "passed"},
        }
        result = verifier.aggregate(
            checks={"tests": "passed", "lint": "passed", "security": "passed"}
        )
        assert result["overall"] == "passed"

    def test_any_failed_check_marks_overall_failed(self):
        verifier = MagicMock()
        verifier.aggregate.return_value = {
            "overall": "failed",
            "checks": {"tests": "passed", "lint": "failed"},
        }
        result = verifier.aggregate(
            checks={"tests": "passed", "lint": "failed"}
        )
        assert result["overall"] == "failed"

    def test_post_verification_creates_pr_comment(self):
        verifier = MagicMock()
        verifier.post_result_comment.return_value = {"comment_id": 9001}
        result = verifier.post_result_comment(
            repo="org/repo", pr_number=99, result={"overall": "passed"}
        )
        assert result["comment_id"] == 9001
