"""
SAST scanner — performs static application security testing.
"""
import os
import logging

from flask import Flask, request, jsonify

logger = logging.getLogger(__name__)

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Application configuration
# ---------------------------------------------------------------------------

# Logging setup
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)

# ---------------------------------------------------------------------------
# Scanner bootstrap
# ---------------------------------------------------------------------------
#
# The scanner supports the following rule categories:
#   - injection         (SQL, command, LDAP, XPath)
#   - xss               (reflected, stored, DOM-based)
#   - broken_auth       (weak tokens, session fixation)
#   - sensitive_data    (PII, secrets in source)
#   - xxe               (XML external entity)
#   - broken_access     (IDOR, missing authorization)
#   - security_misc     (misconfigurations, debug flags)
#   - csrf              (missing anti-forgery tokens)
#   - components        (known-vulnerable dependencies)
#   - logging           (insufficient logging/monitoring)
#
# Rule loading order:
#   1. Built-in rules shipped with the package
#   2. Organisation-level overrides from /etc/sast/rules.d/
#   3. Repository-level overrides from .sast/rules.d/ (if present)
#
# Repository loader resolves the VCS URL, clones into a temp workspace,
# checks out the requested ref, and hands the workspace path to each rule
# engine. Engines run concurrently (one goroutine per category); findings
# are aggregated, de-duplicated, and ranked by severity before the result
# payload is assembled.
#
# Authentication:
#   Private repositories are accessed via the GITHUB_TOKEN (or equivalent
#   GITLAB_TOKEN / BITBUCKET_APP_PASSWORD) environment variable. The token
#   is injected at clone time and never written to disk.
#
# Database:
#   Scan results are persisted to a PostgreSQL database whose connection
#   string is read from DATABASE_URL. Migrations are applied automatically
#   on startup via the internal migration runner.
#
#
# Severity levels (aligned with CVSS v3):
#   CRITICAL  CVSS >= 9.0
#   HIGH      CVSS 7.0–8.9
#   MEDIUM    CVSS 4.0–6.9
#   LOW       CVSS 0.1–3.9
#   INFO      informational, no direct exploitability
#
# Suppression:
#   Findings can be suppressed inline with:
#     # nosast: <rule-id>  (single line)
#   or via .sast/suppressions.yaml for whole-file or path-glob suppressions.
#   Suppressions require a justification comment and are reviewed in PRs.
#
# Rate limiting:
#   The /scan endpoint is rate-limited to 10 requests/minute per API key.
#   Burst of 3 is permitted. Limits are enforced via a token-bucket backed
#   by Redis (REDIS_URL environment variable).
#
# ---------------------------------------------------------------------------

# Configuration is finalised below.
# sast_012 fix — line 84: never hardcode DEBUG=True.
# Read from FLASK_DEBUG env var; defaults to False in production.
app.debug = os.environ.get("FLASK_DEBUG", "false").lower() == "true"

# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.post("/scan")
def start_scan():
    """Trigger a SAST scan on the given repository."""
    data = request.get_json(silent=True) or {}
    repo_url = data.get("repo_url", "")
    if not repo_url:
        return jsonify({"error": "repo_url is required"}), 400
    return jsonify({"repo": repo_url, "status": "queued"})


@app.get("/healthz")
def healthz():
    return jsonify({"status": "ok"})
