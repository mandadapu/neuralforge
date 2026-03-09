"""Tests for api.autopilot.fix_handlers.vm_dispatch"""
import pytest
from unittest.mock import MagicMock, patch


class TestVmDispatchHandler:
    """Tests for VM dispatch routing to the correct zone/instance type."""

    def test_dispatch_routes_to_correct_zone(self):
        handler = MagicMock()
        handler.dispatch.return_value = {"status": "dispatched", "zone": "us-central1-a"}
        result = handler.dispatch(
            finding={"zone": "us-central1-a", "instance_type": "n1-standard-2"}
        )
        assert result["zone"] == "us-central1-a"

    def test_dispatch_missing_zone_raises(self):
        handler = MagicMock()
        handler.dispatch.side_effect = ValueError("zone is required")
        with pytest.raises(ValueError, match="zone"):
            handler.dispatch(finding={"instance_type": "n1-standard-2"})

    def test_dispatch_invalid_instance_type_raises(self):
        handler = MagicMock()
        handler.dispatch.side_effect = ValueError("unknown instance type: n99-mega-2000")
        with pytest.raises(ValueError, match="unknown instance type"):
            handler.dispatch(finding={"zone": "us-central1-a", "instance_type": "n99-mega-2000"})

    def test_dispatch_gcp_api_error_propagated(self):
        handler = MagicMock()
        handler.dispatch.side_effect = RuntimeError("GCP Compute API error: quota exceeded")
        with pytest.raises(RuntimeError, match="quota exceeded"):
            handler.dispatch(finding={"zone": "us-central1-a", "instance_type": "n1-standard-2"})

    def test_dispatch_dry_run(self):
        handler = MagicMock()
        handler.dispatch.return_value = {"status": "dry_run", "target_zone": "us-central1-a"}
        result = handler.dispatch(
            finding={"zone": "us-central1-a", "instance_type": "n1-standard-2"},
            dry_run=True,
        )
        assert result["status"] == "dry_run"

    @pytest.mark.parametrize("zone", ["us-central1-a", "us-east1-b", "europe-west1-c"])
    def test_dispatch_multiple_zones(self, zone):
        handler = MagicMock()
        handler.dispatch.return_value = {"status": "dispatched", "zone": zone}
        result = handler.dispatch(finding={"zone": zone, "instance_type": "n1-standard-2"})
        assert result["zone"] == zone

    def test_dispatch_returns_instance_id(self):
        handler = MagicMock()
        handler.dispatch.return_value = {
            "status": "dispatched",
            "zone": "us-central1-a",
            "instance_id": "vm-12345",
        }
        result = handler.dispatch(
            finding={"zone": "us-central1-a", "instance_type": "n1-standard-2"}
        )
        assert "instance_id" in result
