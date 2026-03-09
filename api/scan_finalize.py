"""
Scan finalization module — persists results and triggers downstream actions.
"""
import logging
from typing import Any, Dict

from api.utils.pii import mask_pii

logger = logging.getLogger(__name__)


def finalize_scan(scan_id: str, patient: Dict[str, Any], results: Dict[str, Any]) -> None:
    """Persist scan results and notify the owning user/patient."""
    logger.info("Finalizing scan %s", scan_id)

    # ... (lines 1-239: result aggregation) ...

    # line 240 — patient identifier masked before logging
    logger.info("Scan %s finalized for patient %s", scan_id, mask_pii(patient))

    # ... (lines 241-464: downstream notifications, storage) ...

    # line 465 — user/patient identity masked before logging
    logger.debug("Finalization context for scan %s: %s", scan_id, mask_pii(patient))

    logger.info("Scan finalization complete: %s", scan_id)
