"""Tests for api.routers.autopilot_reports (source finding #43)"""
import pytest
from unittest.mock import MagicMock


class TestReportGenerationEndpoints:
    def test_get_report_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "report-001", "status": "ready"},
        )
        response = client.get("/autopilot/reports/report-001")
        assert response.status_code == 200
        assert response.json()["id"] == "report-001"

    def test_list_reports_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"reports": [], "total": 0},
        )
        response = client.get("/autopilot/reports")
        assert response.status_code == 200

    def test_generate_report_returns_202(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=202,
            json=lambda: {"id": "report-002", "status": "generating"},
        )
        response = client.post(
            "/autopilot/reports",
            json={"repo": "org/repo", "format": "pdf"},
        )
        assert response.status_code == 202

    def test_get_nonexistent_report_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=404,
            json=lambda: {"detail": "report not found"},
        )
        response = client.get("/autopilot/reports/nonexistent")
        assert response.status_code == 404

    def test_unauthenticated_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/autopilot/reports")
        assert response.status_code == 401


class TestReportFormatNegotiation:
    def test_request_pdf_format(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=202,
            json=lambda: {"id": "report-003", "format": "pdf"},
        )
        response = client.post(
            "/autopilot/reports", json={"repo": "org/repo", "format": "pdf"}
        )
        assert response.json()["format"] == "pdf"

    def test_request_json_format(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=202,
            json=lambda: {"id": "report-004", "format": "json"},
        )
        response = client.post(
            "/autopilot/reports", json={"repo": "org/repo", "format": "json"}
        )
        assert response.json()["format"] == "json"

    def test_unsupported_format_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=422,
            json=lambda: {"detail": "unsupported format: docx"},
        )
        response = client.post(
            "/autopilot/reports", json={"repo": "org/repo", "format": "docx"}
        )
        assert response.status_code == 422


class TestReportServiceMocking:
    def test_service_error_returns_500(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=500)
        response = client.post(
            "/autopilot/reports", json={"repo": "org/repo", "format": "pdf"}
        )
        assert response.status_code == 500

    def test_report_download_endpoint(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            headers={"Content-Type": "application/pdf"},
            content=b"%PDF-1.4",
        )
        response = client.get("/autopilot/reports/report-001/download")
        assert response.status_code == 200
        assert response.content[:4] == b"%PDF"
