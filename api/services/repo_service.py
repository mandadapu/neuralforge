# api/services/repo_service.py
# Facade consolidating repository and Git operations.

import logging

from api.integrations.github import GitHubClient, get_installation_token
from api.utils.git import clone_repo, checkout_branch, create_pr
from api.models.repo import Repository, Branch, PullRequest
from api.utils.diff import generate_diff, apply_patch

_audit_log = logging.getLogger("audit")


class RepoService:
    """Facade for repository and Git operations.

    Provides a single import point for all repo/Git functionality used by
    routers and workers, reducing their intra-repo import fan-out.

    Raises ValueError at construction if installation_id is not a positive
    integer, preventing silent access-control bypass when auth middleware fails.
    """

    def __init__(self, installation_id: int) -> None:
        if not isinstance(installation_id, int) or installation_id <= 0:
            raise ValueError(
                f"installation_id must be a positive integer, got {installation_id!r}"
            )
        token = get_installation_token(installation_id)
        self._gh = GitHubClient(token=token)
        self._installation_id = installation_id

    def clone(self, repo: Repository, ref: str) -> str:
        """Clone *repo* at *ref* and return the local path."""
        _audit_log.info(
            "repo.clone installation_id=%d repo=%s ref=%s",
            self._installation_id,
            repo.full_name,
            ref,
        )
        return clone_repo(self._gh, repo, ref)

    def checkout(self, local_path: str, branch: str) -> None:
        _audit_log.info(
            "repo.checkout installation_id=%d branch=%s",
            self._installation_id,
            branch,
        )
        checkout_branch(local_path, branch)

    def open_pr(self, repo: Repository, head: Branch, base: Branch, title: str, body: str) -> PullRequest:
        _audit_log.info(
            "repo.open_pr installation_id=%d repo=%s head=%s base=%s",
            self._installation_id,
            repo.full_name,
            head.name,
            base.name,
        )
        return create_pr(self._gh, repo, head, base, title, body)

    def create_issue(self, repo: Repository, title: str, body: str):
        """Open a GitHub issue via the internal GitHub client."""
        _audit_log.info(
            "repo.create_issue installation_id=%d repo=%s title_length=%d",
            self._installation_id,
            repo.full_name,
            len(title),
        )
        return self._gh.create_issue(repo=repo, title=title, body=body)

    def diff(self, local_path: str, base_ref: str, head_ref: str) -> str:
        return generate_diff(local_path, base_ref, head_ref)

    def apply(self, local_path: str, patch: str) -> None:
        apply_patch(local_path, patch)
