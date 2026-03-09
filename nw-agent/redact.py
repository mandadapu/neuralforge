"""
Credential redaction utilities for safe logging.

Use these helpers to strip sensitive values from dicts and strings before
passing them to any logger, so that credentials never appear in log output.
"""

import re
from typing import Any

_SECRET_KEYS = re.compile(
    r"\b(password|passwd|secret|token|api_key|apikey|auth|credential|"
    r"private_key|access_key|authorization)\b",
    re.IGNORECASE,
)

_REDACTED = "***REDACTED***"


def redact_dict(d: Any, *, recursive: bool = True) -> Any:
    """Return a copy of *d* with sensitive values replaced by '***REDACTED***'.

    Args:
        d: The value to redact.  Non-dict values are returned unchanged.
        recursive: When True (default), redact nested dicts as well.

    Returns:
        A shallow (or deep, when recursive=True) copy of *d* with sensitive
        field values replaced.
    """
    if not isinstance(d, dict):
        return d
    result: dict = {}
    for k, v in d.items():
        if _SECRET_KEYS.search(str(k)):
            result[k] = _REDACTED
        elif recursive and isinstance(v, dict):
            result[k] = redact_dict(v, recursive=True)
        else:
            result[k] = v
    return result


def redact_str(value: str, keys: list[str] | None = None) -> str:
    """Redact known secret patterns from a JSON-like string.

    For each key in *keys*, replaces ``"<key>": "<value>"`` occurrences with
    ``"<key>": "***REDACTED***"``.  When *keys* is omitted the function scans
    for all keys matched by the default secret-key pattern.

    Args:
        value: The string to redact.
        keys: Explicit list of key names to redact.  If None, the default
              secret-key regex is used to find candidate keys.

    Returns:
        The redacted string.
    """
    if keys is None:
        value = re.sub(
            r'("(?:' + _SECRET_KEYS.pattern + r')"\\s*:\\s*)"[^"]*"',
            r'\1"' + _REDACTED + '"',
            value,
            flags=re.IGNORECASE,
        )
    else:
        for key in keys:
            value = re.sub(
                rf'("{re.escape(key)}"\\s*:\\s*)"[^"]*"',
                rf'\\1"' + _REDACTED + '"',
                value,
            )
    return value
