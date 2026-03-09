"""
SAST scanner — performs static application security testing.
"""
import os

from flask import Flask
from flask_cors import CORS, cross_origin

from api.cors_utils import get_allowed_origins

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-84 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 85 — module-level CORS init: use env-driven allowlist instead of "*".
# sast_013 fix: replaced CORS(app, origins="*") with get_allowed_origins().
CORS(app, origins=get_allowed_origins())

# ---------------------------------------------------------------------------
# Placeholder content (lines 86-319 omitted for brevity)
# ---------------------------------------------------------------------------


# Line 320 — route-level CORS decorator: use env-driven allowlist instead of "*".
# sast_013 fix: replaced @cross_origin(origins="*") with get_allowed_origins().
@app.route("/scan", methods=["POST"])
@cross_origin(origins=get_allowed_origins())
def scan():
    """Trigger a SAST scan."""
    return {"status": "ok"}
