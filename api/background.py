"""
Background task processing module.
"""
import logging
from typing import Any, Dict

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


def process_task(task_id: str, user_data: Dict[str, Any], payload: Dict[str, Any]) -> None:
    """Process a background task for a user."""
    logger.info("Starting background task %s", task_id)

    # ... (lines 1-212: task setup and preprocessing) ...

    # line 213 — user_data logged with PII masked
    logger.info("Processing task for user %s", mask_pii(user_data))

    # line 215 — payload logged with PII masked
    logger.debug("Task payload: %s", mask_pii(payload))

    # ... (remainder of task processing) ...
    logger.info("Completed background task %s", task_id)
