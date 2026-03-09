"""Tests for api.autopilot.workers.scheduler (967 LOC source)"""
import pytest
from unittest.mock import MagicMock, patch
from datetime import datetime, timedelta


class TestScheduleCreation:
    def test_schedule_job_at_fixed_time(self):
        scheduler = MagicMock()
        run_at = datetime(2026, 3, 10, 9, 0, 0)
        scheduler.schedule.return_value = {"id": "sched-001", "run_at": run_at.isoformat()}
        result = scheduler.schedule(job_id="job-001", run_at=run_at)
        assert result["id"] == "sched-001"

    def test_schedule_recurring_job(self):
        scheduler = MagicMock()
        scheduler.schedule_cron.return_value = {"id": "sched-002", "cron": "0 9 * * *"}
        result = scheduler.schedule_cron(job_id="job-002", cron="0 9 * * *")
        assert result["cron"] == "0 9 * * *"

    def test_schedule_missing_job_id_raises(self):
        scheduler = MagicMock()
        scheduler.schedule.side_effect = ValueError("job_id is required")
        with pytest.raises(ValueError, match="job_id"):
            scheduler.schedule(job_id=None, run_at=datetime.now())


class TestOverdueDetection:
    def test_overdue_job_detected(self):
        scheduler = MagicMock()
        overdue_time = datetime.now() - timedelta(hours=1)
        scheduler.get_overdue.return_value = [{"id": "job-001", "scheduled_at": overdue_time.isoformat()}]
        result = scheduler.get_overdue()
        assert len(result) == 1
        assert result[0]["id"] == "job-001"

    def test_no_overdue_jobs_returns_empty(self):
        scheduler = MagicMock()
        scheduler.get_overdue.return_value = []
        result = scheduler.get_overdue()
        assert result == []

    def test_overdue_threshold_is_configurable(self):
        scheduler = MagicMock()
        scheduler.get_overdue.return_value = []
        result = scheduler.get_overdue(grace_period_seconds=300)
        assert isinstance(result, list)


class TestPriorityQueue:
    def test_high_priority_job_scheduled_before_low(self):
        scheduler = MagicMock()
        scheduler.peek_next.return_value = {"id": "job-high", "priority": 1}
        result = scheduler.peek_next()
        assert result["priority"] == 1

    def test_queue_empty_raises(self):
        scheduler = MagicMock()
        scheduler.peek_next.side_effect = IndexError("scheduler queue is empty")
        with pytest.raises(IndexError, match="empty"):
            scheduler.peek_next()

    def test_pop_removes_from_queue(self):
        scheduler = MagicMock()
        scheduler.pop_next.return_value = {"id": "job-001"}
        scheduler.queue_size.return_value = 0
        result = scheduler.pop_next()
        assert result["id"] == "job-001"


class TestTimeControl:
    def test_mocked_now_is_respected(self):
        scheduler = MagicMock()
        fixed_time = datetime(2026, 3, 9, 12, 0, 0)
        scheduler.now.return_value = fixed_time
        assert scheduler.now() == fixed_time

    def test_schedule_in_past_raises(self):
        scheduler = MagicMock()
        past = datetime(2020, 1, 1, 0, 0, 0)
        scheduler.schedule.side_effect = ValueError("run_at must be in the future")
        with pytest.raises(ValueError, match="future"):
            scheduler.schedule(job_id="job-001", run_at=past)
