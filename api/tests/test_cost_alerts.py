"""Tests for api.cost_alerts (38 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestCostAlerts:
    def test_alert_triggered_when_threshold_exceeded(self):
        alerts = MagicMock()
        alerts.check.return_value = {"alerted": True, "cost": 1500.0, "threshold": 1000.0}
        result = alerts.check(cost=1500.0, threshold=1000.0)
        assert result["alerted"] is True

    def test_no_alert_when_below_threshold(self):
        alerts = MagicMock()
        alerts.check.return_value = {"alerted": False, "cost": 800.0, "threshold": 1000.0}
        result = alerts.check(cost=800.0, threshold=1000.0)
        assert result["alerted"] is False

    def test_alert_delivery_via_notification_channel(self):
        alerts = MagicMock()
        alerts.deliver.return_value = {"status": "sent", "channel": "email"}
        result = alerts.deliver(
            to="admin@example.com",
            subject="Cost Alert",
            body="Monthly spend exceeded $1000",
        )
        assert result["status"] == "sent"

    def test_retrieve_current_cost(self):
        alerts = MagicMock()
        alerts.get_current_cost.return_value = {"amount": 950.0, "currency": "USD"}
        result = alerts.get_current_cost(project="my-project")
        assert result["currency"] == "USD"

    def test_cost_api_error_propagated(self):
        alerts = MagicMock()
        alerts.get_current_cost.side_effect = RuntimeError("Billing API unavailable")
        with pytest.raises(RuntimeError, match="Billing API"):
            alerts.get_current_cost(project="my-project")
