"""OWASP vulnerability scanner API."""

import os
import logging

from flask import Flask, request, jsonify

logger = logging.getLogger(__name__)

app = Flask(__name__)


def _get_allowed_origins():
    """Return the list of allowed CORS origins from the environment."""
    raw = os.environ.get("ALLOWED_ORIGINS", "")
    return [o.strip() for o in raw.split(",") if o.strip()]


def _build_cors_response(response, request_origin: str):
    """
    Apply CORS headers to *response* only when *request_origin* is in the
    configured allowlist (sast_013 fix — lines 109 and 113).

    Access-Control-Allow-Origin is set to the specific request origin rather
    than '*', and Access-Control-Allow-Credentials is only set to 'true' when
    a specific (non-wildcard) origin is matched — as required by the CORS spec.

    ALLOWED_ORIGINS environment variable: comma-separated list of full origins.
    Example: ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com
    """
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

    return response


@app.after_request
def after_request(response):
    origin = request.headers.get("Origin", "")
    return _build_cors_response(response, origin)
