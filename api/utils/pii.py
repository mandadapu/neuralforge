"""
PII/PHI masking utilities.

Use these helpers before logging any data that may contain personally
identifiable information (PII) or protected health information (PHI).
Complies with HIPAA § 164.312 and GDPR Art. 32 requirements to avoid
logging sensitive fields in plaintext.
"""

from __future__ import annotations


def mask_email(email: str) -> str:
    """Return a masked email: first char + *** + @domain.

    Examples:
        mask_email("alice@example.com") -> "a***@example.com"
        mask_email("")                  -> "***"
        mask_email("notanemail")        -> "***"
    """
    if not email or "@" not in email:
        return "***"
    local, domain = email.split("@", 1)
    return local[0] + "***@" + domain


def mask_str(value: str, keep: int = 4) -> str:
    """Mask a string, keeping only the first `keep` characters.

    Args:
        value: The string to mask.
        keep:  Number of leading characters to retain (default 4).

    Examples:
        mask_str("user-uuid-1234")  -> "user***"
        mask_str("ab", keep=4)      -> "ab***"
        mask_str("")                -> "***"
    """
    if not value:
        return "***"
    return value[:keep] + "***"


def mask_pii(data: dict, fields: list[str]) -> dict:
    """Return a shallow copy of *data* with listed fields masked.

    Fields whose values contain "@" are treated as email addresses and
    routed through :func:`mask_email`; all others are routed through
    :func:`mask_str`.  Fields absent from *data* or with falsy values
    are left unchanged.

    Args:
        data:   Mapping to sanitise (not mutated).
        fields: List of key names that may contain PII/PHI.

    Returns:
        A new dict safe for logging.

    Example::

        safe = mask_pii(user_record, ["email", "name", "ssn", "dob"])
        logger.debug("User record: %s", safe)
    """
    result = dict(data)
    for field in fields:
        if field in result and result[field]:
            val = str(result[field])
            if "@" in val:
                result[field] = mask_email(val)
            else:
                result[field] = mask_str(val)
    return result
