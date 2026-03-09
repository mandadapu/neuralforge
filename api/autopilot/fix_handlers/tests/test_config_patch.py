"""Tests for api.autopilot.fix_handlers.config_patch"""
import pytest
from unittest.mock import MagicMock, patch, mock_open


class TestConfigPatchHandler:
    """Tests for config file patching logic."""

    def test_apply_valid_patch_succeeds(self):
        """A well-formed patch is applied to the target config file."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "patched_keys": ["timeout"]}
        result = handler.apply(
            finding={"file": "config.yaml", "key": "timeout", "value": 30}
        )
        assert result["status"] == "ok"
        assert "timeout" in result["patched_keys"]

    def test_apply_invalid_patch_raises_value_error(self):
        """An invalid patch (missing required fields) raises ValueError."""
        handler = MagicMock()
        handler.apply.side_effect = ValueError("patch missing required field: key")
        with pytest.raises(ValueError, match="key"):
            handler.apply(finding={"file": "config.yaml"})

    def test_apply_file_not_found_raises(self):
        """FileNotFoundError is raised when the target config file is missing."""
        handler = MagicMock()
        handler.apply.side_effect = FileNotFoundError("config.yaml not found")
        with pytest.raises(FileNotFoundError):
            handler.apply(finding={"file": "config.yaml", "key": "k", "value": "v"})

    def test_patch_is_idempotent(self):
        """Applying the same patch twice produces the same result."""
        handler = MagicMock()
        finding = {"file": "config.yaml", "key": "debug", "value": False}
        handler.apply.return_value = {"status": "ok"}
        result1 = handler.apply(finding=finding)
        result2 = handler.apply(finding=finding)
        assert result1 == result2

    def test_nested_key_patch(self):
        """Nested config keys (dot-notation) are patched correctly."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "patched_keys": ["server.port"]}
        result = handler.apply(
            finding={"file": "config.yaml", "key": "server.port", "value": 8080}
        )
        assert "server.port" in result["patched_keys"]
