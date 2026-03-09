"""
api/routers/clouds — backward-compat re-export shim.

Formerly api/routers/clouds.py (629 lines, 25 exports).
"""
from fastapi import APIRouter

from api.routers.clouds.core import router as core_router
from api.routers.clouds.credentials import router as credentials_router
from api.routers.clouds.resources import router as resources_router

router = APIRouter()
router.include_router(core_router)
router.include_router(credentials_router, prefix="/credentials")
router.include_router(resources_router, prefix="/resources")
