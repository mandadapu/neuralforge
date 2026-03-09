"""
Application state management.
"""
import logging
from typing import Any, Dict

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


class StateManager:
    """Manages in-memory and persisted application state."""

    def __init__(self) -> None:
        self._state: Dict[str, Any] = {}

    def update_user_state(self, user_id: str, user_data: Dict[str, Any]) -> None:
        """Update state for a given user."""
        # ... (lines 1-259: state merge logic) ...

        # line 260 — user data masked before logging
        logger.debug("Updated state for user %s: %s", user_id, mask_pii(user_data))

    # ... remainder of StateManager methods ...
