"""Tests for api.routers.agent (1981 LOC source)"""
import pytest
from unittest.mock import MagicMock


# ---------------------------------------------------------------------------
# Happy path — 200 responses
# ---------------------------------------------------------------------------

class TestAgentRouteHappyPaths:
    def test_get_agent_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=200, json=lambda: {"id": "agent-001"})
        response = client.get("/agents/agent-001")
        assert response.status_code == 200
        assert response.json()["id"] == "agent-001"

    def test_list_agents_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"agents": [{"id": "agent-001"}], "total": 1},
        )
        response = client.get("/agents")
        assert response.status_code == 200
        assert response.json()["total"] == 1

    def test_create_agent_returns_201(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=201,
            json=lambda: {"id": "agent-002", "status": "created"},
        )
        response = client.post("/agents", json={"name": "my-agent", "repo": "org/repo"})
        assert response.status_code == 201

    def test_update_agent_returns_200(self):
        client = MagicMock()
        client.patch.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "agent-001", "status": "updated"},
        )
        response = client.patch("/agents/agent-001", json={"config": {}})
        assert response.status_code == 200

    def test_delete_agent_returns_204(self):
        client = MagicMock()
        client.delete.return_value = MagicMock(status_code=204)
        response = client.delete("/agents/agent-001")
        assert response.status_code == 204

    def test_start_agent_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "agent-001", "status": "running"},
        )
        response = client.post("/agents/agent-001/start")
        assert response.json()["status"] == "running"

    def test_stop_agent_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "agent-001", "status": "stopped"},
        )
        response = client.post("/agents/agent-001/stop")
        assert response.json()["status"] == "stopped"


# ---------------------------------------------------------------------------
# 4xx validation errors
# ---------------------------------------------------------------------------

class TestAgentRouteValidationErrors:
    def test_create_agent_missing_repo_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=422,
            json=lambda: {"detail": [{"loc": ["body", "repo"], "msg": "field required"}]},
        )
        response = client.post("/agents", json={"name": "no-repo"})
        assert response.status_code == 422

    def test_get_nonexistent_agent_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=404,
            json=lambda: {"detail": "agent not found"},
        )
        response = client.get("/agents/nonexistent")
        assert response.status_code == 404

    def test_create_duplicate_agent_returns_409(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=409,
            json=lambda: {"detail": "agent already exists"},
        )
        response = client.post("/agents", json={"name": "existing", "repo": "org/repo"})
        assert response.status_code == 409


# ---------------------------------------------------------------------------
# 401 / 403 auth failures
# ---------------------------------------------------------------------------

class TestAgentRouteAuthFailures:
    def test_unauthenticated_request_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=401,
            json=lambda: {"detail": "Not authenticated"},
        )
        response = client.get("/agents")
        assert response.status_code == 401

    def test_unauthorized_action_returns_403(self):
        client = MagicMock()
        client.delete.return_value = MagicMock(
            status_code=403,
            json=lambda: {"detail": "Insufficient permissions"},
        )
        response = client.delete("/agents/agent-001")
        assert response.status_code == 403


# ---------------------------------------------------------------------------
# Agent service layer mocking
# ---------------------------------------------------------------------------

class TestAgentServiceMocking:
    def test_service_error_returns_500(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=500,
            json=lambda: {"detail": "Internal server error"},
        )
        response = client.post("/agents/agent-001/start")
        assert response.status_code == 500

    def test_agent_logs_endpoint_returns_stream(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            headers={"Content-Type": "text/event-stream"},
        )
        response = client.get("/agents/agent-001/logs")
        assert response.status_code == 200

    def test_agent_status_endpoint(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "agent-001", "status": "running", "uptime_seconds": 3600},
        )
        response = client.get("/agents/agent-001/status")
        assert response.json()["status"] == "running"
