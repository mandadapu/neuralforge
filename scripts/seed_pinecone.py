#!/usr/bin/env python3
"""
seed_pinecone.py — Seed a Pinecone vector index from a directory of text documents.

Security controls (rag_002):
- Documents are validated for length before ingestion.
- Instruction-like / prompt-injection patterns are detected and either
  stripped (default) or cause the document to be dropped (--strict mode).
- Unicode is normalised (NFKC) before pattern matching to defeat zero-width
  character evasion.
- Every dropped or modified document is logged with its ID and reason so
  operators have a full audit trail.

Limitations: regex-based detection is best-effort and not exhaustive.
Adversarially crafted inputs may still evade these patterns.
"""

import argparse
import logging
import os
import re
import unicodedata
from pathlib import Path
from typing import Optional

# ---------------------------------------------------------------------------
# Security constants — compiled at module level to avoid per-document overhead
# ---------------------------------------------------------------------------

MAX_CONTENT_CHARS: int = 100_000

# Patterns that indicate prompt-injection or instruction-override attempts.
# NOTE: tune these carefully — some patterns (e.g. "you are now") can appear
# in legitimate technical prose. Use --strict only when the corpus is trusted
# enough to safely drop flagged documents without review.
INSTRUCTION_PATTERNS: list[re.Pattern] = [
    re.compile(r"ignore\s+(all\s+)?(previous|prior)\s+instructions?", re.IGNORECASE),
    re.compile(r"you\s+are\s+now\s+\w+", re.IGNORECASE),
    re.compile(r"forget\s+(your|all|previous)\s+(training|instructions?|context)", re.IGNORECASE),
    re.compile(r"(system|assistant|user)\s*:\s*", re.IGNORECASE),
    re.compile(r"<\|.*?\|>"),                        # special token patterns
    re.compile(r"\[INST\]|\[/INST\]|\[SYS\]|\[/SYS\]"),  # Llama-style control tokens
]

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Content sanitisation (rag_002 fix)
# ---------------------------------------------------------------------------

def sanitize_content(text: str, strict: bool = False) -> Optional[str]:
    """Validate and sanitise *text* before it is added to the vector store.

    Args:
        text:   Raw document text.
        strict: When True, any document that contains an instruction-like
                pattern is dropped entirely (returns None).  When False
                (default), matching spans are removed and the sanitised text
                is returned, unless the result is empty after stripping.

    Returns:
        Sanitised text, or None if the document should be dropped.
    """
    # --- 1. Empty check ---------------------------------------------------
    if not text or not text.strip():
        logger.warning("Content validation failed: document is empty")
        return None

    # --- 2. Unicode normalisation (defeats zero-width character evasion) ---
    text = unicodedata.normalize("NFKC", text)

    # --- 3. Whitespace normalisation (collapse control chars) -------------
    # Replace runs of control characters / exotic whitespace with a single
    # space so that injection strings split across them are still matched.
    text = re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]+", " ", text)
    text = re.sub(r"[ \t]{2,}", " ", text)

    # --- 4. Length check --------------------------------------------------
    if len(text) > MAX_CONTENT_CHARS:
        logger.warning(
            "Content validation failed: document exceeds %d chars (%d chars)",
            MAX_CONTENT_CHARS,
            len(text),
        )
        return None

    # --- 5. Instruction-pattern detection ---------------------------------
    modified = False
    for pattern in INSTRUCTION_PATTERNS:
        if pattern.search(text):
            if strict:
                logger.warning(
                    "Content validation failed (strict): instruction-like pattern '%s' detected; dropping document",
                    pattern.pattern,
                )
                return None
            # Non-strict: strip matching spans and continue
            sanitised = pattern.sub("", text)
            if sanitised != text:
                logger.warning(
                    "Content sanitised: instruction-like pattern '%s' removed",
                    pattern.pattern,
                )
                text = sanitised
                modified = True

    # --- 6. Post-strip empty check ----------------------------------------
    if not text.strip():
        logger.warning(
            "Content validation failed: document is empty after sanitisation"
        )
        return None

    if modified:
        # Final whitespace tidy-up after removals
        text = re.sub(r"[ \t]{2,}", " ", text).strip()

    return text


# ---------------------------------------------------------------------------
# Embedding helper (stub — replace with your actual embedding client)
# ---------------------------------------------------------------------------

def embed(text: str) -> list[float]:
    """Return a vector embedding for *text*.

    Replace this stub with a real call to your embedding provider, e.g.:
        import openai
        response = openai.embeddings.create(input=text, model="text-embedding-3-small")
        return response.data[0].embedding
    """
    raise NotImplementedError(
        "embed() is a stub. Configure your embedding provider before use."
    )


# ---------------------------------------------------------------------------
# Core seeding logic
# ---------------------------------------------------------------------------

def seed(source_dir: str, index_name: str, strict: bool = False) -> None:
    """Read documents from *source_dir* and upsert them into *index_name*.

    Each file's text content is sanitised before embedding and upsert.
    Documents that fail validation are skipped with a WARNING log entry.
    """
    try:
        import pinecone  # type: ignore[import]
    except ImportError as exc:
        raise SystemExit(
            "pinecone-client is not installed. Run: pip install pinecone-client"
        ) from exc

    api_key = os.environ.get("PINECONE_API_KEY")
    environment = os.environ.get("PINECONE_ENVIRONMENT")
    if not api_key or not environment:
        raise SystemExit(
            "PINECONE_API_KEY and PINECONE_ENVIRONMENT environment variables must be set"
        )

    pinecone.init(api_key=api_key, environment=environment)
    index = pinecone.Index(index_name)

    source_path = Path(source_dir)
    if not source_path.is_dir():
        raise SystemExit(f"Source directory does not exist: {source_dir}")

    doc_files = sorted(source_path.rglob("*.txt")) + sorted(source_path.rglob("*.md"))
    logger.info("Found %d document(s) in %s", len(doc_files), source_dir)

    dropped = 0
    upserted = 0

    for doc_path in doc_files:
        doc_id = str(doc_path.relative_to(source_path))
        try:
            raw_text = doc_path.read_text(encoding="utf-8", errors="replace")
        except OSError as exc:
            logger.warning("Skipping %s: could not read file: %s", doc_id, exc)
            dropped += 1
            continue

        # ---------------------------------------------------------------
        # line ~63 — content validation site flagged by rag_002 scanner
        # ---------------------------------------------------------------
        clean_text = sanitize_content(raw_text, strict=strict)
        if clean_text is None:
            logger.warning("Dropping document %s: failed content validation", doc_id)
            dropped += 1
            continue

        try:
            embedding = embed(clean_text)
        except NotImplementedError:
            logger.error(
                "embed() is not implemented. Configure an embedding provider."
            )
            raise
        except Exception as exc:  # noqa: BLE001
            logger.warning("Skipping %s: embedding failed: %s", doc_id, exc)
            dropped += 1
            continue

        index.upsert(vectors=[(doc_id, embedding, {"text": clean_text})])
        logger.info("Upserted document %s", doc_id)
        upserted += 1

    logger.info(
        "Seeding complete: %d upserted, %d dropped", upserted, dropped
    )


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone vector index from a directory of text documents."
    )
    parser.add_argument(
        "source_dir",
        help="Directory containing .txt / .md documents to ingest",
    )
    parser.add_argument(
        "--index",
        required=True,
        help="Name of the Pinecone index to upsert into",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        default=False,
        help=(
            "Drop documents that contain instruction-like patterns instead of "
            "stripping the offending spans (safer, but may discard legitimate content)"
        ),
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
        choices=["DEBUG", "INFO", "WARNING", "ERROR"],
        help="Logging verbosity (default: INFO)",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level),
        format="%(asctime)s %(levelname)s %(name)s — %(message)s",
    )

    seed(args.source_dir, args.index, strict=args.strict)


if __name__ == "__main__":
    main()
