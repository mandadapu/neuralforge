"""Unit tests for api.utils.pii masking helpers."""

import logging
import pytest

from api.utils.pii import mask_email, mask_str, mask_pii


# ---------------------------------------------------------------------------
# mask_email
# ---------------------------------------------------------------------------

class TestMaskEmail:
    def test_standard_email(self):
        assert mask_email("alice@example.com") == "a***@example.com"

    def test_single_char_local(self):
        assert mask_email("b@example.com") == "b***@example.com"

    def test_subdomain(self):
        assert mask_email("user@mail.corp.example.com") == "u***@mail.corp.example.com"

    def test_empty_string(self):
        assert mask_email("") == "***"

    def test_not_an_email(self):
        assert mask_email("notanemail") == "***"

    def test_multiple_at_signs(self):
        # Only the first "@" is used as the split point
        result = mask_email("foo@bar@baz.com")
        assert result == "f***@bar@baz.com"

    def test_none_like_falsy(self):
        # Passing None should be guarded by callers, but empty-string boundary
        assert mask_email("") == "***"


# ---------------------------------------------------------------------------
# mask_str
# ---------------------------------------------------------------------------

class TestMaskStr:
    def test_long_string(self):
        assert mask_str("abcdefgh") == "abcd***"

    def test_shorter_than_keep(self):
        assert mask_str("ab") == "ab***"

    def test_exact_keep_length(self):
        assert mask_str("abcd") == "abcd***"

    def test_empty_string(self):
        assert mask_str("") == "***"

    def test_custom_keep(self):
        assert mask_str("hello-world", keep=2) == "he***"

    def test_keep_zero(self):
        assert mask_str("secret", keep=0) == "***"

    def test_uuid_like(self):
        result = mask_str("550e8400-e29b-41d4-a716-446655440000")
        assert result == "550e***"
        assert "8400" not in result


# ---------------------------------------------------------------------------
# mask_pii
# ---------------------------------------------------------------------------

class TestMaskPii:
    def test_email_field_masked(self):
        data = {"email": "bob@example.com", "id": "123"}
        result = mask_pii(data, ["email"])
        assert result["email"] == "b***@example.com"
        assert result["id"] == "123"

    def test_non_email_field_masked(self):
        data = {"name": "Robert Smith", "role": "admin"}
        result = mask_pii(data, ["name"])
        assert result["name"] == "Robe***"
        assert result["role"] == "admin"

    def test_multiple_fields(self):
        data = {"email": "carol@example.com", "ssn": "123-45-6789", "city": "Boston"}
        result = mask_pii(data, ["email", "ssn"])
        assert "@" not in result["email"] or result["email"].startswith("c***@")
        assert result["ssn"] == "123-***"
        assert result["city"] == "Boston"

    def test_absent_field_ignored(self):
        data = {"id": "abc"}
        result = mask_pii(data, ["email", "phone"])
        assert result == {"id": "abc"}

    def test_none_value_ignored(self):
        data = {"email": None, "name": ""}
        result = mask_pii(data, ["email", "name"])
        # Falsy values are left as-is
        assert result["email"] is None
        assert result["name"] == ""

    def test_original_dict_not_mutated(self):
        data = {"email": "dave@example.com"}
        original_email = data["email"]
        mask_pii(data, ["email"])
        assert data["email"] == original_email

    def test_returns_new_dict(self):
        data = {"email": "eve@example.com"}
        result = mask_pii(data, ["email"])
        assert result is not data

    def test_empty_fields_list(self):
        data = {"email": "frank@example.com"}
        result = mask_pii(data, [])
        assert result == data

    def test_empty_data(self):
        result = mask_pii({}, ["email"])
        assert result == {}


# ---------------------------------------------------------------------------
# Log-output integration: verify no raw PII reaches log records
# ---------------------------------------------------------------------------

class TestNoPiiInLogs:
    def test_masked_email_not_in_log(self, caplog):
        with caplog.at_level(logging.DEBUG):
            logger = logging.getLogger("test.pii")
            email = "grace@example.com"
            logger.info("Login attempt for: %s", mask_email(email))

        assert "grace@example.com" not in caplog.text
        assert "grace" not in caplog.text.split("Login attempt for: ")[1]

    def test_masked_dict_not_in_log(self, caplog):
        with caplog.at_level(logging.DEBUG):
            logger = logging.getLogger("test.pii")
            record = {"email": "heidi@example.com", "ssn": "999-88-7777", "id": "u-001"}
            safe = mask_pii(record, ["email", "ssn"])
            logger.debug("User record: %s", safe)

        assert "heidi@example.com" not in caplog.text
        assert "999-88-7777" not in caplog.text
        # Non-PII id should still be present
        assert "u-001" in caplog.text
