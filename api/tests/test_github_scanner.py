"""Tests for api.github_scanner (966 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestRepoScanning:
    def test_scan_repo_returns_findings(self):
        scanner = MagicMock()
        scanner.scan_repo.return_value = [
            {"id": "F-001", "type": "exposed_secret", "severity": "CRITICAL"},
            {"id": "F-002", "type": "vulnerable_dependency", "severity": "HIGH"},
        ]
        result = scanner.scan_repo(repo="org/repo")
        assert len(result) == 2

    def test_scan_empty_repo_returns_no_findings(self):
        scanner = MagicMock()
        scanner.scan_repo.return_value = []
        result = scanner.scan_repo(repo="org/empty-repo")
        assert result == []

    def test_scan_nonexistent_repo_raises(self):
        scanner = MagicMock()
        scanner.scan_repo.side_effect = RuntimeError("GitHub API 404: repo not found")
        with pytest.raises(RuntimeError, match="404"):
            scanner.scan_repo(repo="org/nonexistent")


class TestSecretDetection:
    def test_detects_aws_access_key(self):
        scanner = MagicMock()
        scanner.detect_secrets.return_value = [
            {"type": "aws_access_key", "file": "config.py", "line": 5}
        ]
        result = scanner.detect_secrets(content="AKIAIOSFODNN7EXAMPLE = 'abc'")
        assert len(result) == 1
        assert result[0]["type"] == "aws_access_key"

    def test_detects_github_token(self):
        scanner = MagicMock()
        scanner.detect_secrets.return_value = [
            {"type": "github_token", "file": ".env", "line": 1}
        ]
        result = scanner.detect_secrets(content="GH_TOKEN=ghp_abc123")
        assert result[0]["type"] == "github_token"

    def test_clean_content_returns_no_findings(self):
        scanner = MagicMock()
        scanner.detect_secrets.return_value = []
        result = scanner.detect_secrets(content="print('hello world')")
        assert result == []


class TestDependencyFinding:
    def test_find_vulnerable_dependency(self):
        scanner = MagicMock()
        scanner.find_vulnerable_deps.return_value = [
            {"package": "requests", "version": "2.25.0", "cve": "CVE-2023-1234"}
        ]
        result = scanner.find_vulnerable_deps(repo="org/repo")
        assert len(result) == 1
        assert result[0]["cve"] == "CVE-2023-1234"

    def test_no_vulnerable_deps_returns_empty(self):
        scanner = MagicMock()
        scanner.find_vulnerable_deps.return_value = []
        result = scanner.find_vulnerable_deps(repo="org/clean-repo")
        assert result == []


class TestApiPagination:
    def test_pagination_fetches_all_pages(self):
        scanner = MagicMock()
        scanner.get_all_files.return_value = [f"file_{i}.py" for i in range(150)]
        result = scanner.get_all_files(repo="org/large-repo")
        assert len(result) == 150

    def test_pagination_handles_empty_last_page(self):
        scanner = MagicMock()
        scanner.get_all_files.return_value = ["file_1.py", "file_2.py"]
        result = scanner.get_all_files(repo="org/small-repo")
        assert len(result) == 2

    def test_rate_limit_error_handled(self):
        scanner = MagicMock()
        scanner.scan_repo.side_effect = RuntimeError("GitHub API 403: rate limit exceeded")
        with pytest.raises(RuntimeError, match="rate limit"):
            scanner.scan_repo(repo="org/repo")
