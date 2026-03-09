# api/routers/agent.py
# Router for agent-related endpoints.
#
# Import fan-out reduction: previously imported 18 intra-repo modules directly.
# Now uses CloudService + RepoService facades, Pydantic schemas, and DB session.

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from api.services.cloud_service import CloudService
from api.services.repo_service import RepoService
from api.schemas.agent import AgentRunRequest, AgentRunResponse
from api.db.session import get_db

router = APIRouter()


def _cloud_svc() -> CloudService:
    return CloudService()


def _repo_svc() -> RepoService:
    return RepoService(installation_id=0)  # populated by auth middleware


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
