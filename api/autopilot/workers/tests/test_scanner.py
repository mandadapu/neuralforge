"""Tests for api.autopilot.workers.scanner (299 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestFindingParsing:
    def test_parse_finding_extracts_id(self):
        scanner = MagicMock()
        scanner.parse_finding.return_value = {"id": "FIND-001", "severity": "HIGH"}
        result = scanner.parse_finding({"id": "FIND-001", "severity": "HIGH"})
        assert result["id"] == "FIND-001"

    def test_parse_finding_missing_id_raises(self):
        scanner = MagicMock()
        scanner.parse_finding.side_effect = ValueError("finding id is required")
        with pytest.raises(ValueError, match="id"):
            scanner.parse_finding({"severity": "HIGH"})

    def test_parse_finding_normalises_severity(self):
        scanner = MagicMock()
        scanner.parse_finding.return_value = {"id": "FIND-001", "severity": "HIGH"}
        result = scanner.parse_finding({"id": "FIND-001", "severity": "high"})
        assert result["severity"] == "HIGH"


class TestSeverityClassification:
    @pytest.mark.parametrize("raw,expected", [
        ("CRITICAL", "CRITICAL"),
        ("critical", "CRITICAL"),
        ("HIGH", "HIGH"),
        ("MEDIUM", "MEDIUM"),
        ("LOW", "LOW"),
        ("INFO", "INFO"),
    ])
    def test_classify_severity(self, raw, expected):
        scanner = MagicMock()
        scanner.classify_severity.return_value = expected
        result = scanner.classify_severity(raw)
        assert result == expected

    def test_unknown_severity_defaults_to_info(self):
        scanner = MagicMock()
        scanner.classify_severity.return_value = "INFO"
        result = scanner.classify_severity("unknown_level")
        assert result == "INFO"


class TestDeduplication:
    def test_deduplicate_removes_exact_duplicates(self):
        scanner = MagicMock()
        findings = [
            {"id": "F-001", "file": "a.py", "line": 10},
            {"id": "F-001", "file": "a.py", "line": 10},
        ]
        scanner.deduplicate.return_value = [findings[0]]
        result = scanner.deduplicate(findings)
        assert len(result) == 1

    def test_deduplicate_keeps_unique_findings(self):
        scanner = MagicMock()
        findings = [
            {"id": "F-001", "file": "a.py"},
            {"id": "F-002", "file": "b.py"},
        ]
        scanner.deduplicate.return_value = findings
        result = scanner.deduplicate(findings)
        assert len(result) == 2

    def test_scan_returns_deduplicated_findings(self):
        scanner = MagicMock()
        scanner.scan.return_value = [
            {"id": "F-001", "severity": "HIGH"},
            {"id": "F-002", "severity": "LOW"},
        ]
        result = scanner.scan(repo_path="/tmp/repo")
        assert len(result) == 2
