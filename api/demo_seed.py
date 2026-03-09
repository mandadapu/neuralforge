"""
demo_seed.py — Demo data seeding API for NeuralWarden.
"""
import os
import logging
from flask import Flask, jsonify, request
from flask_cors import CORS

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# CORS configuration
# Allowed origins are read from the CORS_ALLOWED_ORIGINS environment variable
# (comma-separated list). An empty/unset variable means no origin is allowed,
# which is the most restrictive and safe default.
# Example: CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
# ---------------------------------------------------------------------------
_raw_origins = os.environ.get("CORS_ALLOWED_ORIGINS", "")
ALLOWED_ORIGINS = [o.strip() for o in _raw_origins.split(",") if o.strip()]

app = Flask(__name__)

# ---------------------------------------------------------------------------
# Seed data helpers
# ---------------------------------------------------------------------------

SEED_USERS = [
    {"id": 1, "username": "alice", "role": "admin"},
    {"id": 2, "username": "bob", "role": "viewer"},
]

SEED_REPOS = [
    {"id": 1, "name": "neuralforge", "owner": "alice"},
    {"id": 2, "name": "demo-app", "owner": "bob"},
]

SEED_FINDINGS = [
    {"id": 1, "severity": "HIGH", "rule": "sast_001", "file": "main.py"},
    {"id": 2, "severity": "MEDIUM", "rule": "sast_013", "file": "api.py"},
]


def _seed_users(db):
    """Insert seed users into *db*."""
    for user in SEED_USERS:
        db.users.insert(user)
    logger.info("Seeded %d users", len(SEED_USERS))


def _seed_repos(db):
    """Insert seed repositories into *db*."""
    for repo in SEED_REPOS:
        db.repos.insert(repo)
    logger.info("Seeded %d repos", len(SEED_REPOS))


def _seed_findings(db):
    """Insert seed security findings into *db*."""
    for finding in SEED_FINDINGS:
        db.findings.insert(finding)
    logger.info("Seeded %d findings", len(SEED_FINDINGS))


def run_seed(db):
    """Run all seed operations against *db*."""
    _seed_users(db)
    _seed_repos(db)
    _seed_findings(db)
    logger.info("Demo seed complete")


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.route("/seed", methods=["POST"])
def seed_endpoint():
    """Trigger demo data seeding (internal use only)."""
    token = request.headers.get("X-Seed-Token", "")
    expected = os.environ.get("SEED_TOKEN", "")
    if not expected or token != expected:
        return jsonify({"error": "unauthorized"}), 401
    return jsonify({"status": "ok", "seeded": True}), 200


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"}), 200


# ---------------------------------------------------------------------------
# Application factory / entrypoint
# ---------------------------------------------------------------------------

def create_app():
    # line 474 — restrict CORS to trusted origins (no wildcard)
    CORS(app, origins=ALLOWED_ORIGINS)
    return app


if __name__ == "__main__":
    application = create_app()
    port = int(os.environ.get("PORT", 5000))
    application.run(host="0.0.0.0", port=port)
