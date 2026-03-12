"""
Compliance-aware HTTP client wrapping the `requests` library.

All outbound HTTP calls from nw-agent MUST use this module instead of
importing `requests` directly. This ensures:
  - TLS certificate verification is always enabled (no verify=False).
  - Timeouts are enforced to prevent hung connections.
  - Every request is logged with method, host, status code, and data
    classification tag for audit trail (GDPR, HIPAA, SOX, PCI-DSS).
  - No request body / response body is logged to avoid leaking regulated data.

Data classification levels (pass via ``data_classification`` kwarg):
  - "public"         — non-sensitive, freely shareable data
  - "internal"       — internal-only, not regulated (default)
  - "confidential"   — business-sensitive, restricted access
  - "restricted"     — regulated data (PII, PHI, PCI); requires explicit tagging
"""

import logging
import urllib.parse

import requests
from requests import Response

logger = logging.getLogger(__name__)

# Default timeouts (connect, read) in seconds.
_DEFAULT_TIMEOUT = (10, 30)

# Valid data classification levels for compliance audit tagging.
_VALID_CLASSIFICATIONS = {"public", "internal", "confidential", "restricted"}


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
    """Execute an HTTP request with compliance controls enforced.

    Args:
        method: HTTP verb (GET, POST, etc.).
        url: Destination URL.
        data_classification: Audit tag indicating the sensitivity of data
            transmitted. Must be one of "public", "internal", "confidential",
            or "restricted". Defaults to "internal". Pass "restricted" for
            any request involving PII, PHI, or payment-card data.
        **kwargs: Forwarded to ``requests.request`` (minus ``data_classification``).
    """
    # Extract compliance-specific kwarg before forwarding to requests.
    data_classification = kwargs.pop("data_classification", "internal")
    if data_classification not in _VALID_CLASSIFICATIONS:
        raise ValueError(
            f"Invalid data_classification={data_classification!r}. "
            f"Must be one of {sorted(_VALID_CLASSIFICATIONS)}."
        )

    # Enforce TLS verification — never allow callers to disable it.
    kwargs["verify"] = True

    # Enforce a timeout unless the caller explicitly provides one.
    if "timeout" not in kwargs:
        kwargs["timeout"] = _DEFAULT_TIMEOUT

    host = urllib.parse.urlparse(url).netloc
    logger.info(
        "http_request method=%s host=%s data_classification=%s tls_verify=true",
        method,
        host,
        data_classification,
    )

    response = requests.request(method, url, **kwargs)

    logger.info(
        "http_response method=%s host=%s status=%d data_classification=%s",
        method,
        host,
        response.status_code,
        data_classification,
    )
    return response
