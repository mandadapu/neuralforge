# api/services/notification_service.py
# Facade consolidating all outbound notification channels.

from api.utils.slack import post_message, format_alert
from api.utils.email import send_email
from api.utils.webhooks import post_webhook


class NotificationService:
    """Facade for all outbound notifications.

    Provides a single import point for Slack, email, and webhook delivery,
    reducing caller fan-out.
    """

    def slack(self, channel: str, text: str, *, level: str = "info") -> None:
        payload = format_alert(text, level=level)
        post_message(channel, payload)

    def email(self, to: str, subject: str, body: str) -> None:
        send_email(to=to, subject=subject, body=body)

    def webhook(self, url: str, payload: dict) -> None:
        post_webhook(url, payload)
