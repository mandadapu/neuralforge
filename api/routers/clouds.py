# api/routers/clouds.py
# Router for cloud account and credential management endpoints.
#
# Import fan-out reduction: previously imported 24 intra-repo modules directly.
# Now uses CloudService facade + NotificationService, model, and DB session
# (~8 total imports, down from 24).

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from api.services.cloud_service import CloudService
from api.services.notification_service import NotificationService
from api.models.cloud import CloudAccount, CloudCredential
from api.schemas.clouds import (
    CloudAccountCreate,
    CloudAccountResponse,
    CloudCredentialCreate,
)
from api.db.session import get_db

router = APIRouter()


def _cloud_svc() -> CloudService:
    return CloudService()


def _notify_svc() -> NotificationService:
    return NotificationService()


@router.get("/accounts", response_model=list[CloudAccountResponse])
async def list_accounts(
    db: Session = Depends(get_db),
    cloud: CloudService = Depends(_cloud_svc),
) -> list[CloudAccountResponse]:
    """List all connected cloud accounts."""
    accounts = cloud.list_aws_accounts()
    accounts += cloud.list_gcp_projects()
    accounts += cloud.list_azure_subscriptions()
    return accounts


@router.post("/accounts", response_model=CloudAccountResponse)
async def add_account(
    body: CloudAccountCreate,
    db: Session = Depends(get_db),
    cloud: CloudService = Depends(_cloud_svc),
    notify: NotificationService = Depends(_notify_svc),
) -> CloudAccountResponse:
    """Connect a new cloud account."""
    credential = CloudCredential(provider=body.provider, secret=body.secret)
    valid = cloud.validate(credential)
    if valid:
        notify.slack(channel="#clouds", text=f"Cloud account {body.name} connected")
    return CloudAccountResponse(name=body.name, provider=body.provider, valid=valid)
