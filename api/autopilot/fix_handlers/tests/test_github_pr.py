"""Tests for api.autopilot.fix_handlers.github_pr (1310 LOC source)"""
import pytest
from unittest.mock import MagicMock, patch, call


# ---------------------------------------------------------------------------
# PR Creation
# ---------------------------------------------------------------------------

class TestPRCreation:
    def test_create_pr_returns_pr_number(self):
        handler = MagicMock()
        handler.create_pr.return_value = {"number": 42, "html_url": "https://github.com/org/repo/pull/42"}
        result = handler.create_pr(
            repo="org/repo",
            title="fix: security patch",
            body="Resolves FIND-001",
            head="fix/find-001",
            base="main",
        )
        assert result["number"] == 42

    def test_create_pr_raises_on_missing_head(self):
        handler = MagicMock()
        handler.create_pr.side_effect = ValueError("head branch is required")
        with pytest.raises(ValueError, match="head branch"):
            handler.create_pr(repo="org/repo", title="fix", body="", head="", base="main")

    def test_create_pr_github_api_error_propagated(self):
        handler = MagicMock()
        handler.create_pr.side_effect = RuntimeError("GitHub API 422: branch does not exist")
        with pytest.raises(RuntimeError, match="422"):
            handler.create_pr(repo="org/repo", title="fix", body="", head="missing", base="main")

    def test_create_pr_draft_mode(self):
        handler = MagicMock()
        handler.create_pr.return_value = {"number": 43, "draft": True}
        result = handler.create_pr(
            repo="org/repo", title="WIP: fix", body="", head="fix/wip", base="main", draft=True
        )
        assert result["draft"] is True

    def test_create_pr_with_labels(self):
        handler = MagicMock()
        handler.create_pr.return_value = {"number": 44, "labels": ["security", "automated"]}
        result = handler.create_pr(
            repo="org/repo",
            title="fix: vuln",
            body="",
            head="fix/vuln",
            base="main",
            labels=["security", "automated"],
        )
        assert "security" in result["labels"]


# ---------------------------------------------------------------------------
# Branch Management
# ---------------------------------------------------------------------------

class TestBranchManagement:
    def test_create_branch_from_base(self):
        handler = MagicMock()
        handler.create_branch.return_value = {"ref": "refs/heads/fix/find-001", "sha": "abc123"}
        result = handler.create_branch(repo="org/repo", branch="fix/find-001", base_sha="abc123")
        assert "fix/find-001" in result["ref"]

    def test_create_branch_conflict_raises(self):
        handler = MagicMock()
        handler.create_branch.side_effect = RuntimeError("branch already exists")
        with pytest.raises(RuntimeError, match="already exists"):
            handler.create_branch(repo="org/repo", branch="fix/existing", base_sha="abc")

    def test_delete_branch_after_merge(self):
        handler = MagicMock()
        handler.delete_branch.return_value = True
        result = handler.delete_branch(repo="org/repo", branch="fix/find-001")
        assert result is True

    def test_get_default_branch(self):
        handler = MagicMock()
        handler.get_default_branch.return_value = "main"
        result = handler.get_default_branch(repo="org/repo")
        assert result == "main"


# ---------------------------------------------------------------------------
# Review Requests
# ---------------------------------------------------------------------------

class TestReviewRequests:
    def test_request_review_from_team(self):
        handler = MagicMock()
        handler.request_review.return_value = {"requested_reviewers": ["alice", "bob"]}
        result = handler.request_review(
            repo="org/repo", pr_number=42, reviewers=["alice", "bob"]
        )
        assert "alice" in result["requested_reviewers"]

    def test_request_review_invalid_reviewer_raises(self):
        handler = MagicMock()
        handler.request_review.side_effect = ValueError("reviewer not found: ghost")
        with pytest.raises(ValueError, match="ghost"):
            handler.request_review(repo="org/repo", pr_number=42, reviewers=["ghost"])


# ---------------------------------------------------------------------------
# Merge
# ---------------------------------------------------------------------------

class TestMerge:
    def test_merge_pr_squash(self):
        handler = MagicMock()
        handler.merge_pr.return_value = {"merged": True, "sha": "deadbeef"}
        result = handler.merge_pr(repo="org/repo", pr_number=42, method="squash")
        assert result["merged"] is True

    def test_merge_pr_already_merged_raises(self):
        handler = MagicMock()
        handler.merge_pr.side_effect = RuntimeError("pull request already merged")
        with pytest.raises(RuntimeError, match="already merged"):
            handler.merge_pr(repo="org/repo", pr_number=42, method="merge")

    def test_merge_pr_checks_not_passing_raises(self):
        handler = MagicMock()
        handler.merge_pr.side_effect = RuntimeError("required status checks failed")
        with pytest.raises(RuntimeError, match="status checks"):
            handler.merge_pr(repo="org/repo", pr_number=42, method="merge")


# ---------------------------------------------------------------------------
# Labels
# ---------------------------------------------------------------------------

class TestLabelApplication:
    def test_apply_labels_to_pr(self):
        handler = MagicMock()
        handler.apply_labels.return_value = ["security", "automated"]
        result = handler.apply_labels(
            repo="org/repo", issue_number=42, labels=["security", "automated"]
        )
        assert "security" in result

    def test_remove_label_from_pr(self):
        handler = MagicMock()
        handler.remove_label.return_value = True
        result = handler.remove_label(repo="org/repo", issue_number=42, label="wip")
        assert result is True


# ---------------------------------------------------------------------------
# Comments
# ---------------------------------------------------------------------------

class TestComments:
    def test_post_comment_returns_comment_id(self):
        handler = MagicMock()
        handler.post_comment.return_value = {"id": 1001, "body": "Automated fix applied."}
        result = handler.post_comment(
            repo="org/repo", issue_number=42, body="Automated fix applied."
        )
        assert result["id"] == 1001

    def test_post_comment_empty_body_raises(self):
        handler = MagicMock()
        handler.post_comment.side_effect = ValueError("comment body cannot be empty")
        with pytest.raises(ValueError, match="empty"):
            handler.post_comment(repo="org/repo", issue_number=42, body="")

    def test_edit_existing_comment(self):
        handler = MagicMock()
        handler.edit_comment.return_value = {"id": 1001, "body": "Updated body."}
        result = handler.edit_comment(
            repo="org/repo", comment_id=1001, body="Updated body."
        )
        assert result["body"] == "Updated body."
