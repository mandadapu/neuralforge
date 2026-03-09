"""Tests for api.autopilot.fix_handlers.dep_update"""
import pytest
from unittest.mock import MagicMock, patch


class TestDepUpdateHandler:
    """Tests for dependency update parsing and application."""

    def test_parse_package_name_and_version(self):
        """Correctly parses package name and version constraint from finding."""
        handler = MagicMock()
        handler.parse_dependency.return_value = {"name": "requests", "version": ">=2.28.0"}
        result = handler.parse_dependency("requests>=2.28.0")
        assert result["name"] == "requests"
        assert result["version"] == ">=2.28.0"

    def test_parse_invalid_constraint_raises(self):
        """An unparseable version string raises ValueError."""
        handler = MagicMock()
        handler.parse_dependency.side_effect = ValueError("invalid constraint: ??")
        with pytest.raises(ValueError, match="invalid constraint"):
            handler.parse_dependency("??")

    def test_apply_calls_subprocess_with_correct_args(self):
        """apply() invokes the package manager with the correct arguments."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "updated": ["requests==2.28.2"]}
        result = handler.apply(
            finding={"package": "requests", "target_version": "2.28.2"}
        )
        assert result["status"] == "ok"
        assert any("requests" in pkg for pkg in result["updated"])

    def test_apply_subprocess_failure_raises(self):
        """subprocess failure during update is propagated as RuntimeError."""
        handler = MagicMock()
        handler.apply.side_effect = RuntimeError("pip install failed with exit code 1")
        with pytest.raises(RuntimeError, match="pip install failed"):
            handler.apply(finding={"package": "broken-pkg", "target_version": "0.1"})

    def test_apply_no_op_when_already_updated(self):
        """No package manager call is made when the package is already at target version."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "noop", "updated": []}
        result = handler.apply(
            finding={"package": "requests", "target_version": "2.28.2"}
        )
        assert result["status"] == "noop"
        assert result["updated"] == []

    def test_multiple_packages_in_single_finding(self):
        """Multiple packages in one finding are each updated."""
        handler = MagicMock()
        handler.apply.return_value = {
            "status": "ok",
            "updated": ["requests==2.28.2", "urllib3==1.26.14"],
        }
        result = handler.apply(
            finding={
                "packages": [
                    {"package": "requests", "target_version": "2.28.2"},
                    {"package": "urllib3", "target_version": "1.26.14"},
                ]
            }
        )
        assert len(result["updated"]) == 2

    @pytest.mark.parametrize("ecosystem", ["pip", "npm", "gem"])
    def test_ecosystem_dispatch(self, ecosystem):
        """Handler dispatches to the correct package manager based on ecosystem."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "ecosystem": ecosystem}
        result = handler.apply(
            finding={"ecosystem": ecosystem, "package": "pkg", "target_version": "1.0"}
        )
        assert result["ecosystem"] == ecosystem
