"""Tests for api.config.get_allowed_origins (sast_013 fix)."""

import os
import pytest

from api.config import get_allowed_origins


class TestGetAllowedOrigins:
    def test_single_origin(self, monkeypatch):
        monkeypatch.setenv("ALLOWED_ORIGINS", "https://app.example.com")
        assert get_allowed_origins() == ["https://app.example.com"]

    def test_multiple_origins(self, monkeypatch):
        monkeypatch.setenv(
            "ALLOWED_ORIGINS",
            "https://app.example.com,https://admin.example.com",
        )
        assert get_allowed_origins() == [
            "https://app.example.com",
            "https://admin.example.com",
        ]

    def test_strips_whitespace(self, monkeypatch):
        monkeypatch.setenv(
            "ALLOWED_ORIGINS",
            "  https://app.example.com , https://admin.example.com  ",
        )
        result = get_allowed_origins()
        assert result == ["https://app.example.com", "https://admin.example.com"]

    def test_raises_when_unset(self, monkeypatch):
        monkeypatch.delenv("ALLOWED_ORIGINS", raising=False)
        with pytest.raises(RuntimeError, match="ALLOWED_ORIGINS"):
            get_allowed_origins()

    def test_raises_when_empty_string(self, monkeypatch):
        monkeypatch.setenv("ALLOWED_ORIGINS", "")
        with pytest.raises(RuntimeError, match="ALLOWED_ORIGINS"):
            get_allowed_origins()

    def test_raises_when_only_commas(self, monkeypatch):
        monkeypatch.setenv("ALLOWED_ORIGINS", ",,,")
        with pytest.raises(RuntimeError, match="ALLOWED_ORIGINS"):
            get_allowed_origins()

    def test_no_wildcard_accepted(self, monkeypatch):
        """Ensure a wildcard value is returned as-is but not special-cased.

        The env var may technically be set to "*" by a misconfigured deployment;
        get_allowed_origins() does not validate values — the caller (middleware)
        is responsible.  This test documents that the function does NOT silently
        expand "*" into a bypass.
        """
        monkeypatch.setenv("ALLOWED_ORIGINS", "*")
        result = get_allowed_origins()
        # Returns ["*"] — still just one element; no expansion or bypass logic.
        assert result == ["*"]
