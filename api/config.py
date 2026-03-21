"""Shared configuration utilities for the API."""

import os


def get_allowed_origins() -> list[str]:
    """Return the list of trusted CORS origins from the environment.

    Set ALLOWED_ORIGINS as a comma-separated list of origins.
    Example: ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
    """
    raw = os.getenv("ALLOWED_ORIGINS", "")
    origins = [o.strip() for o in raw.split(",") if o.strip()]
    if not origins:
        raise RuntimeError(
            "ALLOWED_ORIGINS environment variable must be set to a non-empty "
            "comma-separated list of trusted origins."
        )
    return origins
