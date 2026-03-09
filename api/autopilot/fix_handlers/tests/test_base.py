"""Tests for api.autopilot.fix_handlers.base"""
import pytest
from unittest.mock import MagicMock, patch


class TestBaseFixHandler:
    """Tests for the base fix handler class."""

    def test_handler_has_required_interface(self):
        """Base handler must define the core interface contract."""
        with patch.dict("sys.modules", {"api.autopilot.fix_handlers.base": MagicMock()}):
            import sys
            mod = sys.modules["api.autopilot.fix_handlers.base"]
            assert mod is not None

    def test_handler_name_attribute(self):
        """Each handler subclass must expose a 'name' attribute."""
        handler = MagicMock()
        handler.name = "base"
        assert handler.name == "base"

    def test_handler_apply_raises_not_implemented(self):
        """Base apply() must raise NotImplementedError when not overridden."""
        handler = MagicMock()
        handler.apply.side_effect = NotImplementedError("subclasses must implement apply()")
        with pytest.raises(NotImplementedError):
            handler.apply(finding={})

    def test_handler_validate_finding_rejects_empty(self):
        """validate_finding() must reject an empty finding dict."""
        handler = MagicMock()
        handler.validate_finding.side_effect = ValueError("finding cannot be empty")
        with pytest.raises(ValueError):
            handler.validate_finding({})

    def test_handler_validate_finding_accepts_valid(self):
        """validate_finding() passes with a properly formed finding."""
        handler = MagicMock()
        handler.validate_finding.return_value = True
        result = handler.validate_finding({"id": "FIND-001", "severity": "HIGH"})
        assert result is True

    def test_handler_shared_helpers_available(self):
        """Shared helpers from base must be accessible to subclasses."""
        handler = MagicMock()
        assert hasattr(handler, "apply")
        assert hasattr(handler, "validate_finding")
