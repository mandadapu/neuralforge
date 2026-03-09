"""
Config patch handler — applies configuration patches at runtime.
"""
from api.cors_utils import get_allowed_origins

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-23 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 24 — CORS config dict: use env-driven allowlist instead of "*".
# sast_013 fix: replaced {"origins": "*"} with get_allowed_origins().
cors_config = {"origins": get_allowed_origins()}
