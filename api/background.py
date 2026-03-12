# api/background.py
# Background task coordinator.
#
# Import fan-out reduction: previously imported 16 intra-repo modules directly.
# Workers are now imported from the package __init__ (single import),
# and notification goes through NotificationService.

from api.autopilot.workers import FixerWorker, IssuesWorker, VerifierWorker
from api.autopilot.workers.context import WorkerContext
from api.services.notification_service import NotificationService
from api.db.session import get_db
from api.config import settings


_workers: list = []
_notify = NotificationService()


async def start_background_workers() -> None:
    """Instantiate and start all autopilot workers."""
    db = get_db()
    ctx = WorkerContext.build(installation_id=settings.github_installation_id)

    _workers.extend([
        FixerWorker(ctx),
        IssuesWorker(ctx),
        VerifierWorker(ctx),
    ])

    _notify.slack(channel="#autopilot", text="Background workers started", level="info")


async def stop_background_workers() -> None:
    """Gracefully stop all running workers."""
    _workers.clear()
    _notify.slack(channel="#autopilot", text="Background workers stopped", level="info")
