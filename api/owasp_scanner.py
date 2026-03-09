"""
owasp_scanner.py — OWASP vulnerability scanner API (FastAPI).
"""
import os
import logging
from typing import Dict, List, Optional

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# CORS configuration — origins read from environment, no wildcard
# ---------------------------------------------------------------------------
_raw_origins = os.environ.get("CORS_ALLOWED_ORIGINS", "")
ALLOWED_ORIGINS = [o.strip() for o in _raw_origins.split(",") if o.strip()]

app = FastAPI(title="OWASP Scanner", version="1.0.0")

# ---------------------------------------------------------------------------
# OWASP check helpers
# ---------------------------------------------------------------------------

OWASP_TOP10 = [
    "A01:2021-Broken_Access_Control",
    "A02:2021-Cryptographic_Failures",
    "A03:2021-Injection",
    "A04:2021-Insecure_Design",
    "A05:2021-Security_Misconfiguration",
    "A06:2021-Vulnerable_and_Outdated_Components",
    "A07:2021-Identification_and_Authentication_Failures",
    "A08:2021-Software_and_Data_Integrity_Failures",
    "A09:2021-Security_Logging_and_Monitoring_Failures",
    "A10:2021-Server-Side_Request_Forgery",
]


class ScanRequest(BaseModel):
    target_url: str
    checks: Optional[List[str]] = None
    timeout: int = 30


class Finding(BaseModel):
    rule: str
    severity: str
    description: str
    evidence: Optional[str] = None


class ScanResult(BaseModel):
    target: str
    findings: List[Finding]
    checks_run: List[str]


def _check_security_headers(target_url: str, timeout: int) -> List[Finding]:
    """Check for missing security headers."""
    import urllib.request

    findings: List[Finding] = []
    required_headers = {
        "X-Frame-Options": "A09",
        "X-Content-Type-Options": "A05",
        "Strict-Transport-Security": "A02",
        "Content-Security-Policy": "A05",
    }
    try:
        req = urllib.request.Request(target_url, method="HEAD")
        with urllib.request.urlopen(req, timeout=timeout) as resp:  # noqa: S310
            headers = {k.lower(): v for k, v in resp.headers.items()}
        for header, rule in required_headers.items():
            if header.lower() not in headers:
                findings.append(Finding(
                    rule=rule,
                    severity="MEDIUM",
                    description=f"Missing security header: {header}",
                    evidence=f"Header '{header}' not present in response",
                ))
    except Exception as exc:
        logger.warning("Header check failed for %s: %s", target_url, exc)
    return findings


def run_owasp_scan(target_url: str, checks: Optional[List[str]], timeout: int) -> ScanResult:
    """Run OWASP checks against *target_url*."""
    all_findings: List[Finding] = []
    checks_run: List[str] = []

    header_findings = _check_security_headers(target_url, timeout)
    all_findings.extend(header_findings)
    checks_run.append("security_headers")

    return ScanResult(target=target_url, findings=all_findings, checks_run=checks_run)


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.get("/health")
def health() -> Dict:
    return {"status": "ok"}


@app.post("/scan", response_model=ScanResult)
def scan(req: ScanRequest):
    try:
        result = run_owasp_scan(req.target_url, req.checks, req.timeout)
    except Exception as exc:
        logger.exception("OWASP scan failed: %s", exc)
        raise HTTPException(status_code=500, detail="scan failed") from exc
    return result


# ---------------------------------------------------------------------------
# CORS middleware — restrict to trusted origins (lines 109 and 113)
# ---------------------------------------------------------------------------
# line 109 — add CORSMiddleware with restricted allow_origins (no wildcard)
app.add_middleware(
    CORSMiddleware,
    # line 113 — allow_origins uses the env-var-driven list, not ["*"]
    allow_origins=ALLOWED_ORIGINS,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type"],
    allow_credentials=True,
)


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("PORT", 5002))
    uvicorn.run(app, host="0.0.0.0", port=port)
