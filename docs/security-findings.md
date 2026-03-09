# Security Findings Log

## llm_010 — False Positive (job repo:e47b27f)

**Finding:** Restrict model access with authentication. Do not serve raw model files publicly.
**Reported file:** `api/schemas.py:562`
**Severity:** HIGH

**Status: FALSE POSITIVE — file does not exist in this repository.**

### Analysis

This repository is a pure Go project. There are no Python source files and no `api/` directory. The scanner job `repo:e47b27f` appears to have been run against a different codebase (likely a Python/Django service) and the finding was incorrectly attributed to this repository.

### HTTP Surface Review

The only externally reachable endpoints in this application are:

| Endpoint | Auth | Data exposed |
|---|---|---|
| `POST /webhooks/github` | HMAC signature verification | None — processes webhook events only |
| `GET /health` | None | `{"status":"ok"}` (static string) |

LLM provider API keys and model names are loaded from environment variables, kept only in memory, and are never written to any HTTP response. No endpoint proxies or exposes raw model calls externally.

### Conclusion

No code changes are required. The finding should be closed as a false positive. The autopilot scan job configuration should be audited to ensure scans are mapped to the correct repository.
