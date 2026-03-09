"""
sast_scanner.py — Static Application Security Testing (SAST) scanner API.
"""
import os
import ast
import logging
import re
from typing import Dict, List, Optional

from flask import Flask, Blueprint, jsonify, request
from flask_cors import CORS

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# CORS configuration — origins read from environment, no wildcard
# ---------------------------------------------------------------------------
_raw_origins = os.environ.get("CORS_ALLOWED_ORIGINS", "")
ALLOWED_ORIGINS = [o.strip() for o in _raw_origins.split(",") if o.strip()]

# ---------------------------------------------------------------------------
# SAST rule definitions
# ---------------------------------------------------------------------------

RULES = {
    "sast_001": {"name": "Hardcoded secret", "severity": "HIGH"},
    "sast_002": {"name": "SQL injection risk", "severity": "HIGH"},
    "sast_003": {"name": "Command injection risk", "severity": "HIGH"},
    "sast_010": {"name": "Insecure deserialization", "severity": "MEDIUM"},
    "sast_013": {"name": "Wildcard CORS origin", "severity": "MEDIUM"},
}

_HARDCODED_SECRET_RE = re.compile(
    r"(?i)(password|secret|api_?key|token)\s*=\s*['\"][^'\"]{6,}['\"]"
)
_SQL_INJECTION_RE = re.compile(
    r"(?i)(execute|executemany|cursor\.execute)\s*\(\s*['\"].*%s"
)
_CMD_INJECTION_RE = re.compile(
    r"(?i)(os\.system|subprocess\.call|subprocess\.run)\s*\(\s*['\"]"
)
_CORS_WILDCARD_RE = re.compile(
    r"""(?i)(allow_origins\s*=\s*\[['\"]\*['\"]\]|origins\s*=\s*['\"]\*['\"])"""
)


# ---------------------------------------------------------------------------
# Scanner functions
# ---------------------------------------------------------------------------

def _scan_line_patterns(lines: List[str]) -> List[Dict]:
    findings = []
    checks = [
        (_HARDCODED_SECRET_RE, "sast_001"),
        (_SQL_INJECTION_RE, "sast_002"),
        (_CMD_INJECTION_RE, "sast_003"),
        (_CORS_WILDCARD_RE, "sast_013"),
    ]
    for lineno, line in enumerate(lines, start=1):
        for pattern, rule_id in checks:
            if pattern.search(line):
                findings.append({
                    "line": lineno,
                    "rule": rule_id,
                    "severity": RULES[rule_id]["severity"],
                    "name": RULES[rule_id]["name"],
                    "snippet": line.strip()[:120],
                })
    return findings


# line 85 — app-level CORS applied with restricted origins (no wildcard)
app = Flask(__name__)
CORS(app, origins=ALLOWED_ORIGINS)

# ---------------------------------------------------------------------------
# Blueprint for v1 API
# ---------------------------------------------------------------------------
v1 = Blueprint("v1", __name__, url_prefix="/v1")


@v1.route("/scan", methods=["POST"])
def scan_endpoint():
    body = request.get_json(silent=True) or {}
    source = body.get("source", "")
    filename = body.get("filename", "<unknown>")
    if not source:
        return jsonify({"error": "source is required"}), 400

    lines = source.splitlines()
    findings = _scan_line_patterns(lines)
    return jsonify({"filename": filename, "findings": findings}), 200


@v1.route("/rules", methods=["GET"])
def list_rules():
    return jsonify({"rules": RULES}), 200


# ---------------------------------------------------------------------------
# Routes (app-level, legacy)
# ---------------------------------------------------------------------------

@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"}), 200


@app.route("/scan", methods=["POST"])
def legacy_scan():
    return scan_endpoint()


# ---------------------------------------------------------------------------
# Register blueprints
# ---------------------------------------------------------------------------
app.register_blueprint(v1)


# ---------------------------------------------------------------------------
# Application factory / entrypoint
# ---------------------------------------------------------------------------

def create_app():
    # line 320 — blueprint/router-level CORS with restricted origins (no wildcard)
    CORS(v1, origins=ALLOWED_ORIGINS)
    return app


if __name__ == "__main__":
    application = create_app()
    port = int(os.environ.get("PORT", 5003))
    application.run(host="0.0.0.0", port=port)
