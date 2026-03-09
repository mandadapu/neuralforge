"""agent router package — composes sub-routers for backwards compatibility."""
from fastapi import APIRouter
from api.routers.agent.runs import router as runs_router
from api.routers.agent.tasks import router as tasks_router
from api.routers.agent.context import router as context_router
from api.routers.agent.config import router as config_router

router = APIRouter(prefix="/agents", tags=["agents"])
router.include_router(runs_router)
router.include_router(tasks_router)
router.include_router(context_router)
router.include_router(config_router)
