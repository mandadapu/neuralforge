"""Configuration patch handler for autopilot fix workflows."""

import os
import logging
from datetime import datetime, timezone
from urllib.parse import urlparse

from flask import Flask, request
from flask_cors import CORS

logger = logging.getLogger(__name__)

app = Flask(__name__)


def _validate_origin(origin: str) -> bool:
    """Return True if origin is a well-formed http/https URL with no wildcards."""
    if not origin or "*" in origin:
        return False
    try:
        parsed = urlparse(origin)
        return (
            parsed.scheme in ("http", "https")
            and bool(parsed.netloc)
            and not parsed.path.strip("/")
            and not parsed.query
            and not parsed.fragment
        )
    except Exception:
        return False


# ---------------------------------------------------------------------------
# CORS configuration — line 24 (sast_013 fix)
# Read allowed origins from the environment variable ALLOWED_ORIGINS.
# Format: comma-separated list of full origins.
# Example: ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com
# If the variable is unset or empty, no wildcard is used and CORS requests
# from unknown origins will be denied.
# ---------------------------------------------------------------------------
_allowed_origins_env = os.environ.get("ALLOWED_ORIGINS", "")
_allowed_origins = []
for _entry in _allowed_origins_env.split(","):
    _entry = _entry.strip()
    if _entry:
        if _validate_origin(_entry):
            _allowed_origins.append(_entry)
        else:
            logger.warning("CORS startup validation: ALLOWED_ORIGINS entry %r is invalid and will be skipped", _entry)

if not _allowed_origins:
    logger.error(
        "CORS startup validation FAILED: ALLOWED_ORIGINS is unset or contains no valid entries. "
        "All cross-origin requests will be denied. "
        "Set ALLOWED_ORIGINS to a comma-separated list of trusted origins "
        "(e.g. ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com)."
    )

CORS(app, origins=_allowed_origins)


@app.after_request
def _cors_audit_log(response):
    origin = request.headers.get("Origin", "")
    ts = datetime.now(timezone.utc).isoformat()
    request_id = request.headers.get("X-Request-Id", "-")
    if origin:
        if origin in _allowed_origins:
            logger.info(
                "CORS audit ts=%s request_id=%s method=%s path=%s origin=%r decision=allowed",
                ts, request_id, request.method, request.path, origin,
            )
        else:
            logger.warning(
                "CORS audit ts=%s request_id=%s method=%s path=%s origin=%r decision=denied reason=origin_not_trusted",
                ts, request_id, request.method, request.path, origin,
            )
    return response
