"""
scripts/seed_pinecone.py

Utility script for seeding a Pinecone vector store with document embeddings.
Content is validated and sanitized before ingestion to prevent RAG poisoning
and prompt injection attacks.
"""

import argparse
import hashlib
import logging
import re
import unicodedata
from typing import Optional

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

# ---------------------------------------------------------------------------
# Content sanitization
# ---------------------------------------------------------------------------

# Patterns that signal prompt injection / instruction-hijacking attempts
_INJECTION_PATTERNS = [
    re.compile(r"ignore\s+(all\s+)?previous\s+instructions?", re.IGNORECASE),
    re.compile(r"you\s+are\s+now\s+", re.IGNORECASE),
    re.compile(r"act\s+as\s+(a|an)\s+", re.IGNORECASE),
    re.compile(r"disregard\s+(all\s+)?prior\s+", re.IGNORECASE),
    re.compile(r"system\s*:\s*", re.IGNORECASE),
    re.compile(r"<\s*/?system\s*>", re.IGNORECASE),
    re.compile(r"\[INST\]|\[/INST\]", re.IGNORECASE),
    re.compile(r"<\|im_start\|>|<\|im_end\|>"),
    re.compile(r"<\|system\|>|<\|user\|>|<\|assistant\|>"),
]

MAX_CHUNK_CHARS = 8_000  # roughly ~2 k tokens

# ---------------------------------------------------------------------------
# Audit instrumentation
# ---------------------------------------------------------------------------

# Rejection reason codes — used in structured log entries and metrics
REJECT_NOT_STRING = "not_string"
REJECT_EMPTY = "empty"
REJECT_TOO_LONG = "too_long"
REJECT_CONTROL_CHARS = "control_chars"
REJECT_INJECTION_EMPTY = "empty_after_injection_strip"

# Per-category rejection counters for compliance reporting
_rejection_counts: dict[str, int] = {
    REJECT_NOT_STRING: 0,
    REJECT_EMPTY: 0,
    REJECT_TOO_LONG: 0,
    REJECT_CONTROL_CHARS: 0,
    REJECT_INJECTION_EMPTY: 0,
}

# Sanitization rule version — SHA-256 of the pattern set for change audit trail.
# Logged on startup so operators can verify which ruleset is active.
_RULE_VERSION: str = hashlib.sha256(
    "|".join(p.pattern for p in _INJECTION_PATTERNS).encode()
).hexdigest()[:16]

logging.info("sanitize_content rule_version=%s", _RULE_VERSION)


def _payload_hash(text: str) -> str:
    """SHA-256 hex digest of raw text — safe to log as a forensic fingerprint."""
    return hashlib.sha256(text.encode("utf-8", errors="replace")).hexdigest()


def sanitize_content_with_reason(
    text: str,
) -> tuple[Optional[str], Optional[str]]:
    """
    Validate and sanitize document content before ingestion into the vector store.

    Returns (clean_text, rejection_reason_code).
    On success: (cleaned_str, None).
    On rejection: (None, reason_code) where reason_code is one of the REJECT_* constants.
    """
    if not isinstance(text, str):
        _rejection_counts[REJECT_NOT_STRING] += 1
        return None, REJECT_NOT_STRING

    # Unicode normalization to defeat homoglyph / encoding evasion
    text = unicodedata.normalize("NFKC", text)

    text = text.strip()
    if not text:
        _rejection_counts[REJECT_EMPTY] += 1
        return None, REJECT_EMPTY

    # Reject excessively long chunks
    if len(text) > MAX_CHUNK_CHARS:
        _rejection_counts[REJECT_TOO_LONG] += 1
        return None, REJECT_TOO_LONG

    # Allowlisted control characters: \t (\x09), \n (\x0a), \r (\x0d).
    # All other C0/C1 control characters and DEL are rejected.
    if re.search(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", text):
        _rejection_counts[REJECT_CONTROL_CHARS] += 1
        return None, REJECT_CONTROL_CHARS

    # Strip instruction-like patterns (replace with empty string)
    for pattern in _INJECTION_PATTERNS:
        text = pattern.sub("", text)

    text = text.strip()
    if not text:
        _rejection_counts[REJECT_INJECTION_EMPTY] += 1
        return None, REJECT_INJECTION_EMPTY

    return text, None


def sanitize_content(text: str) -> Optional[str]:
    """Thin wrapper around sanitize_content_with_reason(); returns clean text or None."""
    clean, _ = sanitize_content_with_reason(text)
    return clean


# ---------------------------------------------------------------------------
# Seeding helpers
# ---------------------------------------------------------------------------


def generate_embedding(text: str) -> list[float]:
    """
    Generate an embedding vector for the given text.

    Replace this stub with a real embedding client (e.g. OpenAI, Cohere,
    or a local sentence-transformer) before running in production.
    """
    raise NotImplementedError(
        "generate_embedding() must be implemented with a real embedding client."
    )


def seed_document(index, doc_id: str, raw_text: str, metadata: dict) -> bool:
    """
    Sanitize content, generate an embedding, and upsert into Pinecone.

    Returns True on success, False if the content was rejected by sanitization.
    Document ID is logged on rejection — the raw content is intentionally NOT
    logged to avoid persisting injected payloads in log files.
    """
    clean_text, reason = sanitize_content_with_reason(raw_text)  # line 63 — sanitization applied here
    if clean_text is None:
        logging.warning(
            "Rejected document doc_id=%s reason=%s payload_sha256=%s rule_version=%s",
            doc_id,
            reason,
            _payload_hash(raw_text) if isinstance(raw_text, str) else "n/a",
            _RULE_VERSION,
        )
        return False

    embedding = generate_embedding(clean_text)
    index.upsert(vectors=[(doc_id, embedding, {**metadata, "text": clean_text})])
    logging.info("Upserted document %s (%d chars)", doc_id, len(clean_text))
    return True


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone vector index with sanitized document embeddings."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Run sanitization and embedding generation without writing to Pinecone.",
    )
    parser.add_argument(
        "--input",
        metavar="FILE",
        help="Path to a JSONL file containing documents (fields: id, text, metadata).",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    if args.dry_run:
        logging.info("Dry-run mode enabled — no vectors will be written to Pinecone.")

    if not args.input:
        logging.error("--input is required.")
        raise SystemExit(1)

    import json

    rejected = 0
    accepted = 0

    with open(args.input, encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, start=1):
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except json.JSONDecodeError as exc:
                logging.warning("Line %d: invalid JSON — %s", lineno, exc)
                rejected += 1
                continue

            doc_id = record.get("id", f"doc-{lineno}")
            raw_text = record.get("text", "")
            record.get("metadata", {})

            clean_text, reason = sanitize_content_with_reason(raw_text)
            if clean_text is None:
                logging.warning(
                    "Rejected document doc_id=%s reason=%s payload_sha256=%s rule_version=%s",
                    doc_id,
                    reason,
                    _payload_hash(raw_text) if isinstance(raw_text, str) else "n/a",
                    _RULE_VERSION,
                )
                rejected += 1
                continue

            if args.dry_run:
                logging.info("DRY-RUN: would upsert document %s (%d chars)", doc_id, len(clean_text))
                accepted += 1
                continue

            # In production, initialise the Pinecone index here and pass it to
            # seed_document().  The index initialisation is intentionally left
            # as a stub so that credentials are injected at runtime via
            # environment variables rather than hard-coded in this file.
            logging.warning(
                "Non-dry-run mode requires Pinecone index initialisation — "
                "implement before production use."
            )
            break

    logging.info("Done. accepted=%d rejected=%d", accepted, rejected)
    logging.info(
        "Rejection metrics rule_version=%s %s",
        _RULE_VERSION,
        " ".join(f"{k}={v}" for k, v in _rejection_counts.items() if v > 0),
    )


if __name__ == "__main__":
    main()
