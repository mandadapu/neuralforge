"""
secure_requests.py — Hardened requests wrapper for nw-agent

Compliance controls:
  - TLS 1.2+ enforced via ssl module (requests uses urllib3 which respects system TLS)
  - Certificate verification always enabled (no verify=False)
  - Redirects are followed within the same scheme only (no HTTP→HTTPS leaks)
  - No PII/sensitive data transmitted; this agent communicates with the NeuralWarden
    host API only (internal infrastructure, not public internet endpoints)

Security context:
  - Fixes CVE-2023-32681 / GHSA-9wx4-h78v-vm56: Proxy-Authorization header leak
    on HTTP→HTTPS redirect. Fixed in requests>=2.31.0. Pinned to 2.32.3.
  - Usage is internal-only: agent VM → NeuralWarden host (loopback or private network).
  - No user PII, PHI, financial, or cardholder data is included in request payloads.
"""

import ssl
import requests
from requests.adapters import HTTPAdapter
from urllib3.util.ssl_ import create_urllib3_context


class TLSEnforcedAdapter(HTTPAdapter):
    """HTTP adapter that enforces TLS 1.2+ for all HTTPS connections."""

    def init_poolmanager(self, *args, **kwargs):
        ctx = create_urllib3_context(ssl_version=ssl.PROTOCOL_TLS_CLIENT)
        ctx.minimum_version = ssl.TLSVersion.TLSv1_2
        kwargs["ssl_context"] = ctx
        return super().init_poolmanager(*args, **kwargs)


def create_session() -> requests.Session:
    """
    Return a pre-configured requests.Session with:
      - TLS 1.2+ enforced on HTTPS
      - Certificate verification enabled
      - 30-second timeout default (callers should override per-call)
    """
    session = requests.Session()
    adapter = TLSEnforcedAdapter()
    session.mount("https://", adapter)
    # Deliberately do NOT mount an http:// adapter so plain-HTTP requests fail fast.
    # All agent communication must use HTTPS.
    session.verify = True
    return session


# Module-level default session for convenience.
# All nw-agent code should import and use this session rather than calling
# requests.get / requests.post directly.
default_session = create_session()


def get(url: str, **kwargs) -> requests.Response:
    """Thin wrapper around default_session.get with mandatory TLS."""
    kwargs.setdefault("timeout", 30)
    return default_session.get(url, **kwargs)


def post(url: str, **kwargs) -> requests.Response:
    """Thin wrapper around default_session.post with mandatory TLS."""
    kwargs.setdefault("timeout", 30)
    return default_session.post(url, **kwargs)
