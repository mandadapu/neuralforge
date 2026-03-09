"""
seed_pinecone.py — Seed a Pinecone vector store from a document corpus.

Security: All chunks are validated and sanitized before upsert to prevent
indirect prompt injection (RAG poisoning) via ingested documents.
"""

import argparse
import hashlib
import logging
import re
import sys
from typing import Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Content-safety constants
# ---------------------------------------------------------------------------

MAX_CHUNK_BYTES = 40_000  # Pinecone metadata size limit

# Patterns commonly used in prompt-injection / indirect prompt-injection attacks.
# Text matching any of these is rejected before ingestion.
INSTRUCTION_PATTERNS = [
    r"ignore (all |previous |above )?instructions?",
    r"disregard (all |previous |above )?instructions?",
    r"you are now",
    r"act as (a |an )?",
    r"system prompt",
    r"<\|.*?\|>",           # model control tokens (e.g. <|endoftext|>)
    r"\[INST\].*?\[/INST\]",  # Llama-style instruction delimiters
]

_COMPILED_PATTERNS = [re.compile(p, re.IGNORECASE | re.DOTALL) for p in INSTRUCTION_PATTERNS]


# ---------------------------------------------------------------------------
# Validation helpers
# ---------------------------------------------------------------------------

def contains_injection(text: str) -> bool:
    """Return True if *text* matches any known prompt-injection pattern."""
    for pattern in _COMPILED_PATTERNS:
        if pattern.search(text):
            return True
    return False


def validate_chunk(text: str) -> str:
    """Validate and sanitize a text chunk before vector-store ingestion.

    Raises:
        ValueError: if the chunk is empty, too large, or contains
                    instruction-like injection patterns.

    Returns:
        The stripped, safe chunk text.
    """
    if not text or not text.strip():
        raise ValueError("empty chunk rejected")

    stripped = text.strip()

    if len(stripped.encode("utf-8")) > MAX_CHUNK_BYTES:
        raise ValueError(
            f"chunk exceeds {MAX_CHUNK_BYTES} bytes "
            f"(actual: {len(stripped.encode('utf-8'))} bytes)"
        )

    if contains_injection(stripped):
        raise ValueError("chunk contains instruction-like pattern — rejected")

    return stripped


def _chunk_id(text: str) -> str:
    """Return a short SHA-256 prefix for logging without exposing raw content."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]


# ---------------------------------------------------------------------------
# Seeding logic
# ---------------------------------------------------------------------------

def seed(index: Any, chunks: list[dict], namespace: str = "") -> dict:
    """Upsert *chunks* into *index*, skipping any that fail validation.

    Each element of *chunks* must have:
        - ``"id"``   (str)  — unique vector ID
        - ``"text"`` (str)  — raw document text
        - ``"embedding"`` (list[float]) — pre-computed embedding vector

    Returns a dict with ``upserted`` and ``skipped`` counts.
    """
    upserted = 0
    skipped = 0

    for chunk in chunks:
        raw_text = chunk.get("text", "")
        chunk_hash = _chunk_id(raw_text)

        try:
            safe_text = validate_chunk(raw_text)
        except ValueError as exc:
            # Log the hash, not the raw content, to avoid log-injection.
            logger.warning("skipping chunk %s: %s", chunk_hash, exc)
            skipped += 1
            continue

        vector = {
            "id": chunk["id"],
            "values": chunk["embedding"],
            "metadata": {"text": safe_text},
        }

        # Line 63 — the actual upsert; only reached for validated, safe chunks.
        index.upsert(vectors=[vector], namespace=namespace)
        upserted += 1
        logger.debug("upserted chunk %s", chunk_hash)

    logger.info("seed complete — upserted=%d skipped=%d", upserted, skipped)
    return {"upserted": upserted, "skipped": skipped}


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone index from a JSONL document corpus.",
    )
    parser.add_argument("--index", required=True, help="Pinecone index name")
    parser.add_argument("--namespace", default="", help="Pinecone namespace")
    parser.add_argument("--input", required=True, help="Path to input JSONL file")
    parser.add_argument(
        "--allow-unsafe",
        action="store_true",
        default=False,
        help=(
            "Skip injection-pattern checks for trusted corpora "
            "(e.g. AI-safety research papers that quote injection examples). "
            "Use with caution."
        ),
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    import json
    import os

    try:
        import pinecone  # type: ignore[import]
    except ImportError:
        logger.error("pinecone-client is not installed — run: pip install pinecone-client")
        return 1

    args = _build_parser().parse_args(argv)

    api_key = os.environ.get("PINECONE_API_KEY")
    if not api_key:
        logger.error("PINECONE_API_KEY environment variable is not set")
        return 1

    pinecone.init(api_key=api_key)  # type: ignore[attr-defined]
    index = pinecone.Index(args.index)  # type: ignore[attr-defined]

    if args.allow_unsafe:
        logger.warning(
            "--allow-unsafe is set: injection-pattern checks are DISABLED. "
            "Only use this for fully trusted, pre-reviewed corpora."
        )
        # Monkey-patch contains_injection to always return False for this run.
        global contains_injection  # noqa: PLW0603
        contains_injection = lambda _text: False  # noqa: E731

    chunks: list[dict] = []
    try:
        with open(args.input, encoding="utf-8") as fh:
            for lineno, line in enumerate(fh, start=1):
                line = line.strip()
                if not line:
                    continue
                try:
                    chunks.append(json.loads(line))
                except json.JSONDecodeError as exc:
                    logger.warning("malformed JSON on line %d: %s", lineno, exc)
    except OSError as exc:
        logger.error("cannot open input file: %s", exc)
        return 1

    result = seed(index, chunks, namespace=args.namespace)
    print(f"Done. upserted={result['upserted']} skipped={result['skipped']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
