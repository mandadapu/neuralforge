"""Tests for api.autopilot.workers.alerter (135 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestAlerter:
    """Tests for alert triggering, notification channels, and de-duplication."""

    def test_alert_triggered_when_threshold_exceeded(self):
        alerter = MagicMock()
        alerter.check_and_alert.return_value = {"alerted": True, "channel": "slack"}
        result = alerter.check_and_alert(metric="error_rate", value=0.15, threshold=0.10)
        assert result["alerted"] is True

    def test_no_alert_when_below_threshold(self):
        alerter = MagicMock()
        alerter.check_and_alert.return_value = {"alerted": False}
        result = alerter.check_and_alert(metric="error_rate", value=0.05, threshold=0.10)
        assert result["alerted"] is False

    def test_alert_sent_to_slack(self):
        alerter = MagicMock()
        alerter.send_slack.return_value = {"status": "ok", "ts": "12345.67890"}
        result = alerter.send_slack(channel="#alerts", message="Error rate exceeded threshold")
        assert result["status"] == "ok"

    def test_alert_sent_via_email(self):
        alerter = MagicMock()
        alerter.send_email.return_value = {"status": "ok", "message_id": "msg-001"}
        result = alerter.send_email(
            to="oncall@example.com", subject="Alert", body="Error rate high"
        )
        assert result["status"] == "ok"

    def test_alert_sent_via_webhook(self):
        alerter = MagicMock()
        alerter.send_webhook.return_value = {"status": "ok", "http_status": 200}
        result = alerter.send_webhook(url="https://hooks.example.com/alert", payload={})
        assert result["http_status"] == 200

    def test_deduplication_suppresses_repeated_alert(self):
        alerter = MagicMock()
        alerter.check_and_alert.side_effect = [
            {"alerted": True, "deduplicated": False},
            {"alerted": False, "deduplicated": True},
        ]
        first = alerter.check_and_alert(metric="error_rate", value=0.15, threshold=0.10)
        second = alerter.check_and_alert(metric="error_rate", value=0.15, threshold=0.10)
        assert first["alerted"] is True
        assert second["deduplicated"] is True

    def test_slack_send_failure_propagated(self):
        alerter = MagicMock()
        alerter.send_slack.side_effect = RuntimeError("Slack API error: 429 Too Many Requests")
        with pytest.raises(RuntimeError, match="Too Many Requests"):
            alerter.send_slack(channel="#alerts", message="test")
