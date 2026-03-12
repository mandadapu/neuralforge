"""
config_patch.py — Autopilot fix handler for CORS configuration patches.

This handler is invoked by the autopilot pipeline to generate a patched
service configuration that replaces wildcard CORS origins with a
restricted, environment-variable-driven allowlist.
"""
import os
import copy
import logging
from typing import Any, Dict

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# CORS configuration — origins read from environment, no wildcard
# ---------------------------------------------------------------------------
_raw_origins = os.environ.get("CORS_ALLOWED_ORIGINS", "")
ALLOWED_ORIGINS = [o.strip() for o in _raw_origins.split(",") if o.strip()]

if not ALLOWED_ORIGINS:
    logger.warning(
        "CORS_ALLOWED_ORIGINS is not set or empty — all cross-origin requests will be rejected"
    )
else:
    logger.info("CORS allowed origins: %s", ALLOWED_ORIGINS)

# cors_config uses the env-var-driven list, not a wildcard
cors_config: Dict[str, Any] = {
    "origins": ALLOWED_ORIGINS,
    "allow_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    "allow_headers": ["Authorization", "Content-Type"],
    "allow_credentials": True,
}


# ---------------------------------------------------------------------------
# Patch helpers
# ---------------------------------------------------------------------------

def _patch_cors_in_dict(cfg: Dict[str, Any]) -> Dict[str, Any]:
    """
    Recursively walk *cfg* and replace any wildcard CORS origin entries
    (``"*"`` or ``["*"]``) with the restricted *ALLOWED_ORIGINS* list.
    """
    patched = copy.deepcopy(cfg)
    for key, value in patched.items():
        if key in ("origins", "allow_origins"):
            if value == "*" or value == ["*"]:
                logger.info("Patching wildcard CORS origin under key '%s'", key)
                patched[key] = ALLOWED_ORIGINS
        elif isinstance(value, dict):
            patched[key] = _patch_cors_in_dict(value)
        elif isinstance(value, list):
            patched[key] = [
                _patch_cors_in_dict(item) if isinstance(item, dict) else item
                for item in value
            ]
    return patched


def apply_cors_patch(service_config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Return a copy of *service_config* with all wildcard CORS origins replaced
    by the restricted ``ALLOWED_ORIGINS`` list derived from the environment.

    :param service_config: The original service configuration dict.
    :returns: Patched configuration dict.
    :raises ValueError: If the patched config still contains a wildcard origin.
    """
    patched = _patch_cors_in_dict(service_config)

    # Validate: no wildcard remaining — walk the structure directly
    _assert_no_wildcard_origin(patched)

    return patched


def _assert_no_wildcard_origin(cfg: Any, path: str = "") -> None:
    """Raise ValueError if any CORS origin key in *cfg* still holds a wildcard value."""
    if isinstance(cfg, dict):
        for key, value in cfg.items():
            key_path = f"{path}.{key}" if path else key
            if key in ("origins", "allow_origins"):
                if value == "*" or value == ["*"]:
                    raise ValueError(
                        f"Patched config still contains a wildcard CORS origin at '{key_path}'. "
                        "Set CORS_ALLOWED_ORIGINS to a comma-separated list of trusted origins."
                    )
            else:
                _assert_no_wildcard_origin(value, key_path)
    elif isinstance(cfg, list):
        for i, item in enumerate(cfg):
            _assert_no_wildcard_origin(item, f"{path}[{i}]")


def build_cors_middleware_config() -> Dict[str, Any]:
    """
    Build a CORS middleware configuration dict suitable for injection into
    FastAPI / Starlette or Flask-CORS.
    """
    return copy.deepcopy(cors_config)
