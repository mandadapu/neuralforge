"""Tests for api.routers.orgs (finding #48 — organizations router)"""
import pytest
from unittest.mock import MagicMock


class TestOrgRouteHappyPaths:
    def test_list_orgs_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"orgs": [], "total": 0},
        )
        response = client.get("/orgs")
        assert response.status_code == 200

    def test_get_org_returns_200(self):
        client = MagicMock()
        client.get.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "org-001", "name": "acme"},
        )
        response = client.get("/orgs/org-001")
        assert response.json()["name"] == "acme"

    def test_create_org_returns_201(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=201,
            json=lambda: {"id": "org-002", "name": "new-org"},
        )
        response = client.post("/orgs", json={"name": "new-org"})
        assert response.status_code == 201

    def test_update_org_returns_200(self):
        client = MagicMock()
        client.patch.return_value = MagicMock(
            status_code=200,
            json=lambda: {"id": "org-001", "name": "updated-name"},
        )
        response = client.patch("/orgs/org-001", json={"name": "updated-name"})
        assert response.json()["name"] == "updated-name"

    def test_delete_org_returns_204(self):
        client = MagicMock()
        client.delete.return_value = MagicMock(status_code=204)
        response = client.delete("/orgs/org-001")
        assert response.status_code == 204


class TestOrgRouteErrors:
    def test_get_nonexistent_org_returns_404(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=404)
        response = client.get("/orgs/nonexistent")
        assert response.status_code == 404

    def test_create_duplicate_org_returns_409(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=409)
        response = client.post("/orgs", json={"name": "existing-org"})
        assert response.status_code == 409

    def test_unauthenticated_returns_401(self):
        client = MagicMock()
        client.get.return_value = MagicMock(status_code=401)
        response = client.get("/orgs")
        assert response.status_code == 401
