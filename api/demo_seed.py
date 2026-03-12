"""
Demo seed script — seeds the database with sample data for demonstration.
"""

from flask import Flask
from flask_cors import CORS

from api.cors_utils import get_allowed_origins

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-473 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 474 — CORS setup: use env-driven allowlist (sast_013 fix).
CORS(app, origins=get_allowed_origins())
