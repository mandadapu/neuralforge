"""Unit tests for nw-agent/redact.py"""

import pytest
from redact import redact_dict, redact_str, _REDACTED


class TestRedactDict:
    def test_redacts_password(self):
        result = redact_dict({"password": "s3cr3t", "username": "alice"})
        assert result["password"] == _REDACTED
        assert result["username"] == "alice"

    def test_redacts_token(self):
        result = redact_dict({"token": "abc123", "name": "service"})
        assert result["token"] == _REDACTED
        assert result["name"] == "service"

    def test_redacts_api_key(self):
        result = redact_dict({"api_key": "key-xyz", "endpoint": "https://example.com"})
        assert result["api_key"] == _REDACTED
        assert result["endpoint"] == "https://example.com"

    def test_redacts_secret(self):
        result = redact_dict({"secret": "topsecret"})
        assert result["secret"] == _REDACTED

    def test_redacts_access_key(self):
        result = redact_dict({"access_key": "AKIA...", "region": "us-east-1"})
        assert result["access_key"] == _REDACTED
        assert result["region"] == "us-east-1"

    def test_redacts_authorization(self):
        result = redact_dict({"authorization": "Bearer tok", "content_type": "json"})
        assert result["authorization"] == _REDACTED
        assert result["content_type"] == "json"

    def test_non_sensitive_keys_pass_through(self):
        data = {"user": "bob", "count": 42, "items": ["a", "b"]}
        result = redact_dict(data)
        assert result == data

    def test_non_dict_returns_unchanged(self):
        assert redact_dict("plain string") == "plain string"
        assert redact_dict(42) == 42
        assert redact_dict(None) is None

    def test_recursive_redacts_nested_dict(self):
        data = {"outer": "value", "config": {"password": "secret", "host": "localhost"}}
        result = redact_dict(data, recursive=True)
        assert result["config"]["password"] == _REDACTED
        assert result["config"]["host"] == "localhost"
        assert result["outer"] == "value"

    def test_recursive_false_skips_nested(self):
        data = {"config": {"password": "secret"}}
        result = redact_dict(data, recursive=False)
        assert result["config"]["password"] == "secret"

    def test_empty_dict(self):
        assert redact_dict({}) == {}

    def test_case_insensitive_key_matching(self):
        result = redact_dict({"PASSWORD": "s3cr3t", "Token": "abc"})
        assert result["PASSWORD"] == _REDACTED
        assert result["Token"] == _REDACTED

    def test_does_not_redact_token_count(self):
        """token_count has '_' after 'token' — word boundary prevents match."""
        result = redact_dict({"token_count": 100})
        assert result["token_count"] == 100


class TestRedactStr:
    def test_redacts_password_in_json_string(self):
        s = '{"username": "alice", "password": "s3cr3t"}'
        result = redact_str(s, keys=["password"])
        assert "s3cr3t" not in result
        assert _REDACTED in result

    def test_non_matching_string_unchanged(self):
        s = '{"name": "alice", "age": 30}'
        result = redact_str(s, keys=["password"])
        assert result == s
