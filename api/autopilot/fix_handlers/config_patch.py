"""Config patch handler — generates patched configuration fragments."""

from api.config import get_allowed_origins


def build_cors_config() -> dict:
    """Return a CORS configuration dict suitable for middleware registration.

    sast_013 fix (line 24): the previous implementation used an unrestricted
    origin list which allowed any origin.  The fix replaces it with the
    runtime-resolved allowlist from the ALLOWED_ORIGINS environment variable.
    """
    return {
        "allow_origins": get_allowed_origins(),  # sast_013 fix: restricted origins
        "allow_credentials": True,
        "allow_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
        "allow_headers": ["Authorization", "Content-Type"],
    }


def patch_cors_settings(config: dict) -> dict:
    """Apply CORS security patch to an existing configuration dictionary.

    Merges the secure CORS settings from build_cors_config into config,
    overwriting any existing CORS keys.
    """
    config.update(build_cors_config())
    return config
