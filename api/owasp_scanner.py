"""OWASP vulnerability scanner API."""

import os
import re
import logging
from urllib.parse import urlparse

from flask import Flask, request

logger = logging.getLogger(__name__)

app = Flask(__name__)

_ORIGIN_RE = re.compile(r'^https?://[^/*]+$')


def _validate_origin(origin: str) -> bool:
    """Return True if *origin* is a well-formed http/https URL with no wildcards."""
    try:
        parsed = urlparse(origin)
        return bool(_ORIGIN_RE.match(origin) and parsed.scheme in ("http", "https") and parsed.netloc)
    except Exception:
        return False


def _get_allowed_origins():
    """Return validated trusted origins from the ALLOWED_ORIGINS env var."""
    raw = os.environ.get("ALLOWED_ORIGINS", "")
    validated = []
    for o in raw.split(","):
        o = o.strip()
        if o:
            if _validate_origin(o):
                validated.append(o)
            else:
                logger.warning("ALLOWED_ORIGINS: skipping invalid entry %r", o)
    return validated


def _build_cors_response(response, request_origin: str):
    """Set CORS response headers for trusted origins only."""
    allowed = _get_allowed_origins()

    if not allowed:
        logger.warning(
            "ALLOWED_ORIGINS is not set. All cross-origin requests will be denied. "
            "Set ALLOWED_ORIGINS to a comma-separated list of trusted origins."
        )
        return response

    # line 109 — Access-Control-Allow-Origin
    if request_origin and request_origin in allowed:
        response.headers["Access-Control-Allow-Origin"] = request_origin

        # line 113 — Access-Control-Allow-Credentials
        # Only valid when a specific (non-wildcard) origin is set.
        response.headers["Access-Control-Allow-Credentials"] = "true"
        logger.info("CORS: permitted origin=%r", request_origin)
    else:
        logger.info("CORS: denied origin=%r (not in trusted list)", request_origin or "(none)")

    return response


@app.after_request
def after_request(response):
    origin = request.headers.get("Origin", "")
    return _build_cors_response(response, origin)
