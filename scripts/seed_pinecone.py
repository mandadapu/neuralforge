"""
scripts/seed_pinecone.py — Seed documents into a Pinecone vector store.

Security: All document content is validated and sanitized before ingestion to
prevent prompt-injection attacks via RAG retrieval.
"""

from __future__ import annotations

import datetime
import json
import logging
import os
import re
import sys
from typing import Any

# ---------------------------------------------------------------------------
# Audit logging — structured JSON events for accountability
# ---------------------------------------------------------------------------

_audit_logger = logging.getLogger("seed_pinecone.audit")
_audit_logger.setLevel(logging.INFO)
if not _audit_logger.handlers:
    _handler = logging.StreamHandler(sys.stderr)
    _handler.setFormatter(logging.Formatter("%(message)s"))
    _audit_logger.addHandler(_handler)


def _audit(event: str, **fields: Any) -> None:
    """Emit a structured JSON audit event to stderr."""
    record = {"timestamp": datetime.datetime.utcnow().isoformat() + "Z", "event": event, **fields}
    _audit_logger.info(json.dumps(record, default=str))

# ---------------------------------------------------------------------------
# Content sanitization — must run before any upsert (addresses line 63)
# ---------------------------------------------------------------------------

# Patterns that indicate prompt-injection attempts or instruction hijacking.
_INSTRUCTION_PATTERNS = [
    re.compile(r"ignore\s+(all\s+)?(previous|prior|above)\s+instructions?", re.IGNORECASE),
    re.compile(r"disregard\s+(all\s+)?(previous|prior|above)", re.IGNORECASE),
    re.compile(r"you\s+are\s+now\s+(?:a|an)\s+", re.IGNORECASE),
    re.compile(r"act\s+as\s+(a|an)\s+", re.IGNORECASE),
    re.compile(r"<\s*system\s*>", re.IGNORECASE),
    re.compile(r"\[SYSTEM\]", re.IGNORECASE),
    re.compile(r"^\s*system\s*:", re.IGNORECASE | re.MULTILINE),
    re.compile(r"^\s*user\s*:", re.IGNORECASE | re.MULTILINE),
    re.compile(r"^\s*assistant\s*:", re.IGNORECASE | re.MULTILINE),
    re.compile(r"new\s+instructions?\s*:", re.IGNORECASE),
    re.compile(r"override\s+(previous\s+)?instructions?", re.IGNORECASE),
    re.compile(r"<\s*/?prompt\s*>", re.IGNORECASE),
    re.compile(r"<<<.*?>>>", re.DOTALL),
]

MAX_CONTENT_BYTES = 40_000  # Pinecone metadata limit is 40 KB


def sanitize_content(text: str) -> tuple[str, list[str]]:
    """Validate and sanitize document content before ingestion into the vector store.

    - Enforces a maximum byte length (raises ValueError if exceeded).
    - Strips instruction-like patterns that could enable prompt injection.

    Returns ``(sanitized_text, redacted_patterns)`` where ``redacted_patterns``
    is a list of regex pattern strings that matched and were redacted.
    Raises ValueError if content is invalid or empty after sanitization.
    """
    if not isinstance(text, str):
        raise ValueError(f"Content must be a string, got {type(text)}")

    # Enforce byte limit — raise rather than silently corrupt
    encoded = text.encode("utf-8")
    if len(encoded) > MAX_CONTENT_BYTES:
        raise ValueError(
            f"Content exceeds maximum allowed size of {MAX_CONTENT_BYTES} bytes "
            f"({len(encoded)} bytes). Truncate or split the document before ingestion."
        )

    # Strip instruction-like patterns and track what was redacted
    redacted_patterns: list[str] = []
    for pattern in _INSTRUCTION_PATTERNS:
        new_text, count = pattern.subn("[REDACTED]", text)
        if count:
            redacted_patterns.append(pattern.pattern)
            text = new_text

    # Collapse runs of whitespace introduced by redaction
    text = re.sub(r"\n{3,}", "\n\n", text)
    text = text.strip()

    if not text:
        raise ValueError("Content is empty after sanitization")

    return text, redacted_patterns


# ---------------------------------------------------------------------------
# Embedding helper
# ---------------------------------------------------------------------------


def get_embedding(text: str) -> list[float]:
    """Return an embedding vector for *text*.

    Requires the ``openai`` package and the ``OPENAI_API_KEY`` environment
    variable.  Swap this implementation for any embedding provider as needed.
    """
    try:
        import openai  # type: ignore[import]
    except ImportError as exc:
        raise RuntimeError(
            "openai package is required for embeddings: pip install openai"
        ) from exc

    client = openai.OpenAI(api_key=os.environ["OPENAI_API_KEY"])
    response = client.embeddings.create(
        model=os.environ.get("EMBEDDING_MODEL", "text-embedding-3-small"),
        input=text,
    )
    return response.data[0].embedding


# ---------------------------------------------------------------------------
# Pinecone helpers
# ---------------------------------------------------------------------------


def get_index() -> Any:
    """Return a Pinecone Index object using environment-configured credentials."""
    try:
        from pinecone import Pinecone  # type: ignore[import]
    except ImportError as exc:
        raise RuntimeError(
            "pinecone-client package is required: pip install pinecone-client"
        ) from exc

    pc = Pinecone(api_key=os.environ["PINECONE_API_KEY"])
    index_name = os.environ["PINECONE_INDEX_NAME"]
    return pc.Index(index_name)


# ---------------------------------------------------------------------------
# Core seeding logic
# ---------------------------------------------------------------------------


def seed_documents(
    documents: list[dict[str, Any]],
    *,
    dry_run: bool = False,
    batch_size: int = 100,
) -> None:
    """Sanitize and upsert *documents* into the configured Pinecone index.

    Each document dict must have:
      - ``id``   (str)  — unique document identifier
      - ``text`` (str)  — raw text content to embed
      - ``metadata`` (dict, optional) — additional metadata fields

    Documents that fail sanitization are skipped with a warning.
    """
    index = None if dry_run else get_index()

    vectors_to_upsert: list[dict[str, Any]] = []

    for doc in documents:
        doc_id = doc.get("id", "?")
        try:
            # --- line 63: sanitize before any upsert ---
            clean_text, redacted_patterns = sanitize_content(doc["text"])
        except (ValueError, KeyError) as exc:
            _audit(
                "document_skipped",
                doc_id=doc_id,
                reason=str(exc),
            )
            print(f"[SKIP] Document '{doc_id}' failed sanitization: {exc}")
            continue

        if redacted_patterns:
            _audit(
                "content_redacted",
                doc_id=doc_id,
                original_length=len(doc["text"]),
                sanitized_length=len(clean_text),
                redacted_pattern_count=len(redacted_patterns),
                redacted_patterns=redacted_patterns,
            )

        if dry_run:
            print(f"[DRY-RUN] Would upsert id={doc_id!r} ({len(clean_text)} chars)")
            continue

        embedding = get_embedding(clean_text)
        vectors_to_upsert.append(
            {
                "id": doc_id,
                "values": embedding,
                "metadata": {
                    "text": clean_text,
                    "original_length": len(doc["text"]),
                    "was_sanitized": bool(redacted_patterns),
                    **doc.get("metadata", {}),
                },
            }
        )

        # Upsert in batches to respect Pinecone request-size limits.
        if len(vectors_to_upsert) >= batch_size:
            index.upsert(vectors=vectors_to_upsert)
            _audit("batch_upserted", count=len(vectors_to_upsert))
            print(f"[INFO] Upserted batch of {len(vectors_to_upsert)} vectors")
            vectors_to_upsert = []

    # Flush remaining vectors
    if vectors_to_upsert:
        index.upsert(vectors=vectors_to_upsert)
        _audit("batch_upserted", count=len(vectors_to_upsert))
        print(f"[INFO] Upserted final batch of {len(vectors_to_upsert)} vectors")


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> None:
    import argparse

    parser = argparse.ArgumentParser(
        description="Seed documents into a Pinecone vector store with sanitization."
    )
    parser.add_argument(
        "input_file",
        help="Path to a JSON file containing a list of document objects.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be upserted without calling Pinecone.",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=100,
        help="Number of vectors per upsert request (default: 100).",
    )
    args = parser.parse_args(argv)

    with open(args.input_file, encoding="utf-8") as fh:
        documents = json.load(fh)

    if not isinstance(documents, list):
        print("ERROR: Input file must contain a JSON array of document objects.", file=sys.stderr)
        sys.exit(1)

    seed_documents(documents, dry_run=args.dry_run, batch_size=args.batch_size)


if __name__ == "__main__":
    main()
