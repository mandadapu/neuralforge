"""GitHub repository scanner — analyses repos for security findings."""

from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware

from api.config import get_allowed_origins

app = FastAPI(title="GitHub Scanner API")

# sast_013 fix: restrict CORS to the trusted origins list.
# Previously this set Access-Control-Allow-Origin unconditionally to any origin.
app.add_middleware(
    CORSMiddleware,
    allow_origins=get_allowed_origins(),
    allow_credentials=True,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type"],
)


def _cors_origin_header(request: Request, response: Response) -> None:
    """Reflect the request Origin only when it appears in the allowlist.

    sast_013 fix: replaced the unconditional wildcard origin header
    with an origin-allowlist check.
    """
    allowed = get_allowed_origins()
    origin = request.headers.get("Origin", "")
    if origin in allowed:
        response.headers["Access-Control-Allow-Origin"] = origin


@app.get("/scan/{owner}/{repo}")
async def scan_repository(owner: str, repo: str) -> dict:
    """Trigger a security scan on the given GitHub repository."""
    return {"owner": owner, "repo": repo, "status": "queued"}
