"""
api/routers/settings — backward-compat re-export shim.

Formerly api/routers/settings.py (598 lines, 40 exports).
"""
from fastapi import APIRouter

from api.routers.settings.user_settings import router as user_settings_router
from api.routers.settings.org_settings import router as org_settings_router
from api.routers.settings.integration_settings import router as integration_settings_router

router = APIRouter()
router.include_router(user_settings_router, prefix="/user")
router.include_router(org_settings_router, prefix="/org")
router.include_router(integration_settings_router, prefix="/integrations")
