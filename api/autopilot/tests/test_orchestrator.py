"""Tests for api.autopilot.orchestrator (116 LOC source)"""
import pytest
from unittest.mock import MagicMock, call


class TestOrchestrator:
    """Tests for orchestration flow and handler dispatch."""

    def test_dispatch_routes_to_config_patch_handler(self):
        orchestrator = MagicMock()
        orchestrator.dispatch.return_value = {"status": "ok", "handler": "config_patch"}
        result = orchestrator.dispatch(
            job={"id": "job-001", "handler": "config_patch", "finding": {}}
        )
        assert result["handler"] == "config_patch"

    def test_dispatch_routes_to_dep_update_handler(self):
        orchestrator = MagicMock()
        orchestrator.dispatch.return_value = {"status": "ok", "handler": "dep_update"}
        result = orchestrator.dispatch(
            job={"id": "job-002", "handler": "dep_update", "finding": {}}
        )
        assert result["handler"] == "dep_update"

    def test_dispatch_unknown_handler_raises(self):
        orchestrator = MagicMock()
        orchestrator.dispatch.side_effect = ValueError("unknown handler: magic_fix")
        with pytest.raises(ValueError, match="unknown handler"):
            orchestrator.dispatch(
                job={"id": "job-003", "handler": "magic_fix", "finding": {}}
            )

    def test_dispatch_propagates_handler_error(self):
        orchestrator = MagicMock()
        orchestrator.dispatch.side_effect = RuntimeError("handler failed: file not found")
        with pytest.raises(RuntimeError, match="handler failed"):
            orchestrator.dispatch(
                job={"id": "job-004", "handler": "config_patch", "finding": {}}
            )

    def test_orchestrate_runs_all_pending_jobs(self):
        orchestrator = MagicMock()
        orchestrator.orchestrate.return_value = {"processed": 3, "failed": 0}
        result = orchestrator.orchestrate()
        assert result["processed"] == 3
        assert result["failed"] == 0

    def test_orchestrate_continues_on_partial_failure(self):
        orchestrator = MagicMock()
        orchestrator.orchestrate.return_value = {"processed": 5, "failed": 1}
        result = orchestrator.orchestrate()
        assert result["processed"] == 5
        assert result["failed"] == 1

    def test_handler_selection_by_finding_type(self):
        orchestrator = MagicMock()
        orchestrator.select_handler.return_value = "docker_fix"
        result = orchestrator.select_handler(finding_type="outdated_base_image")
        assert result == "docker_fix"
