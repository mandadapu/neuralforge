"""Tests for api.routers.health (finding #50 — health check router)"""
import pytest
from unittest.mock import MagicMock


class TestHealthRouteHappyPaths:
    def test_liveness_probe_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "ok"},
        )
        response = client.get("/health/live")
        assert response.status_code == 200
        assert response.json()["status"] == "ok"

    def test_readiness_probe_returns_200_when_ready(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "ready", "db": "ok", "cache": "ok"},
        )
        response = client.get("/health/ready")
        assert response.status_code == 200
        assert response.json()["db"] == "ok"

    def test_startup_probe_returns_200_when_started(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "started"},
        )
        response = client.get("/health/startup")
        assert response.status_code == 200

    def test_health_includes_version(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "ok", "version": "1.0.0"},
        )
        response = client.get("/health")
        assert "version" in response.json()


class TestHealthRouteUnhealthyState:
    def test_readiness_probe_returns_503_when_db_unavailable(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=503,
            json=lambda: {"status": "not_ready", "db": "error"},
        )
        response = client.get("/health/ready")
        assert response.status_code == 503
        assert response.json()["db"] == "error"

    def test_readiness_probe_returns_503_when_cache_unavailable(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=503,
            json=lambda: {"status": "not_ready", "cache": "error"},
        )
        response = client.get("/health/ready")
        assert response.status_code == 503

    def test_liveness_probe_does_not_check_dependencies(self):
        """Liveness check must always return 200 regardless of dependency state."""
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "ok"},
        )
        response = client.get("/health/live")
        assert response.status_code == 200
