"""
seed_pinecone.py — Seed a Pinecone vector index from a document corpus.

Content is sanitized (instruction-like patterns stripped) and validated
(length bounds enforced) before any vector is upserted, addressing rag_002.
"""

from __future__ import annotations

import logging
import re
import unicodedata
from typing import List

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Content moderation — instruction-injection patterns
# ---------------------------------------------------------------------------

_INSTRUCTION_PATTERNS: List[re.Pattern] = [
    re.compile(r"ignore\s+(all\s+)?previous\s+instructions?", re.I),
    re.compile(r"you\s+are\s+(now\s+)?a\s+", re.I),
    re.compile(r"system\s*:\s*", re.I),
    re.compile(r"<\s*/?(?:system|user|assistant)\s*>", re.I),
    re.compile(r"\[INST\]|\[/INST\]", re.I),
    re.compile(r"###\s*Instruction", re.I),
]

MAX_CHARS = 50_000
MIN_CHARS = 20


def _normalize(text: str) -> str:
    """Apply NFKC normalization to collapse homoglyphs and zero-width chars."""
    return unicodedata.normalize("NFKC", text)


def sanitize_content(text: str) -> str:
    """Strip instruction-like patterns and normalise whitespace.

    Args:
        text: Raw document text to sanitize.

    Returns:
        Sanitized text with prompt-injection patterns removed.
    """
    text = _normalize(text)
    for pat in _INSTRUCTION_PATTERNS:
        text = pat.sub("", text)
    # Collapse excessive whitespace introduced by stripping
    text = re.sub(r"\s{3,}", "\n\n", text)
    return text.strip()


def validate_content(text: str) -> None:
    """Validate that sanitized content meets length requirements.

    Args:
        text: Sanitized document text to validate.

    Raises:
        ValueError: If the text is too short or too long.
    """
    if len(text) < MIN_CHARS:
        raise ValueError(
            f"Content too short after sanitization ({len(text)} chars, min={MIN_CHARS})"
        )
    if len(text) > MAX_CHARS:
        raise ValueError(
            f"Content exceeds maximum length ({len(text)} chars, max={MAX_CHARS})"
        )


# ---------------------------------------------------------------------------
# Seeding logic
# ---------------------------------------------------------------------------

def seed_documents(index, documents, embed_fn, batch_size: int = 100) -> None:
    """Sanitize, validate, embed, and upsert documents into a Pinecone index.

    Args:
        index:      Pinecone Index object (already initialised by the caller).
        documents:  Iterable of dicts with keys ``id`` and ``text``.
        embed_fn:   Callable that accepts a list of strings and returns a list
                    of float vectors (one per string).
        batch_size: Number of vectors to upsert per batch.
    """
    vectors = []
    skipped = 0

    for doc in documents:
        doc_id: str = doc["id"]
        raw_text: str = doc["text"]

        # --- line 63: sanitize and validate before upsert ---
        safe_text = sanitize_content(raw_text)
        try:
            validate_content(safe_text)
        except ValueError as exc:
            logger.warning("Skipping document %r: %s", doc_id, exc)
            skipped += 1
            continue

        embedding = embed_fn([safe_text])[0]
        vectors.append(
            {"id": doc_id, "values": embedding, "metadata": {"text": safe_text}}
        )

        if len(vectors) >= batch_size:
            index.upsert(vectors=vectors)
            logger.info("Upserted batch of %d vectors", len(vectors))
            vectors.clear()

    if vectors:
        index.upsert(vectors=vectors)
        logger.info("Upserted final batch of %d vectors", len(vectors))

    logger.info("Seeding complete. Skipped %d document(s).", skipped)


# ---------------------------------------------------------------------------
# CLI entry-point (optional — configure via environment variables)
# ---------------------------------------------------------------------------

def main() -> None:
    import os

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")

    api_key = os.environ.get("PINECONE_API_KEY", "")
    index_name = os.environ.get("PINECONE_INDEX", "")

    if not api_key or not index_name:
        raise SystemExit(
            "PINECONE_API_KEY and PINECONE_INDEX environment variables must be set."
        )

    try:
        from pinecone import Pinecone  # type: ignore[import]
    except ImportError as exc:
        raise SystemExit(f"pinecone-client is not installed: {exc}") from exc

    pc = Pinecone(api_key=api_key)
    idx = pc.Index(index_name)

    # Placeholder: replace with your actual document loader and embedding function.
    sample_documents: list[dict] = []
    def placeholder_embed(texts: list[str]) -> list[list[float]]:
        raise NotImplementedError("Replace placeholder_embed with a real embedding function.")

    seed_documents(idx, sample_documents, placeholder_embed)


if __name__ == "__main__":
    main()
