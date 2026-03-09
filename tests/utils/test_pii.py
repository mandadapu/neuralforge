"""Unit tests for api.utils.pii masking helpers."""
import pytest

from api.utils.pii import mask_email, mask_pii


class TestMaskEmail:
    def test_standard_email(self):
        assert mask_email("alice@example.com") == "al***@example.com"

    def test_short_local_part(self):
        # local part with only 1 character — still show first 2 chars (which is just 1)
        result = mask_email("a@example.com")
        assert "@example.com" in result
        assert "***" in result

    def test_invalid_email_returns_redacted(self):
        assert mask_email("not-an-email") == "***"

    def test_empty_string_returns_redacted(self):
        assert mask_email("") == "***"

    def test_preserves_domain(self):
        result = mask_email("bob@corp.internal")
        assert result.endswith("@corp.internal")

    def test_raw_address_not_in_output(self):
        raw = "charlie@hospital.org"
        result = mask_email(raw)
        assert raw not in result


class TestMaskPii:
    def test_flat_dict_sensitive_keys_redacted(self):
        data = {"email": "user@example.com", "password": "s3cr3t", "status": "active"}
        result = mask_pii(data)
        assert result["email"] == "***"
        assert result["password"] == "***"
        # Non-sensitive key preserved
        assert result["status"] == "active"

    def test_nested_dict_recursive_masking(self):
        data = {
            "user": {
                "name": "Alice",
                "email": "alice@example.com",
                "role": "admin",
            }
        }
        result = mask_pii(data)
        assert result["user"]["name"] == "***"
        assert result["user"]["email"] == "***"
        assert result["user"]["role"] == "admin"

    def test_list_elements_masked(self):
        data = [{"email": "a@b.com", "id": 1}, {"email": "c@d.com", "id": 2}]
        result = mask_pii(data)
        assert result[0]["email"] == "***"
        assert result[0]["id"] == 1
        assert result[1]["email"] == "***"

    def test_tuple_elements_masked(self):
        data = ({"username": "alice"}, {"username": "bob"})
        result = mask_pii(data)
        assert isinstance(result, tuple)
        assert result[0]["username"] == "***"
        assert result[1]["username"] == "***"

    def test_non_dict_scalar_passed_through(self):
        assert mask_pii(42) == 42
        assert mask_pii("hello") == "hello"
        assert mask_pii(None) is None

    def test_all_sensitive_keys_redacted(self):
        sensitive_keys = [
            "email", "password", "token", "secret", "ssn", "dob",
            "date_of_birth", "phone", "address", "name", "first_name",
            "last_name", "username", "patient_id", "user_id",
            "recipient", "login", "credential",
        ]
        data = {k: "sensitive-value" for k in sensitive_keys}
        result = mask_pii(data)
        for k in sensitive_keys:
            assert result[k] == "***", f"Expected {k!r} to be masked"

    def test_case_insensitive_key_matching(self):
        data = {"Email": "user@example.com", "PASSWORD": "hunter2"}
        result = mask_pii(data)
        assert result["Email"] == "***"
        assert result["PASSWORD"] == "***"

    def test_empty_dict(self):
        assert mask_pii({}) == {}

    def test_empty_list(self):
        assert mask_pii([]) == []

    def test_non_sensitive_keys_preserved(self):
        data = {"scan_id": "abc-123", "status": "complete", "timestamp": "2026-03-09"}
        result = mask_pii(data)
        assert result == data

    def test_raw_pii_not_in_string_repr(self):
        """Ensure str() of the masked dict does not contain raw email."""
        data = {"email": "patient@clinic.org", "scan_id": "x99"}
        result = mask_pii(data)
        assert "patient@clinic.org" not in str(result)
