"""
OWASP scanner — runs OWASP-based security checks against a target application.
"""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from flask import Flask
from flask_cors import CORS

from api.cors_utils import get_allowed_origins

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=get_allowed_origins(),
    allow_credentials=True,
    allow_methods=["GET", "POST"],
    allow_headers=["Authorization", "Content-Type"],
)

flask_app = Flask(__name__)
CORS(flask_app, origins=get_allowed_origins())
