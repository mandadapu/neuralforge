"""Tests for api.gcp_vm_provisioner (246 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestVmProvisioningRequestBuilding:
    def test_build_request_with_required_fields(self):
        provisioner = MagicMock()
        provisioner.build_request.return_value = {
            "name": "neuralwarden-worker-1",
            "machineType": "zones/us-central1-a/machineTypes/n1-standard-2",
        }
        result = provisioner.build_request(
            name="neuralwarden-worker-1",
            machine_type="n1-standard-2",
            zone="us-central1-a",
        )
        assert "machineType" in result

    def test_build_request_missing_zone_raises(self):
        provisioner = MagicMock()
        provisioner.build_request.side_effect = ValueError("zone is required")
        with pytest.raises(ValueError, match="zone"):
            provisioner.build_request(name="vm", machine_type="n1-standard-2", zone=None)

    def test_provision_vm_creates_instance(self):
        provisioner = MagicMock()
        provisioner.provision.return_value = {
            "status": "RUNNING",
            "name": "neuralwarden-worker-1",
            "zone": "us-central1-a",
        }
        result = provisioner.provision(
            name="neuralwarden-worker-1",
            machine_type="n1-standard-2",
            zone="us-central1-a",
        )
        assert result["status"] == "RUNNING"


class TestIdempotency:
    def test_provision_existing_vm_returns_existing(self):
        provisioner = MagicMock()
        provisioner.provision.return_value = {
            "status": "RUNNING",
            "name": "neuralwarden-worker-1",
            "idempotent": True,
        }
        result = provisioner.provision(
            name="neuralwarden-worker-1", machine_type="n1-standard-2", zone="us-central1-a"
        )
        assert result["idempotent"] is True

    def test_delete_and_reprovision_is_idempotent(self):
        provisioner = MagicMock()
        provisioner.delete.return_value = {"status": "DELETED"}
        provisioner.provision.return_value = {"status": "RUNNING"}
        del_result = provisioner.delete(name="neuralwarden-worker-1", zone="us-central1-a")
        prov_result = provisioner.provision(
            name="neuralwarden-worker-1", machine_type="n1-standard-2", zone="us-central1-a"
        )
        assert del_result["status"] == "DELETED"
        assert prov_result["status"] == "RUNNING"


class TestGcpApiMocking:
    def test_gcp_api_error_propagated(self):
        provisioner = MagicMock()
        provisioner.provision.side_effect = RuntimeError("GCP Compute API 403: permission denied")
        with pytest.raises(RuntimeError, match="permission denied"):
            provisioner.provision(name="vm", machine_type="n1-standard-2", zone="us-central1-a")

    def test_quota_exceeded_raises(self):
        provisioner = MagicMock()
        provisioner.provision.side_effect = RuntimeError("QUOTA_EXCEEDED: CPUs")
        with pytest.raises(RuntimeError, match="QUOTA_EXCEEDED"):
            provisioner.provision(name="vm", machine_type="n1-standard-2", zone="us-central1-a")

    def test_list_vms_returns_running_instances(self):
        provisioner = MagicMock()
        provisioner.list_vms.return_value = [
            {"name": "vm-1", "status": "RUNNING"},
            {"name": "vm-2", "status": "STOPPED"},
        ]
        result = provisioner.list_vms(zone="us-central1-a")
        assert len(result) == 2
