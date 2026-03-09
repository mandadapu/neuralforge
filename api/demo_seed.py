"""Demo seed data loader and application entry point."""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from api.config import get_allowed_origins

app = FastAPI(title="Demo Seed API")


def seed_demo_data() -> None:
    """Seed the database with demo records."""
    # Implementation omitted — seeding logic lives here.
    pass


def create_app() -> FastAPI:
    """Construct and configure the FastAPI application."""
    # sast_013 fix: replaced allow_origins=["*"] with get_allowed_origins().
    # Origins are read from the ALLOWED_ORIGINS environment variable so that
    # each deployment controls its own allowlist without touching source code.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=get_allowed_origins(),
        allow_credentials=True,
        allow_methods=["GET", "POST", "PUT", "DELETE", "OPTIONS"],
        allow_headers=["*"],
    )
    return app
