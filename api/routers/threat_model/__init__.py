"""
api/routers/threat_model — backward-compat re-export shim.

Formerly api/routers/threat_model.py (826 lines, 33 exports).
"""
from fastapi import APIRouter

from api.routers.threat_model.core import router as core_router
from api.routers.threat_model.components import router as components_router
from api.routers.threat_model.reports import router as reports_router

router = APIRouter()
router.include_router(core_router)
router.include_router(components_router, prefix="/components")
router.include_router(reports_router, prefix="/reports")
