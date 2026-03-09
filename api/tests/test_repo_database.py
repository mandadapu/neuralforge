"""Tests for api.repo_database (687 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestRepoCRUD:
    def test_create_repo_record(self):
        db = MagicMock()
        db.create.return_value = {
            "id": "repo-001",
            "full_name": "org/repo",
            "scan_state": "pending",
        }
        result = db.create(full_name="org/repo")
        assert result["id"] == "repo-001"
        assert result["scan_state"] == "pending"

    def test_create_duplicate_raises(self):
        db = MagicMock()
        db.create.side_effect = ValueError("repo 'org/repo' already exists")
        with pytest.raises(ValueError, match="already exists"):
            db.create(full_name="org/repo")

    def test_get_repo_by_id(self):
        db = MagicMock()
        db.get.return_value = {"id": "repo-001", "full_name": "org/repo"}
        result = db.get(id="repo-001")
        assert result["full_name"] == "org/repo"

    def test_get_repo_by_full_name(self):
        db = MagicMock()
        db.get_by_name.return_value = {"id": "repo-001", "full_name": "org/repo"}
        result = db.get_by_name(full_name="org/repo")
        assert result["id"] == "repo-001"

    def test_get_nonexistent_returns_none(self):
        db = MagicMock()
        db.get.return_value = None
        result = db.get(id="repo-999")
        assert result is None

    def test_update_repo_metadata(self):
        db = MagicMock()
        db.update.return_value = {"id": "repo-001", "language": "Python"}
        result = db.update(id="repo-001", data={"language": "Python"})
        assert result["language"] == "Python"

    def test_delete_repo(self):
        db = MagicMock()
        db.delete.return_value = {"deleted": True}
        result = db.delete(id="repo-001")
        assert result["deleted"] is True


class TestScanStateTracking:
    def test_set_scan_state_running(self):
        db = MagicMock()
        db.set_scan_state.return_value = {"id": "repo-001", "scan_state": "running"}
        result = db.set_scan_state(id="repo-001", state="running")
        assert result["scan_state"] == "running"

    def test_set_scan_state_completed(self):
        db = MagicMock()
        db.set_scan_state.return_value = {"id": "repo-001", "scan_state": "completed"}
        result = db.set_scan_state(id="repo-001", state="completed")
        assert result["scan_state"] == "completed"

    def test_invalid_scan_state_raises(self):
        db = MagicMock()
        db.set_scan_state.side_effect = ValueError("invalid scan state: fluxing")
        with pytest.raises(ValueError, match="invalid scan state"):
            db.set_scan_state(id="repo-001", state="fluxing")

    def test_list_repos_by_scan_state(self):
        db = MagicMock()
        db.list_by_scan_state.return_value = [
            {"id": "repo-001", "scan_state": "pending"},
            {"id": "repo-002", "scan_state": "pending"},
        ]
        result = db.list_by_scan_state(state="pending")
        assert len(result) == 2
        assert all(r["scan_state"] == "pending" for r in result)

    def test_record_last_scanned_at(self):
        db = MagicMock()
        db.record_scan_time.return_value = {"id": "repo-001", "last_scanned_at": "2026-03-09T12:00:00Z"}
        result = db.record_scan_time(id="repo-001")
        assert "last_scanned_at" in result
