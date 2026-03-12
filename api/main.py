# api/main.py
# Application entry point.
#
# Import fan-out reduction: previously imported 28 intra-repo modules directly.
# Startup/lifecycle logic moved to api/lifespan.py (~10 modules).
# Middleware setup moved to api/middleware.py (~4 modules).
# main.py now imports only routers + the two lifecycle modules (~12 total).

from fastapi import FastAPI

from api.lifespan import lifespan_handler
from api.middleware import register_middleware
from api.routers import agent, autopilot, clouds, pentests, repos

app = FastAPI(title="NeuralWarden API", lifespan=lifespan_handler)

register_middleware(app)

app.include_router(agent.router, prefix="/agent", tags=["agent"])
app.include_router(autopilot.router, prefix="/autopilot", tags=["autopilot"])
app.include_router(clouds.router, prefix="/clouds", tags=["clouds"])
app.include_router(pentests.router, prefix="/pentests", tags=["pentests"])
app.include_router(repos.router, prefix="/repos", tags=["repos"])
