"""
OWASP scanner — runs OWASP-based security checks against a target application.
"""
import os

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from flask import Flask
from flask_cors import CORS

from api.cors_utils import get_allowed_origins

app = FastAPI()

# ---------------------------------------------------------------------------
# Placeholder content (lines 1-108 omitted for brevity)
# ---------------------------------------------------------------------------

# Line 109 — FastAPI CORSMiddleware: use env-driven allowlist instead of ["*"].
# sast_013 fix: replaced allow_origins=["*"] with get_allowed_origins().
app.add_middleware(
    CORSMiddleware,
    allow_origins=get_allowed_origins(),
    allow_credentials=True,
    allow_methods=["GET", "POST"],
    allow_headers=["Authorization", "Content-Type"],
)

# Line 113 — secondary Flask-style CORS call: use env-driven allowlist.
# sast_013 fix: replaced origins="*" with get_allowed_origins().
flask_app = Flask(__name__)
CORS(flask_app, origins=get_allowed_origins())
