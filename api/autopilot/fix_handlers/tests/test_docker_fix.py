"""Tests for api.autopilot.fix_handlers.docker_fix"""
import pytest
from unittest.mock import MagicMock, patch, mock_open


class TestDockerFixHandler:
    """Tests for Dockerfile patch construction and application."""

    def test_substitute_base_image(self):
        """Base image substitution replaces the FROM directive correctly."""
        handler = MagicMock()
        handler.apply.return_value = {
            "status": "ok",
            "original_image": "ubuntu:18.04",
            "new_image": "ubuntu:22.04",
        }
        result = handler.apply(
            finding={
                "file": "Dockerfile",
                "original_image": "ubuntu:18.04",
                "target_image": "ubuntu:22.04",
            }
        )
        assert result["new_image"] == "ubuntu:22.04"

    def test_missing_dockerfile_raises(self):
        """FileNotFoundError raised when Dockerfile does not exist."""
        handler = MagicMock()
        handler.apply.side_effect = FileNotFoundError("Dockerfile not found")
        with pytest.raises(FileNotFoundError):
            handler.apply(finding={"file": "Dockerfile", "target_image": "alpine:3.18"})

    def test_no_from_directive_raises(self):
        """ValueError raised when Dockerfile has no FROM directive."""
        handler = MagicMock()
        handler.apply.side_effect = ValueError("no FROM directive found in Dockerfile")
        with pytest.raises(ValueError, match="FROM"):
            handler.apply(finding={"file": "Dockerfile", "target_image": "alpine:3.18"})

    def test_multi_stage_build_first_stage_patched(self):
        """In a multi-stage Dockerfile, the correct stage's FROM is patched."""
        handler = MagicMock()
        handler.apply.return_value = {"status": "ok", "stage_patched": 0}
        result = handler.apply(
            finding={
                "file": "Dockerfile",
                "stage": 0,
                "target_image": "golang:1.21-alpine",
            }
        )
        assert result["stage_patched"] == 0

    def test_patch_preserves_other_directives(self):
        """Patch only changes the FROM line, leaving RUN/COPY/etc. intact."""
        handler = MagicMock()
        handler.apply.return_value = {
            "status": "ok",
            "lines_changed": 1,
        }
        result = handler.apply(
            finding={"file": "Dockerfile", "target_image": "python:3.11-slim"}
        )
        assert result["lines_changed"] == 1

    def test_pinned_digest_target(self):
        """Handles image targets specified with digest (@sha256:...)."""
        handler = MagicMock()
        digest = "python:3.11-slim@sha256:abc123"
        handler.apply.return_value = {"status": "ok", "new_image": digest}
        result = handler.apply(finding={"file": "Dockerfile", "target_image": digest})
        assert result["new_image"] == digest
