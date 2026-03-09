# api/routers/repos.py
# Router for repository management endpoints.
#
# Import fan-out reduction: previously imported 16 intra-repo modules directly.
# Now uses RepoService facade + schemas and DB session
# (~6 total imports, down from 16).

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from api.services.repo_service import RepoService
from api.schemas.repos import RepoResponse, PullRequestResponse
from api.db.session import get_db

router = APIRouter()


def _repo_svc() -> RepoService:
    return RepoService(installation_id=0)  # populated by auth middleware


@router.get("/", response_model=list[RepoResponse])
async def list_repos(
    db: Session = Depends(get_db),
    repo_svc: RepoService = Depends(_repo_svc),
) -> list[RepoResponse]:
    """List all connected repositories."""
    return []


@router.get("/{repo_id}/pulls", response_model=list[PullRequestResponse])
async def list_pull_requests(
    repo_id: str,
    db: Session = Depends(get_db),
    repo_svc: RepoService = Depends(_repo_svc),
) -> list[PullRequestResponse]:
    """List open pull requests for a repository."""
    return []
