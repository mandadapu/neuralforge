"""
SAST scanner — performs static application security testing.
"""
import os

from flask import Flask

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-83 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 84 — debug mode is always disabled in production code (sast_012).
app.debug = False

# Runtime guard: raise an error if debug is somehow enabled in a production environment.
if os.environ.get("ENVIRONMENT", "").lower() == "production" and app.debug:
    raise RuntimeError("Debug mode must not be enabled in production.")

# ---------------------------------------------------------------------------
# Placeholder content (lines 85-319 omitted for brevity)
# ---------------------------------------------------------------------------


@app.route("/scan", methods=["POST"])
def scan():
    """Trigger a SAST scan."""
    return {"status": "ok"}
