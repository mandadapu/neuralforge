"""repos router package — composes sub-routers for backwards compatibility."""
from fastapi import APIRouter
from api.routers.repos.core import router as core_router
from api.routers.repos.integrations import router as integrations_router

router = APIRouter(prefix="/repos", tags=["repos"])
router.include_router(core_router)
router.include_router(integrations_router)
