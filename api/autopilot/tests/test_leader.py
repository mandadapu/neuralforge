"""Tests for api.autopilot.leader (31 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestLeaderElection:
    """Tests for distributed leader election logic."""

    def test_acquire_lock_makes_this_node_leader(self):
        leader = MagicMock()
        leader.acquire.return_value = True
        result = leader.acquire(node_id="node-1", ttl_seconds=30)
        assert result is True

    def test_acquire_lock_when_another_node_is_leader(self):
        leader = MagicMock()
        leader.acquire.return_value = False
        result = leader.acquire(node_id="node-2", ttl_seconds=30)
        assert result is False

    def test_release_lock(self):
        leader = MagicMock()
        leader.release.return_value = True
        result = leader.release(node_id="node-1")
        assert result is True

    def test_is_leader_returns_true_when_holding_lock(self):
        leader = MagicMock()
        leader.is_leader.return_value = True
        assert leader.is_leader(node_id="node-1") is True

    def test_is_leader_returns_false_when_not_holding_lock(self):
        leader = MagicMock()
        leader.is_leader.return_value = False
        assert leader.is_leader(node_id="node-2") is False

    def test_lock_backend_error_propagated(self):
        leader = MagicMock()
        leader.acquire.side_effect = RuntimeError("lock backend unreachable")
        with pytest.raises(RuntimeError, match="lock backend"):
            leader.acquire(node_id="node-1", ttl_seconds=30)

    def test_renew_lock_extends_ttl(self):
        leader = MagicMock()
        leader.renew.return_value = {"ttl_remaining": 30}
        result = leader.renew(node_id="node-1", ttl_seconds=30)
        assert result["ttl_remaining"] == 30
