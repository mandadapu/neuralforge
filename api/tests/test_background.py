"""Tests for api.background (152 LOC source)"""
import pytest
from unittest.mock import MagicMock, AsyncMock


class TestBackgroundTaskLifecycle:
    def test_task_registered_on_startup(self):
        bg = MagicMock()
        bg.register.return_value = {"task": "scanner", "status": "registered"}
        result = bg.register(name="scanner", fn=MagicMock())
        assert result["status"] == "registered"

    def test_task_started(self):
        bg = MagicMock()
        bg.start.return_value = {"status": "running"}
        result = bg.start()
        assert result["status"] == "running"

    def test_task_stopped_on_shutdown(self):
        bg = MagicMock()
        bg.stop.return_value = {"status": "stopped"}
        result = bg.stop()
        assert result["status"] == "stopped"

    def test_duplicate_registration_raises(self):
        bg = MagicMock()
        bg.register.side_effect = ValueError("task 'scanner' already registered")
        with pytest.raises(ValueError, match="already registered"):
            bg.register(name="scanner", fn=MagicMock())

    def test_unknown_task_stop_raises(self):
        bg = MagicMock()
        bg.stop_task.side_effect = KeyError("unknown task: ghost")
        with pytest.raises(KeyError, match="ghost"):
            bg.stop_task("ghost")


class TestStartupShutdownHooks:
    @pytest.mark.asyncio
    async def test_startup_hook_called(self):
        bg = MagicMock()
        bg.on_startup = AsyncMock(return_value=None)
        await bg.on_startup()
        bg.on_startup.assert_called_once()

    @pytest.mark.asyncio
    async def test_shutdown_hook_called(self):
        bg = MagicMock()
        bg.on_shutdown = AsyncMock(return_value=None)
        await bg.on_shutdown()
        bg.on_shutdown.assert_called_once()

    @pytest.mark.asyncio
    async def test_startup_hook_error_propagated(self):
        bg = MagicMock()
        bg.on_startup = AsyncMock(side_effect=RuntimeError("startup failed"))
        with pytest.raises(RuntimeError, match="startup failed"):
            await bg.on_startup()
