# api/middleware.py
# FastAPI middleware registration.
# Extracted from api/main.py to reduce main.py fan-out.

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from api.auth.middleware import AuthMiddleware
from api.utils.logging import RequestLoggingMiddleware


def register_middleware(app: FastAPI) -> None:
    """Attach all middleware to *app* in the correct order."""
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    app.add_middleware(AuthMiddleware)
    app.add_middleware(RequestLoggingMiddleware)
