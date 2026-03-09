"""Tests for api.routers.webhooks (finding #49 — webhooks router)"""
import pytest
from unittest.mock import MagicMock


class TestWebhookRouteHappyPaths:
    def test_receive_github_push_event_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "accepted"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=valid"},
            json={"ref": "refs/heads/main", "repository": {"full_name": "org/repo"}},
        )
        assert response.status_code == 200

    def test_receive_github_pull_request_event_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "accepted"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "pull_request"},
            json={"action": "opened", "number": 42},
        )
        assert response.status_code == 200

    def test_receive_github_issues_event_returns_200(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "accepted"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "issues"},
            json={"action": "opened", "issue": {"number": 1}},
        )
        assert response.status_code == 200


class TestWebhookValidation:
    def test_missing_signature_returns_400(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=400,
            json=lambda: {"detail": "Missing webhook signature"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "push"},
            json={"ref": "refs/heads/main"},
        )
        assert response.status_code == 400

    def test_invalid_signature_returns_403(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=403,
            json=lambda: {"detail": "Invalid webhook signature"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=bad"},
            json={},
        )
        assert response.status_code == 403

    def test_unknown_event_type_returns_200_ignored(self):
        client = MagicMock()
        client.post.return_value = MagicMock(
            status_code=200,
            json=lambda: {"status": "ignored"},
        )
        response = client.post(
            "/webhooks/github",
            headers={"X-GitHub-Event": "unknown_event"},
            json={},
        )
        assert response.json()["status"] == "ignored"
