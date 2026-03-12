# api/autopilot/workers/context.py
# Shared dependency container for autopilot workers.
# Workers import WorkerContext instead of each individual service module,
# reducing per-worker fan-out from ~20 imports to a handful.

from dataclasses import dataclass, field

from api.services.cloud_service import CloudService
from api.services.repo_service import RepoService
from api.services.scan_service import ScanService
from api.services.notification_service import NotificationService


@dataclass
class WorkerContext:
    """Dependency container passed to every autopilot worker.

    Each worker receives a fully constructed WorkerContext so it can call
    cloud, repo, scan, and notification operations without importing those
    service modules directly.
    """

    cloud: CloudService
    repo: RepoService
    scan: ScanService
    notify: NotificationService

    @classmethod
    def build(cls, installation_id: int) -> "WorkerContext":
        """Construct a WorkerContext wired with real service instances."""
        return cls(
            cloud=CloudService(),
            repo=RepoService(installation_id=installation_id),
            scan=ScanService(),
            notify=NotificationService(),
        )
