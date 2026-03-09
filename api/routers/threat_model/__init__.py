"""threat_model router package — composes sub-routers for backwards compatibility."""
from fastapi import APIRouter
from api.routers.threat_model.core import router as core_router
from api.routers.threat_model.threats import router as threats_router
from api.routers.threat_model.mitigations import router as mitigations_router

router = APIRouter(prefix="/threat-model", tags=["threat-model"])
router.include_router(core_router)
router.include_router(threats_router)
router.include_router(mitigations_router)
