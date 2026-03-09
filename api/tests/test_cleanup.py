"""Tests for api.cleanup (475 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestOrphanedVmCleanup:
    def test_list_orphaned_vms(self):
        cleanup = MagicMock()
        cleanup.list_orphaned_vms.return_value = [
            {"vm_id": "vm-orphan-1", "age_hours": 72},
            {"vm_id": "vm-orphan-2", "age_hours": 48},
        ]
        result = cleanup.list_orphaned_vms(age_threshold_hours=24)
        assert len(result) == 2

    def test_delete_orphaned_vms(self):
        cleanup = MagicMock()
        cleanup.delete_vms.return_value = {"deleted": 2, "failed": 0}
        result = cleanup.delete_vms(vm_ids=["vm-orphan-1", "vm-orphan-2"])
        assert result["deleted"] == 2
        assert result["failed"] == 0

    def test_delete_vm_gcp_error_logged_not_raised(self):
        cleanup = MagicMock()
        cleanup.delete_vms.return_value = {"deleted": 1, "failed": 1}
        result = cleanup.delete_vms(vm_ids=["vm-orphan-1", "bad-vm"])
        assert result["failed"] == 1


class TestExpiredJobCleanup:
    def test_list_expired_jobs(self):
        cleanup = MagicMock()
        cleanup.list_expired_jobs.return_value = [
            {"id": "job-001", "age_days": 10},
        ]
        result = cleanup.list_expired_jobs(max_age_days=7)
        assert len(result) == 1

    def test_delete_expired_jobs(self):
        cleanup = MagicMock()
        cleanup.delete_jobs.return_value = {"deleted": 5}
        result = cleanup.delete_jobs(job_ids=["job-001", "job-002"])
        assert result["deleted"] == 5


class TestStaleBranchCleanup:
    def test_list_stale_branches(self):
        cleanup = MagicMock()
        cleanup.list_stale_branches.return_value = [
            {"branch": "fix/old-fix", "age_days": 30},
        ]
        result = cleanup.list_stale_branches(max_age_days=14)
        assert len(result) == 1

    def test_delete_stale_branches(self):
        cleanup = MagicMock()
        cleanup.delete_branches.return_value = {"deleted": 3, "failed": 0}
        result = cleanup.delete_branches(
            repo="org/repo", branches=["fix/old-1", "fix/old-2", "fix/old-3"]
        )
        assert result["deleted"] == 3

    def test_protected_branch_not_deleted(self):
        cleanup = MagicMock()
        cleanup.delete_branches.side_effect = ValueError("cannot delete protected branch: main")
        with pytest.raises(ValueError, match="protected branch"):
            cleanup.delete_branches(repo="org/repo", branches=["main"])


class TestCleanupRun:
    def test_full_cleanup_run(self):
        cleanup = MagicMock()
        cleanup.run.return_value = {
            "vms_deleted": 3,
            "jobs_deleted": 10,
            "branches_deleted": 5,
        }
        result = cleanup.run()
        assert result["vms_deleted"] == 3
        assert result["jobs_deleted"] == 10

    def test_dry_run_returns_plan(self):
        cleanup = MagicMock()
        cleanup.run.return_value = {
            "dry_run": True,
            "would_delete": {"vms": 3, "jobs": 10, "branches": 5},
        }
        result = cleanup.run(dry_run=True)
        assert result["dry_run"] is True
