"""
api/routers/agent — backward-compat re-export shim.

Formerly api/routers/agent.py (1981 lines, 17 exports).
Sub-routers are combined into a single `router` object via include_router.
"""
from fastapi import APIRouter

from api.routers.agent.core import router as core_router
from api.routers.agent.runs import router as runs_router
from api.routers.agent.logs import router as logs_router
from api.routers.agent.config import router as config_router
from api.routers.agent.metrics import router as metrics_router

router = APIRouter()
router.include_router(core_router)
router.include_router(runs_router, prefix="/runs")
router.include_router(logs_router, prefix="/logs")
router.include_router(config_router, prefix="/config")
router.include_router(metrics_router, prefix="/metrics")
