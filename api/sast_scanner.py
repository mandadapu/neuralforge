"""SAST scanner — static analysis of source repositories."""

from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware

from api.config import get_allowed_origins

app = FastAPI(title="SAST Scanner API")

# sast_013 fix (line 85): replaced allow_origins=["*"] with
# get_allowed_origins() to restrict cross-origin access to trusted domains.
app.add_middleware(
    CORSMiddleware,
    allow_origins=get_allowed_origins(),   # sast_013 fix — was ["*"]
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type"],
)


# ---------------------------------------------------------------------------
# Sub-application for the results API (mounted at /results).
# sast_013 fix (line 320): the sub-app also previously used allow_origins=["*"].
# ---------------------------------------------------------------------------

results_app = FastAPI(title="SAST Results API")

results_app.add_middleware(
    CORSMiddleware,
    allow_origins=get_allowed_origins(),   # sast_013 fix — was ["*"]
    allow_credentials=True,
    allow_methods=["GET", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type"],
)


@results_app.get("/{job_id}")
async def get_results(job_id: str) -> dict:
    """Return scan results for the given job ID."""
    return {"job_id": job_id, "findings": []}


app.mount("/results", results_app)


@app.post("/scan")
async def start_scan(repo_url: str) -> dict:
    """Trigger a SAST scan on *repo_url*."""
    return {"repo": repo_url, "status": "queued"}
