"""clouds router package — composes sub-routers for backwards compatibility."""
from fastapi import APIRouter
from api.routers.clouds.providers import router as providers_router
from api.routers.clouds.assets import router as assets_router

router = APIRouter(prefix="/clouds", tags=["clouds"])
router.include_router(providers_router)
router.include_router(assets_router)
