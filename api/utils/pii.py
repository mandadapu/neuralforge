"""
PII/PHI masking utilities for safe logging.

HIPAA § 164.312 and GDPR Art. 32 compliant helpers to redact sensitive fields
before they appear in log output.
"""
import re
from typing import Any

_EMAIL_RE = re.compile(r"[^@]+@[^@]+\.[^@]+")

_SENSITIVE_KEYS = frozenset({
    "email", "password", "token", "secret", "ssn", "dob",
    "date_of_birth", "phone", "address", "name", "first_name",
    "last_name", "username", "patient_id", "user_id",
    "recipient", "login", "credential",
})


def mask_email(value: str) -> str:
    """Return a partially redacted email address for safe logging.

    Example: ``mask_email("alice@example.com")`` → ``"al***@example.com"``
    """
    if not value or not _EMAIL_RE.match(value):
        return "***"
    local, domain = value.split("@", 1)
    return f"{local[:2]}***@{domain}"


def mask_pii(data: Any) -> Any:
    """Recursively mask sensitive keys in a dict/list/tuple for safe logging.

    Keys present in ``_SENSITIVE_KEYS`` have their values replaced with
    ``"***"``.  All other keys and non-dict values are passed through
    unchanged, preserving diagnostic fields such as status codes, scan IDs,
    and timestamps.
    """
    if isinstance(data, dict):
        return {
            k: "***" if k.lower() in _SENSITIVE_KEYS else mask_pii(v)
            for k, v in data.items()
        }
    if isinstance(data, (list, tuple)):
        return type(data)(mask_pii(item) for item in data)
    return data
