"""
Shared CORS utilities.

Set the ALLOWED_ORIGINS environment variable to a comma-separated list of
trusted origins, e.g.:

    ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com

If the variable is unset or empty, the helper returns an empty list which
causes CORS middleware to deny all cross-origin requests (fail-closed).
"""
import os


def get_allowed_origins() -> list[str]:
    """Return the list of trusted CORS origins from the environment."""
    raw = os.environ.get("ALLOWED_ORIGINS", "")
    return [o.strip() for o in raw.split(",") if o.strip()]
