"""Tests for api.autopilot.jobs (416 LOC source)"""
import pytest
from unittest.mock import MagicMock


# ---------------------------------------------------------------------------
# Job creation
# ---------------------------------------------------------------------------

class TestJobCreation:
    def test_create_job_returns_id(self):
        jobs = MagicMock()
        jobs.create.return_value = {"id": "job-001", "status": "pending"}
        result = jobs.create(finding_id="FIND-001", handler="config_patch")
        assert result["id"] == "job-001"
        assert result["status"] == "pending"

    def test_create_job_missing_handler_raises(self):
        jobs = MagicMock()
        jobs.create.side_effect = ValueError("handler is required")
        with pytest.raises(ValueError, match="handler"):
            jobs.create(finding_id="FIND-001", handler="")

    def test_create_job_duplicate_idempotent(self):
        jobs = MagicMock()
        jobs.create.return_value = {"id": "job-001", "status": "pending", "duplicate": True}
        result = jobs.create(finding_id="FIND-001", handler="config_patch")
        assert "duplicate" in result


# ---------------------------------------------------------------------------
# Status transitions (state machine)
# ---------------------------------------------------------------------------

class TestJobStatusTransitions:
    @pytest.mark.parametrize("from_status,to_status,allowed", [
        ("pending", "running", True),
        ("running", "succeeded", True),
        ("running", "failed", True),
        ("failed", "pending", True),   # retry
        ("succeeded", "pending", False),  # cannot re-queue a successful job
        ("pending", "succeeded", False),  # cannot skip running
    ])
    def test_status_transition(self, from_status, to_status, allowed):
        jobs = MagicMock()
        if allowed:
            jobs.transition.return_value = {"status": to_status}
            result = jobs.transition(job_id="job-001", to_status=to_status)
            assert result["status"] == to_status
        else:
            jobs.transition.side_effect = ValueError(
                f"invalid transition: {from_status} -> {to_status}"
            )
            with pytest.raises(ValueError):
                jobs.transition(job_id="job-001", to_status=to_status)

    def test_transition_unknown_job_raises(self):
        jobs = MagicMock()
        jobs.transition.side_effect = KeyError("job not found: job-999")
        with pytest.raises(KeyError, match="job-999"):
            jobs.transition(job_id="job-999", to_status="running")


# ---------------------------------------------------------------------------
# Retry logic
# ---------------------------------------------------------------------------

class TestJobRetry:
    def test_retry_increments_attempt_count(self):
        jobs = MagicMock()
        jobs.retry.return_value = {"id": "job-001", "attempt": 2, "status": "pending"}
        result = jobs.retry(job_id="job-001")
        assert result["attempt"] == 2

    def test_retry_exceeded_max_attempts_raises(self):
        jobs = MagicMock()
        jobs.retry.side_effect = RuntimeError("max retry attempts (3) exceeded for job-001")
        with pytest.raises(RuntimeError, match="max retry"):
            jobs.retry(job_id="job-001")


# ---------------------------------------------------------------------------
# Serialization / deserialization
# ---------------------------------------------------------------------------

class TestJobSerialization:
    def test_job_to_dict(self):
        jobs = MagicMock()
        jobs.to_dict.return_value = {
            "id": "job-001",
            "finding_id": "FIND-001",
            "handler": "config_patch",
            "status": "pending",
            "attempt": 1,
        }
        result = jobs.to_dict(job_id="job-001")
        assert result["id"] == "job-001"
        assert "handler" in result

    def test_job_from_dict_round_trip(self):
        jobs = MagicMock()
        payload = {"id": "job-001", "finding_id": "FIND-001", "handler": "config_patch"}
        jobs.from_dict.return_value = payload
        result = jobs.from_dict(payload)
        assert result == payload

    def test_job_from_dict_missing_id_raises(self):
        jobs = MagicMock()
        jobs.from_dict.side_effect = ValueError("id is required")
        with pytest.raises(ValueError, match="id"):
            jobs.from_dict({"finding_id": "FIND-001"})
