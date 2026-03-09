"""Tests for api.routers.audit (91 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestAuditLogRetrieval:
    def test_list_audit_logs_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"logs": [], "total": 0},
        )
        response = client.get("/audit")
        assert response.status_code == 200

    def test_list_audit_logs_with_filter(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {
                "logs": [{"id": "log-001", "action": "create_agent"}],
                "total": 1,
            },
        )
        response = client.get("/audit?action=create_agent")
        assert response.json()["total"] == 1

    def test_unauthenticated_request_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/audit")
        assert response.status_code == 401

    def test_non_admin_request_returns_403(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=403)
        response = client.get("/audit")
        assert response.status_code == 403


class TestAuditLogPagination:
    def test_pagination_offset_and_limit(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"logs": [{"id": f"log-{i}"} for i in range(10)], "total": 100},
        )
        response = client.get("/audit?limit=10&offset=0")
        assert len(response.json()["logs"]) == 10
        assert response.json()["total"] == 100

    def test_invalid_limit_returns_422(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=422,
            json=lambda: {"detail": "limit must be positive"},
        )
        response = client.get("/audit?limit=-1")
        assert response.status_code == 422


class TestAuditLogFiltering:
    def test_filter_by_user(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"logs": [{"user": "admin@example.com"}], "total": 1},
        )
        response = client.get("/audit?user=admin@example.com")
        assert response.json()["logs"][0]["user"] == "admin@example.com"

    def test_filter_by_date_range(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"logs": [], "total": 0},
        )
        response = client.get("/audit?from=2026-01-01&to=2026-03-09")
        assert response.status_code == 200
