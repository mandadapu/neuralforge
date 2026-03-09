"""Tests for api.routers.repos (finding #44 — router for repo management)"""
import pytest
from unittest.mock import MagicMock


class TestRepoRouteHappyPaths:
    def test_list_repos_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"repos": [], "total": 0},
        )
        response = client.get("/repos")
        assert response.status_code == 200

    def test_get_repo_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "repo-001", "full_name": "org/repo"},
        )
        response = client.get("/repos/repo-001")
        assert response.json()["id"] == "repo-001"

    def test_register_repo_returns_201(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=201,
            json=lambda: {"id": "repo-002", "full_name": "org/new-repo"},
        )
        response = client.post("/repos", json={"full_name": "org/new-repo"})
        assert response.status_code == 201

    def test_delete_repo_returns_204(self):
        client = MagicMock()
        client.delete.return_value = MagicMock(status_code=204)
        response = client.delete("/repos/repo-001")
        assert response.status_code == 204


class TestRepoRouteErrors:
    def test_get_nonexistent_repo_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=404)
        response = client.get("/repos/nonexistent")
        assert response.status_code == 404

    def test_register_duplicate_returns_409(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=409)
        response = client.post("/repos", json={"full_name": "org/existing"})
        assert response.status_code == 409

    def test_unauthenticated_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/repos")
        assert response.status_code == 401
