"""
Compliance-aware HTTP client wrapping the `requests` library.

All outbound HTTP calls from nw-agent MUST use this module instead of
importing `requests` directly. This ensures:
  - TLS certificate verification is always enabled (no verify=False).
  - Timeouts are enforced to prevent hung connections.
  - Every request is logged with method, host, and status code for audit.
  - No request body / response body is logged to avoid leaking regulated data.
"""

import logging
import urllib.parse

import requests
from requests import Response

logger = logging.getLogger(__name__)

# Default timeouts (connect, read) in seconds.
_DEFAULT_TIMEOUT = (10, 30)


def get(url: str, **kwargs) -> Response:
    return _request("GET", url, **kwargs)


def post(url: str, **kwargs) -> Response:
    return _request("POST", url, **kwargs)


def put(url: str, **kwargs) -> Response:
    return _request("PUT", url, **kwargs)


def patch(url: str, **kwargs) -> Response:
    return _request("PATCH", url, **kwargs)


def delete(url: str, **kwargs) -> Response:
    return _request("DELETE", url, **kwargs)


def _request(method: str, url: str, **kwargs) -> Response:
    """Execute an HTTP request with compliance controls enforced."""
    # Enforce TLS verification — never allow callers to disable it.
    kwargs["verify"] = True

    # Enforce a timeout unless the caller explicitly provides one.
    if "timeout" not in kwargs:
        kwargs["timeout"] = _DEFAULT_TIMEOUT

    host = urllib.parse.urlparse(url).netloc
    logger.info("http_request method=%s host=%s", method, host)

    response = requests.request(method, url, **kwargs)

    logger.info(
        "http_response method=%s host=%s status=%d",
        method,
        host,
        response.status_code,
    )
    return response
