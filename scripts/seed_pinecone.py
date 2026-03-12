"""
seed_pinecone.py — Seed a Pinecone vector index from source documents.

Security note (rag_002): All text chunks are sanitized via `sanitize_content`
before embedding and upsert to prevent RAG prompt-injection attacks.

Compliance notes:
- Legal basis for processing: legitimate interest / contractual necessity.
  Review with your DPO before deploying in regulated environments (GDPR, HIPAA).
- Third-party data sharing: embeddings and optional metadata are sent to
  OpenAI (embeddings API) and Pinecone (vector storage).  Ensure your DPA /
  BAA with each vendor is in place before ingesting personal or health data.
- Audit log: all sanitization decisions and upsert operations are written to a
  dedicated audit logger (``seed_pinecone.audit``) with ISO-8601 timestamps.
- Retention: vectors should be deleted after RETENTION_DAYS.  Wire
  ``_enforce_retention`` into your scheduler for automated enforcement.
- Data minimisation: full text is NOT stored in Pinecone metadata by default
  (STORE_TEXT_IN_METADATA=False).  Enable only if retrieval requires it and
  only after confirming the legal basis for storing that content.
"""

import datetime
import logging
import os
import re
import unicodedata
from typing import Optional

# ---------------------------------------------------------------------------
# Configuration (from environment — never hardcoded)
# ---------------------------------------------------------------------------
PINECONE_API_KEY: str = os.environ["PINECONE_API_KEY"]
PINECONE_INDEX: str = os.environ["PINECONE_INDEX"]
OPENAI_API_KEY: str = os.environ["OPENAI_API_KEY"]

MIN_CHUNK_LENGTH: int = 20        # characters
MAX_CHUNK_LENGTH: int = 8_000     # characters

# Retention policy: vectors older than this should be purged.
RETENTION_DAYS: int = int(os.environ.get("RETENTION_DAYS", "365"))

# Data minimisation: set to True only when full-text retrieval is required
# and a legal basis for storing plaintext in the vector store exists.
STORE_TEXT_IN_METADATA: bool = os.environ.get("STORE_TEXT_IN_METADATA", "false").lower() == "true"

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
logger = logging.getLogger(__name__)

# Dedicated audit logger — wire a tamper-evident handler (e.g. a SIEM sink or
# an append-only log file) in production.  Uses ISO-8601 timestamps.
_audit_handler = logging.StreamHandler()
_audit_handler.setFormatter(
    logging.Formatter("%(asctime)s AUDIT %(levelname)s %(message)s", datefmt="%Y-%m-%dT%H:%M:%S%z")
)
audit_logger = logging.getLogger("seed_pinecone.audit")
audit_logger.addHandler(_audit_handler)
audit_logger.setLevel(logging.INFO)
audit_logger.propagate = False

# ---------------------------------------------------------------------------
# Injection-pattern detection (rag_002 remediation)
# ---------------------------------------------------------------------------
_INJECTION_PATTERNS = re.compile(
    r"(ignore\s+(all\s+)?(previous|prior|above)\s+instructions?"
    r"|you\s+are\s+now\s+a"
    r"|act\s+as\s+(a|an)\s+"
    r"|system\s*:\s*"
    r"|<\s*system\s*>"
    r"|\[INST\]"
    r"|###\s*(instruction|system|prompt))",
    re.IGNORECASE,
)


def sanitize_content(text: str) -> Optional[str]:
    """Validate and sanitize a text chunk before adding it to the vector store.

    Returns the cleaned text, or None if the chunk should be skipped.

    Security controls applied (rag_002):
    - Unicode normalization (NFKC) to defeat lookalike-character evasion.
    - Whitespace stripping.
    - Minimum length rejection (avoids noise and empty vectors).
    - Injection-pattern detection (prompt-injection / role-play directives).
    - Maximum length truncation (avoids oversized embeddings).
    """
    # 1. Normalize unicode to defeat lookalike-character evasion.
    text = unicodedata.normalize("NFKC", text)

    # 2. Strip leading/trailing whitespace.
    text = text.strip()

    # 3. Reject empty or too-short chunks.
    if len(text) < MIN_CHUNK_LENGTH:
        logger.debug("Skipping chunk: below minimum length (%d chars).", len(text))
        return None

    # 4. Detect and reject instruction-like / prompt-injection patterns.
    match = _INJECTION_PATTERNS.search(text)
    if match:
        # Redact matched content from logs to avoid leaking injected payload
        # into log aggregation systems (compliance finding #3).
        audit_logger.warning(
            "sanitize_content: injection pattern detected at char offset %d — chunk rejected (content redacted)",
            match.start(),
        )
        return None

    # 5. Truncate to safe maximum length.
    if len(text) > MAX_CHUNK_LENGTH:
        audit_logger.info(
            "sanitize_content: chunk truncated from %d to %d chars", len(text), MAX_CHUNK_LENGTH
        )
        text = text[:MAX_CHUNK_LENGTH]

    audit_logger.debug("sanitize_content: chunk accepted (%d chars)", len(text))
    return text


# ---------------------------------------------------------------------------
# Embedding + upsert
# ---------------------------------------------------------------------------

def _get_embedding(text: str) -> list[float]:
    """Return an embedding vector for *text* using the OpenAI Embeddings API."""
    import openai  # imported lazily so the module is testable without the SDK

    client = openai.OpenAI(api_key=OPENAI_API_KEY)
    response = client.embeddings.create(
        model="text-embedding-3-small",
        input=text,
    )
    return response.data[0].embedding


def embed_and_upsert(chunks: list, index, data_subject_id: Optional[str] = None) -> None:
    """Sanitize, embed, and upsert *chunks* into the Pinecone *index*.

    Each element of *chunks* must have a ``.text`` attribute (str) and an
    ``.id`` attribute (str) used as the Pinecone vector ID.

    Chunks that fail sanitization are silently skipped (a warning is logged).

    Args:
        chunks: Iterable of chunk objects with ``.id`` and ``.text``.
        index: Pinecone index object.
        data_subject_id: Optional identifier for the data subject (e.g. user ID
            or document owner) — stored in metadata to support DSAR
            (access/erasure) requests.  Pass ``None`` when not applicable.
    """
    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
    retention_until = (
        datetime.datetime.now(datetime.timezone.utc)
        + datetime.timedelta(days=RETENTION_DAYS)
    ).isoformat()

    vectors: list[tuple] = []
    rejected = 0

    for chunk in chunks:
        # rag_002 fix: validate and sanitize before adding to vector store.
        clean_text = sanitize_content(chunk.text)
        if clean_text is None:
            rejected += 1
            continue  # skip rejected chunk

        embedding = _get_embedding(clean_text)

        # Metadata tagging for compliance:
        # - ingested_at / retention_until: support retention enforcement.
        # - data_subject_id: support DSAR access/erasure lookups.
        # - text: only included when STORE_TEXT_IN_METADATA is True (data minimisation).
        metadata: dict = {
            "ingested_at": now_iso,
            "retention_until": retention_until,
        }
        if data_subject_id is not None:
            metadata["data_subject_id"] = data_subject_id
        if STORE_TEXT_IN_METADATA:
            metadata["text"] = clean_text

        vectors.append((chunk.id, embedding, metadata))

    if vectors:
        index.upsert(vectors=vectors)
        audit_logger.info(
            "embed_and_upsert: upserted %d vectors (rejected=%d, retention_until=%s)",
            len(vectors),
            rejected,
            retention_until,
        )
    else:
        audit_logger.info(
            "embed_and_upsert: no vectors upserted (all %d chunks rejected)", rejected
        )


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def _enforce_retention(index) -> None:
    """Delete vectors whose ``retention_until`` metadata has passed.

    Wire this into a scheduled job (e.g. cron, Celery beat) to automate
    retention policy enforcement.  The implementation depends on your Pinecone
    tier's metadata-filter support; adapt the query as needed.
    """
    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
    audit_logger.info("_enforce_retention: scanning for vectors expired before %s", now_iso)
    # Example: index.delete(filter={"retention_until": {"$lt": now_iso}})
    # Uncomment and adapt once you confirm filter-delete support on your index.
    raise NotImplementedError(
        "Implement _enforce_retention() using index.delete(filter=...) "
        "to purge vectors past their retention_until date."
    )


def main() -> None:
    """Read source documents, split into chunks, and seed the Pinecone index."""
    from pinecone import Pinecone  # type: ignore[import-untyped]

    audit_logger.info("main: seed job started (index=%s)", PINECONE_INDEX)

    pc = Pinecone(api_key=PINECONE_API_KEY)
    index = pc.Index(PINECONE_INDEX)

    # Load and chunk documents.  Replace this block with your actual document
    # loading / chunking logic (e.g., LangChain TextSplitter, custom splitter).
    documents = _load_documents()
    chunks = _split_into_chunks(documents)

    embed_and_upsert(chunks, index)
    audit_logger.info("main: seed job completed")


def _load_documents() -> list:
    """Load source documents.  Override with project-specific logic."""
    raise NotImplementedError(
        "Implement _load_documents() to return a list of document objects."
    )


def _split_into_chunks(documents: list) -> list:
    """Split documents into text chunks.  Override with project-specific logic."""
    raise NotImplementedError(
        "Implement _split_into_chunks() to return a list of chunk objects "
        "with .id (str) and .text (str) attributes."
    )


if __name__ == "__main__":
    main()
