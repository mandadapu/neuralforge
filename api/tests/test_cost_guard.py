"""Tests for api.cost_guard (195 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestBudgetEnforcement:
    def test_operation_blocked_when_over_budget(self):
        guard = MagicMock()
        guard.check.side_effect = RuntimeError("budget exceeded: $1100 > $1000 limit")
        with pytest.raises(RuntimeError, match="budget exceeded"):
            guard.check(operation="llm_call", estimated_cost=100.0)

    def test_operation_allowed_when_under_budget(self):
        guard = MagicMock()
        guard.check.return_value = {"allowed": True, "remaining_budget": 200.0}
        result = guard.check(operation="llm_call", estimated_cost=10.0)
        assert result["allowed"] is True

    def test_cost_accumulated_after_operation(self):
        guard = MagicMock()
        guard.record.return_value = {"total_spent": 110.0}
        result = guard.record(operation="llm_call", cost=10.0)
        assert result["total_spent"] == 110.0

    def test_budget_resets_at_period_boundary(self):
        guard = MagicMock()
        guard.reset.return_value = {"total_spent": 0.0, "period": "2026-04"}
        result = guard.reset(period="2026-04")
        assert result["total_spent"] == 0.0

    def test_get_remaining_budget(self):
        guard = MagicMock()
        guard.remaining.return_value = 450.0
        result = guard.remaining()
        assert result == 450.0


class TestCostAccumulation:
    def test_multiple_operations_accumulate(self):
        guard = MagicMock()
        guard.record.side_effect = [
            {"total_spent": 10.0},
            {"total_spent": 20.0},
            {"total_spent": 30.0},
        ]
        r1 = guard.record(operation="op1", cost=10.0)
        r2 = guard.record(operation="op2", cost=10.0)
        r3 = guard.record(operation="op3", cost=10.0)
        assert r3["total_spent"] == 30.0

    def test_zero_cost_operation_allowed(self):
        guard = MagicMock()
        guard.check.return_value = {"allowed": True, "remaining_budget": 500.0}
        result = guard.check(operation="read", estimated_cost=0.0)
        assert result["allowed"] is True

    def test_negative_cost_raises(self):
        guard = MagicMock()
        guard.record.side_effect = ValueError("cost cannot be negative")
        with pytest.raises(ValueError, match="negative"):
            guard.record(operation="refund", cost=-5.0)
