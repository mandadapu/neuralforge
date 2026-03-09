"""
Email delivery service.
"""
import logging
from typing import Any, Dict, Optional

from api.utils.pii import mask_email, mask_pii

logger = logging.getLogger(__name__)


def send_email(recipient_email: str, subject: str, body: str) -> bool:
    """Send an email to a single recipient."""
    # ... (lines 1-61: SMTP setup) ...

    # line 62 — recipient email masked before logging
    logger.info("Sending email to %s", mask_email(recipient_email))

    # ... (delivery logic) ...
    return True


def send_templated_email(
    recipient_email: str,
    template_name: str,
    context: Dict[str, Any],
) -> bool:
    """Render a template and deliver the resulting email."""
    # ... (lines 63-90: template rendering) ...

    # line 91 — template context masked before logging
    logger.debug("Email context: %s", mask_pii(context))

    # ... (delivery logic) ...
    return True


# ---------------------------------------------------------------------------
# lines 93-552: bulk send helpers, retry logic, bounce handling …
# ---------------------------------------------------------------------------

def notify_user(user: Dict[str, Any], event: str) -> Optional[bool]:
    """Send a notification email for a lifecycle event."""
    try:
        # ... notification logic ...
        pass
    except Exception as exc:  # noqa: BLE001
        # line 553 — user dict masked before logging
        logger.error("Failed for user %s: %s", mask_pii(user), exc)
        return None
    return True
