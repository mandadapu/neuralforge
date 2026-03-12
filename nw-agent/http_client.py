"""
Compliance-aware HTTP client wrapping the requests library.

Data transmission policy (GDPR / SOX / HIPAA):
  - This client is for outbound HTTP calls from the nw-agent component only.
  - No personal data (PII), protected health information (PHI), or financial
    data (PCI/SOX) must be transmitted without prior data-classification review
    and legal-basis documentation per GDPR Art. 6 / Art. 9.
  - All destinations must be registered in the service's data-flow inventory
    before use.

Security controls enforced by this module:
  - HTTPS only: plain-HTTP URLs are rejected at call time.
  - Certificate verification enabled; never pass verify=False in production.
  - Default connect/read timeouts prevent indefinite data-exposure windows.
  - Response bodies are never logged to avoid leaking sensitive data.
  - Audit log captures destination URL, HTTP method, and data-type tag for
    every request to satisfy SOX change-management and HIPAA access-log
    requirements.
"""

import logging
import requests
from requests import Response

logger = logging.getLogger(__name__)

# Default timeout: (connect_timeout_s, read_timeout_s)
_DEFAULT_TIMEOUT = (5, 30)


class ComplianceHTTPClient:
    """
    Thin wrapper around requests.Session that enforces security and compliance
    controls.  Use this class for all outbound HTTP calls; do not import
    requests directly in application code.

    Parameters
    ----------
    data_type : str
        Classification of the data being transmitted (e.g. "public",
        "internal", "pii", "phi", "pci").  Required for audit logging.
    timeout : tuple[int, int]
        (connect_s, read_s) timeout pair.  Defaults to (5, 30).
    """

    def __init__(
        self,
        data_type: str,
        timeout: tuple = _DEFAULT_TIMEOUT,
    ) -> None:
        if not data_type:
            raise ValueError("data_type must be specified for compliance audit logging")
        self._data_type = data_type
        self._timeout = timeout
        self._session = requests.Session()
        # Certificate verification must always be enabled.
        self._session.verify = True

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def get(self, url: str, **kwargs) -> Response:
        return self._request("GET", url, **kwargs)

    def post(self, url: str, **kwargs) -> Response:
        return self._request("POST", url, **kwargs)

    def put(self, url: str, **kwargs) -> Response:
        return self._request("PUT", url, **kwargs)

    def patch(self, url: str, **kwargs) -> Response:
        return self._request("PATCH", url, **kwargs)

    def delete(self, url: str, **kwargs) -> Response:
        return self._request("DELETE", url, **kwargs)

    def close(self) -> None:
        self._session.close()

    def __enter__(self) -> "ComplianceHTTPClient":
        return self

    def __exit__(self, *args) -> None:
        self.close()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _request(self, method: str, url: str, **kwargs) -> Response:
        self._enforce_https(url)
        kwargs.setdefault("timeout", self._timeout)
        # Guarantee verify=True cannot be overridden by callers.
        kwargs["verify"] = True

        self._audit_log(method, url)

        response = self._session.request(method, url, **kwargs)

        # Log status only — never log response body to avoid leaking
        # sensitive data (PII, tokens, PHI).
        logger.info(
            "http_response status=%d method=%s url=%s data_type=%s",
            response.status_code,
            method,
            url,
            self._data_type,
        )

        return response

    @staticmethod
    def _enforce_https(url: str) -> None:
        if not url.lower().startswith("https://"):
            raise ValueError(
                f"Compliance policy requires HTTPS. Rejected URL: {url}"
            )

    def _audit_log(self, method: str, url: str) -> None:
        """Emit a structured audit entry for SOX/HIPAA access logging."""
        logger.info(
            "http_request method=%s url=%s data_type=%s",
            method,
            url,
            self._data_type,
        )
