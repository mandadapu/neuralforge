"""seed_pinecone.py — Seed a Pinecone vector index from a list of documents.

Content validation and sanitization are applied before every upsert to guard
against prompt-injection attacks and malformed input (security finding rag_002).
"""

from __future__ import annotations

import logging
import os
import re
from typing import Any

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")

# ---------------------------------------------------------------------------
# Content sanitization (addresses rag_002 / line-63 finding)
# ---------------------------------------------------------------------------

# Patterns that resemble LLM instruction injection
_INSTRUCTION_PATTERNS = [
    re.compile(r"ignore\s+(all\s+)?previous\s+instructions?", re.IGNORECASE),
    re.compile(r"you\s+are\s+now\s+", re.IGNORECASE),
    re.compile(r"act\s+as\s+(a\s+)?", re.IGNORECASE),
    re.compile(r"disregard\s+(your\s+)?", re.IGNORECASE),
    re.compile(r"system\s*prompt", re.IGNORECASE),
    re.compile(r"<\s*(system|user|assistant)\s*>", re.IGNORECASE),
]
_MAX_CONTENT_BYTES = 40_000  # guard against oversized docs


def sanitize_content(text: str) -> str:
    """Validate and sanitize document content before vector store ingestion.

    Raises ValueError for:
    - Non-string input
    - Content exceeding _MAX_CONTENT_BYTES
    - Empty or whitespace-only content
    - Content matching any instruction-injection pattern

    Strips non-printable control characters (except newline and tab) from
    valid content before returning it.
    """
    if not isinstance(text, str):
        raise ValueError(f"Expected str, got {type(text)}")
    if len(text.encode()) > _MAX_CONTENT_BYTES:
        raise ValueError("Document exceeds maximum allowed size")
    if not text.strip():
        raise ValueError("Document content is empty")
    for pattern in _INSTRUCTION_PATTERNS:
        if pattern.search(text):
            raise ValueError(
                f"Document contains instruction-like pattern and was rejected: "
                f"{pattern.pattern!r}"
            )
    # Strip non-printable control characters (except newline \n and tab \t)
    text = re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", "", text)
    return text


# ---------------------------------------------------------------------------
# Embedding (stub — replace with your actual embedding client)
# ---------------------------------------------------------------------------


def embed(text: str) -> list[float]:
    """Return an embedding vector for *text*.

    Replace this stub with a real embedding call, e.g.:
        import openai
        response = openai.embeddings.create(model="text-embedding-3-small", input=text)
        return response.data[0].embedding
    """
    raise NotImplementedError(
        "embed() is a stub. Configure a real embedding backend before use."
    )


# ---------------------------------------------------------------------------
# Vector construction
# ---------------------------------------------------------------------------


def build_vector(doc: dict[str, Any]) -> dict[str, Any]:
    """Sanitize document content and build a Pinecone-compatible vector record."""
    content = sanitize_content(doc["content"])  # ← sanitization before upsert
    embedding = embed(content)
    return {
        "id": doc["id"],
        "values": embedding,
        "metadata": {"text": content},
    }


# ---------------------------------------------------------------------------
# Seeding entry point
# ---------------------------------------------------------------------------


def seed(index: Any, documents: list[dict[str, Any]]) -> None:
    """Upsert *documents* into *index*, skipping any that fail validation.

    Each document dict must have:
        - "id"      (str) unique identifier
        - "content" (str) text to embed and store
    """
    for doc in documents:
        try:
            vector = build_vector(doc)
            index.upsert([vector])
            logging.info("Upserted document %s", doc.get("id"))
        except ValueError as exc:
            # Log and skip — one bad document must not abort the entire run
            logging.warning("Skipping document %s: %s", doc.get("id"), exc)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def main() -> None:
    import json

    api_key = os.environ.get("PINECONE_API_KEY")
    index_name = os.environ.get("PINECONE_INDEX")
    docs_path = os.environ.get("DOCUMENTS_PATH", "documents.json")

    if not api_key or not index_name:
        raise SystemExit(
            "Required environment variables not set: PINECONE_API_KEY, PINECONE_INDEX"
        )

    try:
        import pinecone  # type: ignore[import]
    except ImportError as exc:
        raise SystemExit("pinecone-client is not installed: pip install pinecone-client") from exc

    pc = pinecone.Pinecone(api_key=api_key)
    index = pc.Index(index_name)

    with open(docs_path, encoding="utf-8") as fh:
        documents = json.load(fh)

    seed(index, documents)


if __name__ == "__main__":
    main()
