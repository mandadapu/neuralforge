"""settings router package — composes sub-routers for backwards compatibility."""
from fastapi import APIRouter
from api.routers.settings.general import router as general_router
from api.routers.settings.integrations import router as integrations_router
from api.routers.settings.notifications import router as notifications_router

router = APIRouter(prefix="/settings", tags=["settings"])
router.include_router(general_router)
router.include_router(integrations_router)
router.include_router(notifications_router)
