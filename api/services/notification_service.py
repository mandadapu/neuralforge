# api/services/notification_service.py
# Facade consolidating all outbound notification channels.

import logging
import re

from api.utils.slack import post_message, format_alert
from api.utils.email import send_email
from api.utils.webhooks import post_webhook

_audit_log = logging.getLogger("audit")

# Patterns for common PII/PHI that must not be forwarded to external channels.
# HIPAA §164.312(b), GDPR Art. 5(1)(c).
_PII_PATTERNS: list[tuple[re.Pattern, str]] = [
    (re.compile(r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b"), "[EMAIL]"),
    (re.compile(r"\b(?:\+?1[\s\-.]?)?\(?\d{3}\)?[\s\-.]?\d{3}[\s\-.]?\d{4}\b"), "[PHONE]"),
    # SSN
    (re.compile(r"\b\d{3}-\d{2}-\d{4}\b"), "[SSN]"),
    # IPv4 addresses (may be internal/sensitive)
    (re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"), "[IP]"),
    # Generic secrets: long hex strings (API keys, tokens, hashes)
    (re.compile(r"\b[0-9a-fA-F]{32,}\b"), "[TOKEN]"),
    # Bearer / Authorization header values
    (re.compile(r"(?i)(bearer\s+)\S+"), r"\1[REDACTED]"),
]


def _redact(text: str) -> str:
    """Remove PII/PHI patterns from *text* before external delivery."""
    for pattern, replacement in _PII_PATTERNS:
        text = pattern.sub(replacement, text)
    return text


class NotificationService:
    """Facade for all outbound notifications.

    Provides a single import point for Slack, email, and webhook delivery,
    reducing caller fan-out.

    All outbound text is passed through PII/PHI redaction before delivery
    (HIPAA §164.312(b), GDPR Art. 5(1)(c)).  Each send operation is recorded
    in the audit log (HIPAA §164.312(b), SOX §404, GDPR Art. 30).
    """

    def slack(self, channel: str, text: str, *, level: str = "info") -> None:
        safe = _redact(text)
        _audit_log.info(
            "notification.slack channel=%s level=%s text_length=%d",
            channel,
            level,
            len(safe),
        )
        payload = format_alert(safe, level=level)
        post_message(channel, payload)

    def email(self, to: str, subject: str, body: str) -> None:
        safe_subject = _redact(subject)
        safe_body = _redact(body)
        _audit_log.info(
            "notification.email to=%s subject_length=%d body_length=%d",
            _redact(to),
            len(safe_subject),
            len(safe_body),
        )
        send_email(to=to, subject=safe_subject, body=safe_body)

    def webhook(self, url: str, payload: dict) -> None:
        _audit_log.info("notification.webhook url=%s", url)
        post_webhook(url, payload)
