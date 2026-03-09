"""
User database access layer.
"""
import logging
from typing import Any, Dict, Optional

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


class UserDatabase:
    """Handles all user record persistence operations."""

    def __init__(self, connection_string: str) -> None:
        self._conn_str = connection_string

    def upsert_user(self, record: Dict[str, Any]) -> Optional[str]:
        """Insert or update a user record; return the user ID on success."""
        # ... (lines 1-579: DB upsert logic) ...

        # line 580 — DB row dict masked before logging so PII fields
        # (email, name, dob, etc.) are never written to the log stream
        logger.info("Upserted user record: %s", mask_pii(record))

        return record.get("id")

    # ... remainder of UserDatabase methods ...
