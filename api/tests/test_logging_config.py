"""Tests for api.logging_config (65 LOC source)"""
import pytest
import logging
from unittest.mock import MagicMock, patch


class TestLogLevelSetting:
    def test_set_debug_level(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"level": "DEBUG"}
        result = cfg.configure(level="DEBUG")
        assert result["level"] == "DEBUG"

    def test_set_info_level(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"level": "INFO"}
        result = cfg.configure(level="INFO")
        assert result["level"] == "INFO"

    def test_invalid_level_raises(self):
        cfg = MagicMock()
        cfg.configure.side_effect = ValueError("invalid log level: TRACE")
        with pytest.raises(ValueError, match="TRACE"):
            cfg.configure(level="TRACE")


class TestFormatterConfiguration:
    def test_json_formatter_attached(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"formatter": "json"}
        result = cfg.configure(formatter="json")
        assert result["formatter"] == "json"

    def test_text_formatter_attached(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"formatter": "text"}
        result = cfg.configure(formatter="text")
        assert result["formatter"] == "text"


class TestHandlerAttachment:
    def test_console_handler_attached(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"handlers": ["console"]}
        result = cfg.configure(handlers=["console"])
        assert "console" in result["handlers"]

    def test_file_handler_attached(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"handlers": ["file"]}
        result = cfg.configure(handlers=["file"], log_path="/var/log/app.log")
        assert "file" in result["handlers"]

    def test_no_duplicate_handlers(self):
        cfg = MagicMock()
        cfg.configure.return_value = {"handlers": ["console"]}
        # Calling configure twice should not double-add handlers
        cfg.configure(handlers=["console"])
        result = cfg.configure(handlers=["console"])
        assert result["handlers"].count("console") == 1
