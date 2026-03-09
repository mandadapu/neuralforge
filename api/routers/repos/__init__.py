"""
api/routers/repos — backward-compat re-export shim.

Formerly api/routers/repos.py (547 lines, 26 exports).
"""
from fastapi import APIRouter

from api.routers.repos.core import router as core_router
from api.routers.repos.scans import router as scans_router
from api.routers.repos.webhooks import router as webhooks_router

router = APIRouter()
router.include_router(core_router)
router.include_router(scans_router, prefix="/scans")
router.include_router(webhooks_router, prefix="/webhooks")
