"""
Authentication routes — login, logout, token refresh.
"""
import logging
from typing import Any, Dict

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


def login(request_data: Dict[str, Any]) -> Dict[str, Any]:
    """Authenticate a user and issue a session token."""
    # ... (lines 1-151: credential validation) ...

    # line 152 — request_data may contain username/password; mask before logging
    logger.warning("Login failed for %s", mask_pii(request_data))

    return {"error": "invalid_credentials"}


def issue_token(user: Dict[str, Any]) -> str:
    """Issue a JWT for an authenticated user."""
    # ... (lines 154-181: token generation) ...

    # line 182 — user dict may contain email/name; mask before logging
    logger.info("Token issued for user %s", mask_pii(user))

    return "token-placeholder"


# ... remainder of auth route handlers ...
