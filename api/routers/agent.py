# api/routers/agent.py
# Router for agent-related endpoints.
#
# Import fan-out reduction: previously imported 18 intra-repo modules directly.
# Now uses CloudService + RepoService facades, Pydantic schemas, and DB session.

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.orm import Session

from api.services.cloud_service import CloudService
from api.services.repo_service import RepoService
from api.schemas.agent import AgentRunRequest, AgentRunResponse
from api.db.session import get_db

router = APIRouter()


def _cloud_svc() -> CloudService:
    return CloudService()


def _repo_svc(request: Request) -> RepoService:
    """Construct RepoService from the installation_id injected by auth middleware.

    Auth middleware is expected to set request.state.installation_id.
    Raises HTTP 401 if the value is missing or invalid so that RepoService
    is never instantiated with installation_id=0 (access-control bypass risk).
    """
    installation_id = getattr(request.state, "installation_id", None)
    if not isinstance(installation_id, int) or installation_id <= 0:
        raise HTTPException(status_code=401, detail="Missing or invalid installation credentials")
    return RepoService(installation_id=installation_id)


@router.post("/run", response_model=AgentRunResponse)
async def run_agent(
    request: AgentRunRequest,
    db: Session = Depends(get_db),
    cloud: CloudService = Depends(_cloud_svc),
    repo: RepoService = Depends(_repo_svc),
) -> AgentRunResponse:
    """Trigger an agent run for a repository."""
    accounts = cloud.list_aws_accounts()
    # ... implementation delegates to service facades
    return AgentRunResponse(status="queued", accounts=accounts)
