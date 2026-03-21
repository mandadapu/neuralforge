"""OWASP Top-10 scanner — runs automated checks against a target URL."""

from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware

from api.config import get_allowed_origins

app = FastAPI(title="OWASP Scanner API")


def configure_cors(application: FastAPI) -> None:
    """Attach CORS middleware with a restricted origin allowlist.

    sast_013 fix (line 109): replaced allow_origins=["*"] with
    get_allowed_origins() so only explicitly trusted domains are permitted.
    """
    application.add_middleware(
        CORSMiddleware,
        allow_origins=get_allowed_origins(),   # sast_013 fix — was ["*"]
        allow_credentials=True,
        allow_methods=["GET", "POST", "OPTIONS"],
        allow_headers=["Authorization", "Content-Type"],
    )


def set_cors_header(request: Request, response: Response) -> None:
    """Set the CORS origin response header for manual route handlers.

    sast_013 fix (line 113): replaced unconditional wildcard header with an
    allowlist check so browsers reject cross-origin requests from unknown
    origins.
    """
    allowed = get_allowed_origins()            # sast_013 fix — was ["*"]
    origin = request.headers.get("Origin", "")
    if origin in allowed:
        response.headers["Access-Control-Allow-Origin"] = origin


configure_cors(app)


@app.get("/scan")
async def run_owasp_scan(target_url: str) -> dict:
    """Initiate an OWASP Top-10 scan against *target_url*."""
    return {"target": target_url, "status": "queued"}
