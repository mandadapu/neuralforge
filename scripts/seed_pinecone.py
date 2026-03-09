"""
seed_pinecone.py — Seed a Pinecone vector index from source documents.

Security note (rag_002): All text chunks are sanitized via `sanitize_content`
before embedding and upsert to prevent RAG prompt-injection attacks.
"""

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

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
logger = logging.getLogger(__name__)

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
        logger.warning(
            "Skipping chunk: injection pattern detected at position %d: %r",
            match.start(),
            match.group(0),
        )
        return None

    # 5. Truncate to safe maximum length.
    if len(text) > MAX_CHUNK_LENGTH:
        logger.warning(
            "Truncating chunk from %d to %d chars.", len(text), MAX_CHUNK_LENGTH
        )
        text = text[:MAX_CHUNK_LENGTH]

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


def embed_and_upsert(chunks: list, index) -> None:
    """Sanitize, embed, and upsert *chunks* into the Pinecone *index*.

    Each element of *chunks* must have a ``.text`` attribute (str) and an
    ``.id`` attribute (str) used as the Pinecone vector ID.

    Chunks that fail sanitization are silently skipped (a warning is logged).
    """
    vectors: list[tuple] = []

    for chunk in chunks:
        # rag_002 fix: validate and sanitize before adding to vector store.
        clean_text = sanitize_content(chunk.text)
        if clean_text is None:
            continue  # skip rejected chunk

        embedding = _get_embedding(clean_text)
        metadata = {"text": clean_text}
        vectors.append((chunk.id, embedding, metadata))

    if vectors:
        index.upsert(vectors=vectors)
        logger.info("Upserted %d vectors.", len(vectors))
    else:
        logger.info("No vectors to upsert (all chunks were rejected or empty).")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    """Read source documents, split into chunks, and seed the Pinecone index."""
    from pinecone import Pinecone  # type: ignore[import-untyped]

    pc = Pinecone(api_key=PINECONE_API_KEY)
    index = pc.Index(PINECONE_INDEX)

    # Load and chunk documents.  Replace this block with your actual document
    # loading / chunking logic (e.g., LangChain TextSplitter, custom splitter).
    documents = _load_documents()
    chunks = _split_into_chunks(documents)

    embed_and_upsert(chunks, index)


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
