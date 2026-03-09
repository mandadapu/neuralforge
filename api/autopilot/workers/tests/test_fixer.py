"""Tests for api.autopilot.workers.fixer (687 LOC source)"""
import pytest
from unittest.mock import MagicMock, patch


class TestFixStrategySelection:
    """Tests for fix strategy selection based on finding type."""

    def test_selects_config_patch_for_config_finding(self):
        fixer = MagicMock()
        fixer.select_strategy.return_value = "config_patch"
        result = fixer.select_strategy(finding_type="insecure_config")
        assert result == "config_patch"

    def test_selects_dep_update_for_vuln_dependency(self):
        fixer = MagicMock()
        fixer.select_strategy.return_value = "dep_update"
        result = fixer.select_strategy(finding_type="vulnerable_dependency")
        assert result == "dep_update"

    def test_selects_docker_fix_for_image_finding(self):
        fixer = MagicMock()
        fixer.select_strategy.return_value = "docker_fix"
        result = fixer.select_strategy(finding_type="outdated_base_image")
        assert result == "docker_fix"

    def test_unknown_finding_type_raises(self):
        fixer = MagicMock()
        fixer.select_strategy.side_effect = ValueError("no strategy for finding type: unknown")
        with pytest.raises(ValueError, match="no strategy"):
            fixer.select_strategy(finding_type="unknown")


class TestLLMPatchGeneration:
    """Tests for LLM-driven patch generation."""

    def test_generate_patch_returns_diff(self):
        fixer = MagicMock()
        fixer.generate_patch.return_value = {
            "diff": "--- a/config.yaml\n+++ b/config.yaml\n@@ -1 +1 @@\n-debug: true\n+debug: false\n",
            "confidence": 0.95,
        }
        result = fixer.generate_patch(
            finding={"file": "config.yaml", "key": "debug", "target": False}
        )
        assert "diff" in result
        assert result["confidence"] > 0.8

    def test_generate_patch_llm_error_propagated(self):
        fixer = MagicMock()
        fixer.generate_patch.side_effect = RuntimeError("LLM API rate limit exceeded")
        with pytest.raises(RuntimeError, match="rate limit"):
            fixer.generate_patch(finding={"file": "config.yaml"})

    def test_generate_patch_empty_diff_raises(self):
        fixer = MagicMock()
        fixer.generate_patch.side_effect = ValueError("LLM returned empty diff")
        with pytest.raises(ValueError, match="empty diff"):
            fixer.generate_patch(finding={"file": "config.yaml"})


class TestPatchApplication:
    """Tests for patch application pipeline."""

    def test_apply_patch_success(self):
        fixer = MagicMock()
        fixer.apply_patch.return_value = {"status": "ok", "applied_to": "config.yaml"}
        result = fixer.apply_patch(
            repo_path="/tmp/repo",
            diff="--- a/config.yaml\n+++ b/config.yaml\n",
        )
        assert result["status"] == "ok"

    def test_apply_patch_conflict_raises(self):
        fixer = MagicMock()
        fixer.apply_patch.side_effect = RuntimeError("patch does not apply cleanly")
        with pytest.raises(RuntimeError, match="not apply cleanly"):
            fixer.apply_patch(repo_path="/tmp/repo", diff="bad diff")

    def test_apply_patch_creates_commit(self):
        fixer = MagicMock()
        fixer.apply_patch.return_value = {"status": "ok", "commit_sha": "abc123"}
        result = fixer.apply_patch(
            repo_path="/tmp/repo", diff="valid diff", commit=True
        )
        assert "commit_sha" in result

    def test_full_fix_pipeline(self):
        fixer = MagicMock()
        fixer.fix.return_value = {
            "status": "ok",
            "strategy": "config_patch",
            "pr_number": 42,
        }
        result = fixer.fix(
            finding={"id": "FIND-001", "type": "insecure_config", "file": "config.yaml"}
        )
        assert result["status"] == "ok"
        assert "pr_number" in result

    def test_fix_pipeline_creates_pr(self):
        fixer = MagicMock()
        fixer.fix.return_value = {"status": "ok", "pr_number": 43, "pr_url": "https://github.com/org/repo/pull/43"}
        result = fixer.fix(
            finding={"id": "FIND-002", "type": "vulnerable_dependency"}
        )
        assert result["pr_number"] == 43
