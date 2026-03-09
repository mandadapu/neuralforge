"""Tests for api.autopilot.fix_handlers.secret_rotation"""
import pytest
from unittest.mock import MagicMock, patch


class TestSecretRotationHandler:
    """Tests for secret rotation trigger, revocation, and propagation."""

    def test_trigger_rotation_returns_new_secret_id(self):
        handler = MagicMock()
        handler.rotate.return_value = {"status": "ok", "new_secret_id": "my-secret/versions/5"}
        result = handler.rotate(
            finding={"secret_name": "my-secret", "project": "my-project"}
        )
        assert result["status"] == "ok"
        assert "my-secret/versions/5" == result["new_secret_id"]

    def test_rotate_revokes_old_version(self):
        handler = MagicMock()
        handler.revoke_old_version.return_value = {"status": "revoked", "version": 4}
        result = handler.revoke_old_version(secret_name="my-secret", version=4)
        assert result["status"] == "revoked"

    def test_new_secret_propagated_to_consumers(self):
        handler = MagicMock()
        handler.propagate.return_value = {"status": "ok", "updated_consumers": 2}
        result = handler.propagate(
            secret_name="my-secret",
            new_value="s3cr3t-v5",
            consumers=["service-a", "service-b"],
        )
        assert result["updated_consumers"] == 2

    def test_rotate_missing_secret_raises(self):
        handler = MagicMock()
        handler.rotate.side_effect = ValueError("secret_name is required")
        with pytest.raises(ValueError, match="secret_name"):
            handler.rotate(finding={"project": "my-project"})

    def test_rotate_api_error_propagated(self):
        handler = MagicMock()
        handler.rotate.side_effect = RuntimeError("GCP Secret Manager 403: permission denied")
        with pytest.raises(RuntimeError, match="permission denied"):
            handler.rotate(finding={"secret_name": "s", "project": "p"})

    def test_dry_run_does_not_rotate(self):
        handler = MagicMock()
        handler.rotate.return_value = {"status": "dry_run", "planned": "create version 5"}
        result = handler.rotate(
            finding={"secret_name": "my-secret", "project": "p"}, dry_run=True
        )
        assert result["status"] == "dry_run"
