"""GitHub repository scanner API."""

import os
import logging

from flask import Flask
from flask_cors import CORS

logger = logging.getLogger(__name__)

app = Flask(__name__)

# ---------------------------------------------------------------------------
# CORS configuration — line 665 (sast_013 fix)
# Read allowed origins from the environment variable ALLOWED_ORIGINS.
# Format: comma-separated list of full origins.
# Example: ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com
# If the variable is unset or empty, no wildcard is used and CORS requests
# from unknown origins will be denied.
# ---------------------------------------------------------------------------
_allowed_origins_env = os.environ.get("ALLOWED_ORIGINS", "")
_allowed_origins = [o.strip() for o in _allowed_origins_env.split(",") if o.strip()]

if not _allowed_origins:
    logger.warning(
        "ALLOWED_ORIGINS is not set. All cross-origin requests will be denied. "
        "Set ALLOWED_ORIGINS to a comma-separated list of trusted origins."
    )

CORS(app, origins=_allowed_origins)
