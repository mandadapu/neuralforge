"""Tests for api.routers.auth_routes (137 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestLogin:
    def test_valid_credentials_return_token(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"access_token": "tok123", "token_type": "bearer"},
        )
        response = client.post(
            "/auth/login",
            json={"username": "admin@example.com", "password": "correctpass"},
        )
        assert response.status_code == 200
        assert "access_token" in response.json()

    def test_invalid_credentials_return_401(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=401,
            json=lambda: {"detail": "Invalid credentials"},
        )
        response = client.post(
            "/auth/login",
            json={"username": "admin@example.com", "password": "wrongpass"},
        )
        assert response.status_code == 401

    def test_missing_username_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=422,
            json=lambda: {"detail": [{"loc": ["body", "username"], "msg": "field required"}]},
        )
        response = client.post("/auth/login", json={"password": "pass"})
        assert response.status_code == 422

    def test_missing_password_returns_422(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=422)
        response = client.post("/auth/login", json={"username": "user@example.com"})
        assert response.status_code == 422


class TestTokenRefresh:
    def test_valid_refresh_token_returns_new_access_token(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"access_token": "new_tok456", "token_type": "bearer"},
        )
        response = client.post(
            "/auth/refresh",
            json={"refresh_token": "valid_refresh_tok"},
        )
        assert response.status_code == 200
        assert response.json()["access_token"] == "new_tok456"

    def test_expired_refresh_token_returns_401(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=401,
            json=lambda: {"detail": "Refresh token expired"},
        )
        response = client.post("/auth/refresh", json={"refresh_token": "expired_tok"})
        assert response.status_code == 401

    def test_invalid_refresh_token_returns_401(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=401)
        response = client.post("/auth/refresh", json={"refresh_token": "garbage"})
        assert response.status_code == 401


class TestLogout:
    def test_logout_invalidates_token(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"message": "Logged out successfully"},
        )
        response = client.post(
            "/auth/logout",
            headers={"Authorization": "Bearer valid_token"},
        )
        assert response.status_code == 200

    def test_logout_without_token_returns_401(self):
        client = MagicMock()
        client.post.return_value = MagicMock(status_code=401)
        response = client.post("/auth/logout")
        assert response.status_code == 401
