"""
Controlled HTTP client for nw-agent outbound requests.

All external calls (GitHub API, Anthropic API) MUST use make_request() or the
session returned by build_session() rather than calling requests directly.
This ensures TLS 1.2+, certificate validation, timeouts, and audit logging are
enforced uniformly for every outbound request.

Approved endpoints:
  - https://api.github.com     (GitHub REST API)
  - https://api.anthropic.com  (Anthropic API)
"""

import logging
import ssl
import requests
from requests.adapters import HTTPAdapter
from urllib3.util.ssl_ import create_urllib3_context

logger = logging.getLogger(__name__)

# Default per-request timeout (connect, read) in seconds.
DEFAULT_TIMEOUT = (10, 30)

# Approved destination hostnames; requests to other hosts are blocked.
APPROVED_HOSTS = {"api.github.com", "api.anthropic.com"}


class _TLS12EnforcingAdapter(HTTPAdapter):
    """Transport adapter that enforces TLS 1.2 as the minimum protocol."""

    def init_poolmanager(self, *args, **kwargs):
        ctx = create_urllib3_context(ssl_minimum_version=ssl.TLSVersion.TLSv1_2)
        kwargs["ssl_context"] = ctx
        super().init_poolmanager(*args, **kwargs)


def build_session() -> requests.Session:
    """
    Return a requests.Session pre-configured with security controls:
      - TLS 1.2+ enforced via a custom transport adapter
      - Certificate validation enabled (verify=True, the library default)
    """
    session = requests.Session()
    adapter = _TLS12EnforcingAdapter()
    session.mount("https://", adapter)
    # Explicitly disable plain HTTP to prevent accidental downgrade.
    session.mount("http://", _BlockHTTPAdapter())
    return session


class _BlockHTTPAdapter(HTTPAdapter):
    """Rejects any non-TLS request at the transport layer."""

    def send(self, request, **kwargs):  # type: ignore[override]
        raise ValueError(
            f"Plain HTTP requests are not permitted. Use HTTPS. URL: {request.url}"
        )


def make_request(
    method: str,
    url: str,
    *,
    session: requests.Session | None = None,
    timeout: tuple[int, int] = DEFAULT_TIMEOUT,
    **kwargs,
) -> requests.Response:
    """
    Make a single outbound HTTPS request with all security controls applied.

    Args:
        method:  HTTP method (GET, POST, PATCH, …).
        url:     Full HTTPS URL. Must resolve to an approved host.
        session: Optional pre-built session (from build_session()). A
                 temporary session is created and closed if not provided.
        timeout: (connect_timeout, read_timeout) in seconds.
        **kwargs: Forwarded verbatim to requests.Session.request().

    Returns:
        requests.Response

    Raises:
        ValueError: if the URL targets a non-approved host.
        requests.exceptions.RequestException: on network/HTTP errors.
    """
    from urllib.parse import urlparse

    host = urlparse(url).hostname or ""
    if host not in APPROVED_HOSTS:
        raise ValueError(
            f"Outbound request to '{host}' is not approved. "
            f"Approved hosts: {sorted(APPROVED_HOSTS)}"
        )

    # kwargs must not override verify=True.
    kwargs.setdefault("verify", True)
    if not kwargs.get("verify", True):
        raise ValueError("Certificate verification (verify=True) cannot be disabled.")

    logger.debug("outbound_request method=%s url=%s", method.upper(), url)

    own_session = session is None
    _session = build_session() if own_session else session
    try:
        response = _session.request(method, url, timeout=timeout, **kwargs)
        logger.debug(
            "outbound_response method=%s url=%s status=%d",
            method.upper(),
            url,
            response.status_code,
        )
        return response
    finally:
        if own_session:
            _session.close()
