"""
github_scanner.py — GitHub repository security scanner API.
"""
import os
import logging
import re
from typing import Dict, List, Optional

from flask import Flask, jsonify, request
from flask_cors import CORS

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# CORS configuration — origins read from environment, no wildcard
# ---------------------------------------------------------------------------
_raw_origins = os.environ.get("CORS_ALLOWED_ORIGINS", "")
ALLOWED_ORIGINS = [o.strip() for o in _raw_origins.split(",") if o.strip()]

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Scanner logic
# ---------------------------------------------------------------------------

GITHUB_API_BASE = "https://api.github.com"

_SECRET_PATTERNS = [
    re.compile(r"(?i)(api[_\-]?key|secret|password|token)\s*=\s*['\"][^'\"]{8,}['\"]"),
    re.compile(r"(?i)aws_access_key_id\s*=\s*[A-Z0-9]{20}"),
    re.compile(r"(?i)sk-[a-zA-Z0-9]{32,}"),
]


def _get_github_token() -> str:
    token = os.environ.get("GITHUB_TOKEN", "")
    if not token:
        raise EnvironmentError("GITHUB_TOKEN environment variable is not set")
    return token


def scan_file_content(content: str) -> List[Dict]:
    """Return a list of findings for the given file content."""
    findings = []
    for i, line in enumerate(content.splitlines(), start=1):
        for pattern in _SECRET_PATTERNS:
            if pattern.search(line):
                findings.append({
                    "line": i,
                    "snippet": line.strip()[:120],
                    "rule": "secret_exposure",
                    "severity": "HIGH",
                })
    return findings


def scan_repo(owner: str, repo: str, ref: Optional[str] = None) -> Dict:
    """
    Scan a GitHub repository for secrets and return findings.

    :param owner: GitHub organisation or user name.
    :param repo:  Repository name.
    :param ref:   Git ref (branch / tag / SHA). Defaults to HEAD.
    :returns:     Dict with ``repo``, ``ref``, and ``findings`` keys.
    """
    import urllib.request
    import json

    token = _get_github_token()
    ref = ref or "HEAD"

    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github.v3+json",
    }

    tree_url = f"{GITHUB_API_BASE}/repos/{owner}/{repo}/git/trees/{ref}?recursive=1"
    req = urllib.request.Request(tree_url, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as resp:  # noqa: S310
        tree_data = json.loads(resp.read())

    all_findings: List[Dict] = []
    for node in tree_data.get("tree", []):
        if node.get("type") != "blob":
            continue
        path: str = node["path"]
        if not any(path.endswith(ext) for ext in (".py", ".js", ".ts", ".yaml", ".yml", ".env")):
            continue

        blob_url = f"{GITHUB_API_BASE}/repos/{owner}/{repo}/git/blobs/{node['sha']}"
        breq = urllib.request.Request(blob_url, headers=headers)
        with urllib.request.urlopen(breq, timeout=30) as bresp:  # noqa: S310
            blob_data = json.loads(bresp.read())

        import base64
        try:
            content = base64.b64decode(blob_data.get("content", "")).decode("utf-8", errors="replace")
        except Exception:
            continue

        file_findings = scan_file_content(content)
        for f in file_findings:
            f["file"] = path
        all_findings.extend(file_findings)

    return {"repo": f"{owner}/{repo}", "ref": ref, "findings": all_findings}


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.route("/scan", methods=["POST"])
def scan_endpoint():
    body = request.get_json(silent=True) or {}
    owner = body.get("owner", "")
    repo = body.get("repo", "")
    ref = body.get("ref")
    if not owner or not repo:
        return jsonify({"error": "owner and repo are required"}), 400
    try:
        result = scan_repo(owner, repo, ref)
    except EnvironmentError as exc:
        logger.error("Configuration error: %s", exc)
        return jsonify({"error": str(exc)}), 500
    except Exception as exc:
        logger.exception("Scan failed for %s/%s: %s", owner, repo, exc)
        return jsonify({"error": "scan failed"}), 500
    return jsonify(result), 200


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"}), 200


# ---------------------------------------------------------------------------
# Application factory / entrypoint
# ---------------------------------------------------------------------------

def create_app():
    # line 665 — restrict CORS to trusted origins (no wildcard)
    CORS(app, origins=ALLOWED_ORIGINS)
    return app


if __name__ == "__main__":
    application = create_app()
    port = int(os.environ.get("PORT", 5001))
    application.run(host="0.0.0.0", port=port)
