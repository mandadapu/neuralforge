"""
SAST scanner — performs static application security testing.
"""
import os

from flask import Flask

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-83 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 84 — sast_012 fix: replaced hardcoded DEBUG=True with env-driven flag.
# Never enable debug mode in production; rely on the FLASK_DEBUG env var instead.
app.debug = os.environ.get("FLASK_DEBUG", "false").lower() == "true"

# ---------------------------------------------------------------------------
# Placeholder content (lines 85-319 omitted for brevity)
# ---------------------------------------------------------------------------


@app.route("/scan", methods=["POST"])
def scan():
    """Trigger a SAST scan."""
    return {"status": "ok"}
