"""
GitHub scanner — analyses GitHub repositories for security issues.
"""

from flask import Flask
from flask_cors import CORS

from api.cors_utils import get_allowed_origins

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-664 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 665 — CORS setup: use env-driven allowlist (sast_013 fix).
CORS(app, origins=get_allowed_origins())
