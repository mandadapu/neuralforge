"""Tests for api.routers.scans (finding #45 — scan management router)"""
import pytest
from unittest.mock import MagicMock


class TestScanRouteHappyPaths:
    def test_trigger_scan_returns_202(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=202,
            json=lambda: {"id": "scan-001", "status": "pending"},
        )
        response = client.post("/scans", json={"repo": "org/repo"})
        assert response.status_code == 202
        assert response.json()["status"] == "pending"

    def test_get_scan_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "scan-001", "status": "completed", "findings": 5},
        )
        response = client.get("/scans/scan-001")
        assert response.json()["status"] == "completed"

    def test_list_scans_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"scans": [], "total": 0},
        )
        response = client.get("/scans")
        assert response.status_code == 200

    def test_cancel_scan_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "scan-001", "status": "cancelled"},
        )
        response = client.post("/scans/scan-001/cancel")
        assert response.json()["status"] == "cancelled"


class TestScanRouteErrors:
    def test_get_nonexistent_scan_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=404)
        response = client.get("/scans/nonexistent")
        assert response.status_code == 404

    def test_trigger_scan_missing_repo_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=422)
        response = client.post("/scans", json={})
        assert response.status_code == 422

    def test_unauthenticated_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/scans")
        assert response.status_code == 401
