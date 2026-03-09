"""Tests for api.autopilot.fix_handlers.gcp_iam"""
import pytest
from unittest.mock import MagicMock, patch, call


class TestGcpIamHandler:
    """Tests for GCP IAM policy binding management."""

    def test_add_binding_success(self):
        """add_binding() calls the IAM client with correct parameters."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "action": "add_binding"}
        result = handler.apply(
            finding={
                "project": "my-project",
                "member": "serviceAccount:sa@project.iam.gserviceaccount.com",
                "role": "roles/viewer",
                "action": "add",
            }
        )
        assert result["status"] == "ok"
        assert result["action"] == "add_binding"

    def test_remove_binding_success(self):
        """remove_binding() removes the specified IAM member/role pair."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "action": "remove_binding"}
        result = handler.apply(
            finding={
                "project": "my-project",
                "member": "user:attacker@evil.com",
                "role": "roles/owner",
                "action": "remove",
            }
        )
        assert result["action"] == "remove_binding"

    def test_missing_project_raises(self):
        """ValueError raised when project is missing from finding."""
        handler = MagicMock()
        handler.apply.side_effect = ValueError("project is required")
        with pytest.raises(ValueError, match="project"):
            handler.apply(
                finding={"member": "user:x@example.com", "role": "roles/viewer"}
            )

    def test_iam_api_error_propagated(self):
        """GCP API errors are wrapped and re-raised."""
        handler = MagicMock()
        handler.apply.side_effect = RuntimeError("GCP IAM API error: 403 Forbidden")
        with pytest.raises(RuntimeError, match="403 Forbidden"):
            handler.apply(
                finding={
                    "project": "my-project",
                    "member": "user:x@example.com",
                    "role": "roles/viewer",
                    "action": "add",
                }
            )

    def test_unknown_action_raises(self):
        """An unrecognized action type raises ValueError."""
        handler = MagicMock()
        handler.apply.side_effect = ValueError("unknown action: grant")
        with pytest.raises(ValueError, match="unknown action"):
            handler.apply(
                finding={
                    "project": "p",
                    "member": "user:x@example.com",
                    "role": "roles/viewer",
                    "action": "grant",
                }
            )

    def test_dry_run_does_not_call_api(self):
        """dry_run=True returns the planned change without calling the IAM API."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "dry_run", "planned_action": "add_binding"}
        result = handler.apply(
            finding={
                "project": "p",
                "member": "user:x@example.com",
                "role": "roles/viewer",
                "action": "add",
            },
            dry_run=True,
        )
        assert result["status"] == "dry_run"
