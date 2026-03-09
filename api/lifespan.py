# api/lifespan.py
# Application startup / shutdown lifecycle handlers.
# Extracted from api/main.py to reduce main.py fan-out.

from contextlib import asynccontextmanager

from fastapi import FastAPI

from api.background import start_background_workers, stop_background_workers
from api.db.session import init_db, close_db


@asynccontextmanager
async def lifespan_handler(app: FastAPI):
    """FastAPI lifespan context manager wiring startup and shutdown."""
    await init_db()
    await start_background_workers()
    try:
        yield
    finally:
        await stop_background_workers()
        await close_db()
