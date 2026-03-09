"""
Async worker — dequeues and processes jobs from the task queue.
"""
import logging
from typing import Any, Dict

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


class Worker:
    """Processes jobs pulled from the task queue."""

    def __init__(self, queue_url: str) -> None:
        self._queue_url = queue_url

    def handle_job(self, job: Dict[str, Any]) -> None:
        """Dispatch a single job to the appropriate handler."""
        # ... (lines 1-99: job deserialization and routing) ...

        # line 100 — job dict may contain user/patient context; mask before logging
        logger.info("Handling job: %s", mask_pii(job))

        # ... (remainder of job dispatch logic) ...

    # ... remainder of Worker methods ...
