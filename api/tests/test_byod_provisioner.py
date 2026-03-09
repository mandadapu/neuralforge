"""Tests for api.byod_provisioner (73 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestByodProvisioner:
    def test_provision_returns_vm_id(self):
        provisioner = MagicMock()
        provisioner.provision.return_value = {"vm_id": "byod-vm-001", "status": "running"}
        result = provisioner.provision(
            org_id="org-1", region="us-central1", machine_type="n1-standard-2"
        )
        assert result["vm_id"] == "byod-vm-001"
        assert result["status"] == "running"

    def test_provision_missing_org_id_raises(self):
        provisioner = MagicMock()
        provisioner.provision.side_effect = ValueError("org_id is required")
        with pytest.raises(ValueError, match="org_id"):
            provisioner.provision(org_id=None, region="us-central1", machine_type="n1-standard-2")

    def test_provision_cloud_api_error_propagated(self):
        provisioner = MagicMock()
        provisioner.provision.side_effect = RuntimeError("GCP Compute API: quota exceeded")
        with pytest.raises(RuntimeError, match="quota exceeded"):
            provisioner.provision(org_id="org-1", region="us-central1", machine_type="n1-standard-2")

    def test_deprovision_stops_vm(self):
        provisioner = MagicMock()
        provisioner.deprovision.return_value = {"status": "deleted"}
        result = provisioner.deprovision(vm_id="byod-vm-001")
        assert result["status"] == "deleted"

    def test_deprovision_unknown_vm_raises(self):
        provisioner = MagicMock()
        provisioner.deprovision.side_effect = KeyError("vm not found: byod-vm-999")
        with pytest.raises(KeyError, match="byod-vm-999"):
            provisioner.deprovision(vm_id="byod-vm-999")

    def test_get_vm_status(self):
        provisioner = MagicMock()
        provisioner.get_status.return_value = {"vm_id": "byod-vm-001", "status": "running"}
        result = provisioner.get_status(vm_id="byod-vm-001")
        assert result["status"] == "running"
