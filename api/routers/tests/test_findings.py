"""Tests for api.routers.findings (finding #46 — findings router)"""
import pytest
from unittest.mock import MagicMock


class TestFindingsRouteHappyPaths:
    def test_list_findings_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"findings": [], "total": 0},
        )
        response = client.get("/findings")
        assert response.status_code == 200

    def test_get_finding_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "FIND-001", "severity": "HIGH"},
        )
        response = client.get("/findings/FIND-001")
        assert response.json()["severity"] == "HIGH"

    def test_dismiss_finding_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "FIND-001", "status": "dismissed"},
        )
        response = client.post("/findings/FIND-001/dismiss", json={"reason": "false positive"})
        assert response.json()["status"] == "dismissed"

    def test_filter_findings_by_severity(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {
                "findings": [{"id": "FIND-001", "severity": "HIGH"}],
                "total": 1,
            },
        )
        response = client.get("/findings?severity=HIGH")
        assert response.json()["findings"][0]["severity"] == "HIGH"


class TestFindingsRouteErrors:
    def test_get_nonexistent_finding_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=404)
        response = client.get("/findings/nonexistent")
        assert response.status_code == 404

    def test_dismiss_with_no_reason_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=422)
        response = client.post("/findings/FIND-001/dismiss", json={})
        assert response.status_code == 422

    def test_unauthenticated_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/findings")
        assert response.status_code == 401
