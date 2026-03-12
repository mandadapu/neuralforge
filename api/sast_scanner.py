"""
SAST scanner — performs static application security testing.
"""

from flask import Flask
from flask_cors import CORS, cross_origin

from api.cors_utils import get_allowed_origins

app = Flask(__name__)

CORS(app, origins=get_allowed_origins())


@app.route("/scan", methods=["POST"])
@cross_origin(origins=get_allowed_origins())
def scan():
    """Trigger a SAST scan."""
    return {"status": "ok"}
