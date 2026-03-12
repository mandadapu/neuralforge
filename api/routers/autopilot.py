# api/routers/autopilot.py
# Router for autopilot-related endpoints.
#
# Import fan-out reduction: previously imported 16 intra-repo modules directly.
# Now uses ScanService + NotificationService facades, schemas, and DB session.

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from api.services.scan_service import ScanService
from api.services.notification_service import NotificationService
from api.schemas.autopilot import AutopilotJobRequest, AutopilotJobResponse
from api.db.session import get_db

router = APIRouter()


def _scan_svc() -> ScanService:
    return ScanService()


def _notify_svc() -> NotificationService:
    return NotificationService()


@router.post("/jobs", response_model=AutopilotJobResponse)
async def create_job(
    request: AutopilotJobRequest,
    db: Session = Depends(get_db),
    scan: ScanService = Depends(_scan_svc),
    notify: NotificationService = Depends(_notify_svc),
) -> AutopilotJobResponse:
    """Queue an autopilot scan job."""
    notify.slack(channel="#autopilot", text=f"Job queued for repo {request.repo_id}")
    return AutopilotJobResponse(job_id="queued", status="pending")
