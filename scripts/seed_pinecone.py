"""
scripts/seed_pinecone.py

Utility script for seeding a Pinecone vector store with document embeddings.
Content is validated and sanitized before ingestion to prevent RAG poisoning
and prompt injection attacks.
"""

import argparse
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


def sanitize_content(text: str) -> Optional[str]:
    """
    Validate and sanitize document content before ingestion into the vector store.

    Steps:
    1. Reject non-string or empty input.
    2. Apply Unicode NFKC normalization to defeat homoglyph/encoding evasion.
    3. Reject null bytes and non-printable control characters.
    4. Reject chunks that exceed the maximum allowed size.
    5. Strip instruction-injection patterns.
    6. Return the cleaned text, or None if it should be rejected entirely.
    """
    if not isinstance(text, str):
        return None

    # Unicode normalization to defeat homoglyph / encoding evasion
    text = unicodedata.normalize("NFKC", text)

    text = text.strip()
    if not text:
        return None

    # Reject excessively long chunks
    if len(text) > MAX_CHUNK_CHARS:
        return None

    # Reject null bytes and non-printable control characters
    if re.search(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", text):
        return None

    # Strip instruction-like patterns (replace with empty string)
    for pattern in _INJECTION_PATTERNS:
        text = pattern.sub("", text)

    text = text.strip()
    if not text:
        return None

    return text


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
    clean_text = sanitize_content(raw_text)  # line 63 — sanitization applied here
    if clean_text is None:
        logging.warning("Rejected document %s: failed sanitization", doc_id)
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
            metadata = record.get("metadata", {})

            clean_text = sanitize_content(raw_text)
            if clean_text is None:
                logging.warning("Rejected document %s: failed sanitization", doc_id)
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


if __name__ == "__main__":
    main()
