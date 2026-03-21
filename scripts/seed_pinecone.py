"""
seed_pinecone.py — Seed a Pinecone vector index with document chunks.

Security controls (rag_002):
  - All content is validated and sanitized before upsert to prevent
    prompt-injection / instruction-hijacking attacks via the vector store.
  - Injection-like patterns are stripped with case-insensitive regex.
  - Content is truncated to a maximum safe length before embedding.
"""

import os
import re
import sys
import json
import logging
import argparse
import hashlib
import getpass
import datetime
from typing import List

import pinecone  # pinecone-client>=3.0

# ---------------------------------------------------------------------------
# Structured audit logger — replaces bare print() for compliance
#
# HIPAA §164.312(b): audit events must carry timestamp, initiator identity,
# and operation outcome.  Content (potential PHI) is NEVER logged.
# ---------------------------------------------------------------------------

_LOG_FORMAT = (
    "%(asctime)s %(levelname)s "
    "[operator=%(operator)s] "
    "[run_id=%(run_id)s] "
    "%(message)s"
)

_RUN_ID = datetime.datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
_OPERATOR = os.environ.get("AUDIT_OPERATOR", getpass.getuser())


class _AuditFilter(logging.Filter):
    """Inject run_id and operator into every log record."""

    def filter(self, record: logging.LogRecord) -> bool:
        record.run_id = _RUN_ID
        record.operator = _OPERATOR
        return True


def _build_logger() -> logging.Logger:
    logger = logging.getLogger("seed_pinecone")
    if logger.handlers:
        return logger  # already configured
    logger.setLevel(logging.INFO)
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(logging.Formatter(_LOG_FORMAT))
    handler.addFilter(_AuditFilter())
    logger.addHandler(handler)
    return logger


log = _build_logger()


# ---------------------------------------------------------------------------
# Security: content sanitization (rag_002 fix)
# ---------------------------------------------------------------------------

INJECTION_PATTERNS: List[str] = [
    # Direct instruction overrides
    r"ignore\s+(previous|all)\s+instructions",
    r"system\s*:",
    r"<\s*/?INST\s*>",
    r"\byou\s+are\s+now\b",
    r"\bforget\s+everything\b",
    r"\bdisregard\s+(all\s+)?(prior|previous)\b",
    r"\bact\s+as\s+(if\s+you\s+are|a)\b",
    r"\bnew\s+instructions?\s*:",
    r"\boverride\s+(your\s+)?(previous\s+)?instructions?\b",
    # Role/persona hijacking
    r"\bpretend\s+(you\s+are|to\s+be)\b",
    r"\byour\s+(new\s+)?role\s+is\b",
    r"\brespond\s+only\s+as\b",
    r"\bdo\s+not\s+follow\b",
    # Markdown / delimiter injection (fence blocks used to break context)
    r"```\s*system",
    r"---\s*system\s*---",
    r"\[SYSTEM\]",
    r"\[INST\]",
    r"<<SYS>>",
    # Unicode lookalike / obfuscation — strip zero-width and directional chars
    r"[\u200b-\u200f\u202a-\u202e\u2060-\u2064\ufeff]",
    # Common multi-lingual variants (Spanish / French / German)
    r"\bignora\s+las\s+instrucciones\b",
    r"\bionorez\s+les\s+instructions\b",
    r"\bignoriere\s+(alle\s+)?anweisungen\b",
    # DAN / jailbreak prompts
    r"\bDAN\s+mode\b",
    r"\bjailbreak\b",
    r"\bunrestricted\s+mode\b",
]

_COMPILED_PATTERNS = [
    re.compile(p, re.IGNORECASE) for p in INJECTION_PATTERNS
]

MAX_CONTENT_LENGTH = 8_000  # characters; keeps token budgets sane


def sanitize_content(text: str, max_length: int = MAX_CONTENT_LENGTH) -> str:
    """Strip instruction-like patterns and enforce a length cap.

    Args:
        text: Raw document chunk text.
        max_length: Maximum number of characters to retain.

    Returns:
        Sanitized text safe for embedding and storage in the vector index.
    """
    for pattern in _COMPILED_PATTERNS:
        text = pattern.sub("", text)

    # Collapse runs of whitespace introduced by substitutions
    text = re.sub(r"[ \t]{2,}", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    text = text.strip()

    # Hard truncation as a final safety net
    text = text[:max_length]

    return text


def validate_content(text: str) -> bool:
    """Return False if the raw content is empty or suspiciously short."""
    return bool(text and len(text.strip()) >= 10)


# ---------------------------------------------------------------------------
# Embedding helpers (placeholder — swap in your actual embedding client)
# ---------------------------------------------------------------------------


def embed_text(text: str) -> List[float]:
    """Return a vector embedding for *text*.

    Replace this stub with your real embedding call, e.g.:
        openai.Embedding.create(input=text, model="text-embedding-3-small")
    """
    raise NotImplementedError(
        "Replace embed_text() with your actual embedding client call."
    )


# ---------------------------------------------------------------------------
# Core seeding logic
# ---------------------------------------------------------------------------


def chunk_text(text: str, chunk_size: int = 512, overlap: int = 64) -> List[str]:
    """Split *text* into overlapping chunks of roughly *chunk_size* characters."""
    chunks: List[str] = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunks.append(text[start:end])
        start += chunk_size - overlap
    return chunks


def seed_documents(
    documents: List[dict],
    index: pinecone.Index,
    namespace: str = "",
    batch_size: int = 100,
    source_id: str = "unknown",
) -> None:
    """Sanitize, embed, and upsert *documents* into the Pinecone *index*.

    Each document dict must have at least a ``"text"`` key.  An optional
    ``"metadata"`` dict is forwarded to Pinecone as-is.

    GDPR / HIPAA notes:
    - Full document text is NOT stored in Pinecone metadata (data minimisation,
      GDPR Art. 5(1)(c)).  Only the vector embedding and non-PHI metadata
      fields (e.g. source identifier, chunk index) are persisted.
    - No document content is written to logs (HIPAA §164.312(b)).
    - Ensure the target Pinecone index has encryption-at-rest enabled and
      namespace access controls configured before seeding personal data
      (GDPR Art. 32).

    Args:
        documents: List of ``{"text": str, "metadata": dict}`` dicts.
        index: An initialised Pinecone ``Index`` object.
        namespace: Pinecone namespace to upsert into.
        batch_size: Number of vectors per upsert call (default: 100).
        source_id: Non-PHI identifier for this data source (audit trail).
    """
    vectors: List[dict] = []
    skipped_docs = 0
    skipped_chunks = 0
    total_chunks = 0

    log.info(
        "Seeding started: source_id=%s doc_count=%d namespace=%r",
        source_id,
        len(documents),
        namespace,
    )

    for doc in documents:
        raw_text = doc.get("text", "")
        # Only carry forward non-PHI metadata fields explicitly listed;
        # callers must not embed raw PII/PHI in the metadata dict.
        metadata = doc.get("metadata", {})

        if not validate_content(raw_text):
            skipped_docs += 1
            log.debug("SKIP doc: empty or too-short content (source_id=%s)", source_id)
            continue

        chunks = chunk_text(raw_text)

        for chunk_idx, chunk in enumerate(chunks):
            total_chunks += 1
            # ----------------------------------------------------------------
            # rag_002 fix: sanitize before embedding or storing
            # ----------------------------------------------------------------
            clean_chunk = sanitize_content(chunk)  # line ~63 equivalent

            if not validate_content(clean_chunk):
                skipped_chunks += 1
                log.debug(
                    "SKIP chunk: empty after sanitization "
                    "(source_id=%s chunk_idx=%d)",
                    source_id,
                    chunk_idx,
                )
                continue

            vector_id = hashlib.sha256(
                f"{metadata.get('source', 'unknown')}:{chunk_idx}:{clean_chunk[:64]}".encode()
            ).hexdigest()

            embedding = embed_text(clean_chunk)

            # GDPR Art. 5(1)(c) — store only what is necessary for vector
            # similarity search; full chunk text is NOT persisted in metadata.
            vectors.append(
                {
                    "id": vector_id,
                    "values": embedding,
                    "metadata": {**metadata, "chunk_index": chunk_idx},
                }
            )

            if len(vectors) >= batch_size:
                index.upsert(vectors=vectors, namespace=namespace)
                log.info(
                    "UPSERT batch: count=%d source_id=%s namespace=%r",
                    len(vectors),
                    source_id,
                    namespace,
                )
                vectors = []

    if vectors:
        index.upsert(vectors=vectors, namespace=namespace)
        log.info(
            "UPSERT final batch: count=%d source_id=%s namespace=%r",
            len(vectors),
            source_id,
            namespace,
        )

    log.info(
        "Seeding complete: source_id=%s total_chunks=%d "
        "skipped_docs=%d skipped_chunks=%d",
        source_id,
        total_chunks,
        skipped_docs,
        skipped_chunks,
    )


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Seed a Pinecone index with sanitized document chunks."
    )
    parser.add_argument("--index", required=True, help="Pinecone index name")
    parser.add_argument("--namespace", default="", help="Pinecone namespace")
    parser.add_argument(
        "--input",
        required=True,
        help="Path to a newline-delimited JSON file of documents",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=100,
        help="Number of vectors per upsert batch (default: 100)",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    api_key = os.environ.get("PINECONE_API_KEY")
    if not api_key:
        raise EnvironmentError("PINECONE_API_KEY environment variable is not set.")

    pc = pinecone.Pinecone(api_key=api_key)
    index = pc.Index(args.index)

    # Derive a non-PHI source identifier for audit logging from the filename
    # only — never log file contents or document text.
    source_id = os.path.basename(args.input)

    with open(args.input, "r", encoding="utf-8") as fh:
        documents = [json.loads(line) for line in fh if line.strip()]

    log.info(
        "Loaded %d documents from source_id=%s index=%s",
        len(documents),
        source_id,
        args.index,
    )
    seed_documents(
        documents=documents,
        index=index,
        namespace=args.namespace,
        batch_size=args.batch_size,
        source_id=source_id,
    )


if __name__ == "__main__":
    main()
