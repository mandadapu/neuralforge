# api/services/repo_service.py
# Facade consolidating repository and Git operations.

from api.integrations.github import GitHubClient, get_installation_token
from api.utils.git import clone_repo, checkout_branch, create_pr
from api.models.repo import Repository, Branch, PullRequest
from api.utils.diff import generate_diff, apply_patch


class RepoService:
    """Facade for repository and Git operations.

    Provides a single import point for all repo/Git functionality used by
    routers and workers, reducing their intra-repo import fan-out.
    """

    def __init__(self, installation_id: int) -> None:
        token = get_installation_token(installation_id)
        self._gh = GitHubClient(token=token)

    def clone(self, repo: Repository, ref: str) -> str:
        """Clone *repo* at *ref* and return the local path."""
        return clone_repo(self._gh, repo, ref)

    def checkout(self, local_path: str, branch: str) -> None:
        checkout_branch(local_path, branch)

    def open_pr(self, repo: Repository, head: Branch, base: Branch, title: str, body: str) -> PullRequest:
        return create_pr(self._gh, repo, head, base, title, body)

    def diff(self, local_path: str, base_ref: str, head_ref: str) -> str:
        return generate_diff(local_path, base_ref, head_ref)

    def apply(self, local_path: str, patch: str) -> None:
        apply_patch(local_path, patch)
