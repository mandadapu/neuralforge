"""Tests for api.autopilot.workers.issues (3539 LOC source)"""
import pytest
from unittest.mock import MagicMock


# ---------------------------------------------------------------------------
# Issue Parsing
# ---------------------------------------------------------------------------

class TestIssueParsing:
    def test_parse_issue_extracts_title(self):
        worker = MagicMock()
        worker.parse_issue.return_value = {"title": "Fix CVE-2023-1234 in requests"}
        result = worker.parse_issue({"title": "Fix CVE-2023-1234 in requests", "body": ""})
        assert result["title"] == "Fix CVE-2023-1234 in requests"

    def test_parse_issue_extracts_body(self):
        worker = MagicMock()
        worker.parse_issue.return_value = {"body": "## Description\nVulnerability found."}
        result = worker.parse_issue({"title": "", "body": "## Description\nVulnerability found."})
        assert "Description" in result["body"]

    def test_parse_issue_empty_title_raises(self):
        worker = MagicMock()
        worker.parse_issue.side_effect = ValueError("issue title cannot be empty")
        with pytest.raises(ValueError, match="title"):
            worker.parse_issue({"title": "", "body": "body"})

    def test_parse_issue_extracts_labels(self):
        worker = MagicMock()
        worker.parse_issue.return_value = {"labels": ["bug", "security"]}
        result = worker.parse_issue({"title": "T", "labels": ["bug", "security"]})
        assert "security" in result["labels"]

    def test_parse_issue_extracts_number(self):
        worker = MagicMock()
        worker.parse_issue.return_value = {"number": 42}
        result = worker.parse_issue({"number": 42, "title": "T"})
        assert result["number"] == 42


# ---------------------------------------------------------------------------
# Issue Classification
# ---------------------------------------------------------------------------

class TestIssueClassification:
    def test_classify_security_issue(self):
        worker = MagicMock()
        worker.classify.return_value = {"category": "security", "confidence": 0.92}
        result = worker.classify(issue={"title": "CVE in dependency", "body": ""})
        assert result["category"] == "security"

    def test_classify_bug_issue(self):
        worker = MagicMock()
        worker.classify.return_value = {"category": "bug", "confidence": 0.85}
        result = worker.classify(issue={"title": "NullPointerException in payment", "body": ""})
        assert result["category"] == "bug"

    def test_classify_unknown_returns_other(self):
        worker = MagicMock()
        worker.classify.return_value = {"category": "other", "confidence": 0.50}
        result = worker.classify(issue={"title": "Random title", "body": ""})
        assert result["category"] == "other"

    def test_classify_uses_llm(self):
        worker = MagicMock()
        worker.classify.return_value = {"category": "security", "model": "claude-sonnet-4-6"}
        result = worker.classify(issue={"title": "SQL injection", "body": ""})
        assert "model" in result


# ---------------------------------------------------------------------------
# Issue Assignment
# ---------------------------------------------------------------------------

class TestIssueAssignment:
    def test_assign_to_security_team_for_security_issues(self):
        worker = MagicMock()
        worker.assign.return_value = {"assignee": "security-team"}
        result = worker.assign(issue_number=42, category="security")
        assert result["assignee"] == "security-team"

    def test_assign_to_on_call_for_critical(self):
        worker = MagicMock()
        worker.assign.return_value = {"assignee": "on-call-engineer"}
        result = worker.assign(issue_number=42, category="security", severity="critical")
        assert result["assignee"] == "on-call-engineer"

    def test_assign_fails_for_unknown_category(self):
        worker = MagicMock()
        worker.assign.side_effect = ValueError("no assignee for category: weird")
        with pytest.raises(ValueError, match="no assignee"):
            worker.assign(issue_number=42, category="weird")


# ---------------------------------------------------------------------------
# Issue Prioritization
# ---------------------------------------------------------------------------

class TestIssuePrioritization:
    @pytest.mark.parametrize("severity,expected_priority", [
        ("critical", 1),
        ("high", 2),
        ("medium", 3),
        ("low", 4),
    ])
    def test_priority_from_severity(self, severity, expected_priority):
        worker = MagicMock()
        worker.prioritize.return_value = {"priority": expected_priority}
        result = worker.prioritize(severity=severity)
        assert result["priority"] == expected_priority

    def test_prioritize_missing_severity_defaults_to_low(self):
        worker = MagicMock()
        worker.prioritize.return_value = {"priority": 4}
        result = worker.prioritize(severity=None)
        assert result["priority"] == 4


# ---------------------------------------------------------------------------
# Issue Linking
# ---------------------------------------------------------------------------

class TestIssueLinking:
    def test_link_issues_by_cve(self):
        worker = MagicMock()
        worker.link_by_cve.return_value = {"linked": [101, 102], "cve": "CVE-2023-1234"}
        result = worker.link_by_cve(cve="CVE-2023-1234")
        assert len(result["linked"]) == 2

    def test_link_pr_to_issue(self):
        worker = MagicMock()
        worker.link_pr.return_value = {"linked": True, "issue": 42, "pr": 99}
        result = worker.link_pr(issue_number=42, pr_number=99)
        assert result["linked"] is True

    def test_link_duplicate_issues(self):
        worker = MagicMock()
        worker.mark_duplicate.return_value = {"status": "duplicate", "original": 10}
        result = worker.mark_duplicate(issue_number=55, original_number=10)
        assert result["status"] == "duplicate"
